// Package redis bridges the application to Redis pub-sub for cross-node
// fan-out. Every running node both publishes to and subscribes from the
// same channel scheme, so a notification produced on node A reaches
// connections on node B without nodes needing to know about each other.
//
// Channel scheme: `notif:user:<user_id>`.
//
// # Subscription topology (Phase 11)
//
// Each node subscribes only to the channels for users it actually holds
// connections for. When the first conn for a user lands on this node,
// the Hub asks the Subscriber to SUBSCRIBE notif:user:<uid>; when the
// last conn drops, the Hub asks to UNSUBSCRIBE. Reference-counted, so a
// user with multiple tabs only causes one Redis subscription.
//
// Trade-offs vs the earlier PSUBSCRIBE wildcard:
//
//	+ each node only sees messages for its own users (Redis routes; no
//	  wasted bandwidth at fleet scale)
//	+ Redis side can shard the keyspace across cluster nodes
//	+ scales linearly with connected users per node, not total users
//	- adds a SUBSCRIBE RTT to the connect path (~ms; sequence recovery
//	  handles any in-flight notification missed during this window)
//	- on dedicated-conn drops, subscriptions must be re-established
//	  (documented limitation; future work)
//
// Tracing: the publisher injects the active W3C trace context into
// Frame.TraceParent before serializing; the subscriber extracts it and
// continues the trace, so Jaeger sees one chain HTTP → use case →
// publish → subscribe → hub.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/redis/rueidis"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/liliksetyawan/realtimehub/internal/adapter/websocket"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

const channelPrefix = "notif:user:"

func channelFor(userID string) string { return channelPrefix + userID }

var tracer = otel.Tracer("realtimehub/redis")

// Publisher implements port.Publisher by publishing to Redis. Every node
// running the subscriber will then deliver the notification to its local
// connections.
type Publisher struct {
	client rueidis.Client
	log    zerolog.Logger
}

func NewPublisher(client rueidis.Client, log zerolog.Logger) *Publisher {
	return &Publisher{client: client, log: log.With().Str("component", "redis-pub").Logger()}
}

func (p *Publisher) SendNotification(ctx context.Context, userID string, n *domain.Notification) error {
	ctx, span := tracer.Start(ctx, "redis.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "redis"),
			attribute.String("messaging.operation.name", "publish"),
			attribute.String("messaging.destination.name", channelFor(userID)),
			attribute.String("user_id", userID),
			attribute.Int64("notification.seq", n.Seq),
		),
	)
	defer span.End()

	payload := websocket.NotificationPayload{
		ID:        n.ID,
		Title:     n.Title,
		Body:      n.Body,
		Data:      n.Data,
		CreatedAt: n.CreatedAt,
	}
	frame := websocket.Frame{
		Type:    websocket.MsgNotification,
		Seq:     n.Seq,
		Payload: mustMarshal(payload),
	}
	// Inject the current trace context into the frame so the subscriber
	// (which may live on a different node) can continue the trace.
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	frame.TraceParent = carrier["traceparent"]

	body, err := json.Marshal(frame)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("marshal frame: %w", err)
	}
	cmd := p.client.B().Publish().Channel(channelFor(userID)).Message(string(body)).Build()
	if err := p.client.Do(ctx, cmd).Error(); err != nil {
		span.RecordError(err)
		return fmt.Errorf("redis publish: %w", err)
	}
	return nil
}

// Subscriber owns the dedicated pub-sub connection. Subscribe / Unsubscribe
// are reference-counted: only the first SUBSCRIBE per user hits Redis,
// and only the last UNSUBSCRIBE does. Concurrency-safe.
//
// Implements websocket.SubscriptionTracker so the Hub can call it
// directly on the first/last conn per user.
type Subscriber struct {
	client rueidis.Client
	hub    *websocket.Hub
	log    zerolog.Logger

	dc      rueidis.DedicatedClient
	cancel  func()
	hookErr <-chan error

	mu   sync.Mutex
	subs map[string]int // user_id → live conn count on this node
}

func NewSubscriber(client rueidis.Client, hub *websocket.Hub, log zerolog.Logger) *Subscriber {
	return &Subscriber{
		client: client,
		hub:    hub,
		log:    log.With().Str("component", "redis-sub").Logger(),
		subs:   make(map[string]int),
	}
}

// Start acquires the dedicated pubsub connection and wires the message
// handler. After Start, callers can Subscribe / Unsubscribe per-user.
func (s *Subscriber) Start(ctx context.Context) error {
	dc, cancel := s.client.Dedicate()
	s.dc = dc
	s.cancel = cancel

	// Install handlers BEFORE any subscribe — otherwise rueidis would
	// drop the very first message that arrived between Subscribe and the
	// handler attach.
	s.hookErr = dc.SetPubSubHooks(rueidis.PubSubHooks{
		OnMessage: func(m rueidis.PubSubMessage) {
			s.handleMessage(context.Background(), m)
		},
	})

	// Release the dedicated connection when the root context cancels.
	go func() {
		<-ctx.Done()
		s.cancel()
	}()

	s.log.Info().Msg("redis subscriber ready (per-user SUBSCRIBE)")
	return nil
}

// Subscribe is reference-counted; only the first call per user hits
// Redis. Safe to call concurrently. On Redis failure the refcount is
// rolled back so a retry can try again from scratch.
func (s *Subscriber) Subscribe(ctx context.Context, userID string) error {
	s.mu.Lock()
	cnt := s.subs[userID]
	s.subs[userID] = cnt + 1
	isFirst := cnt == 0
	s.mu.Unlock()

	if !isFirst {
		return nil
	}

	if err := s.dc.Do(ctx, s.dc.B().Subscribe().Channel(channelFor(userID)).Build()).Error(); err != nil {
		s.mu.Lock()
		if s.subs[userID] > 0 {
			s.subs[userID]--
		}
		if s.subs[userID] == 0 {
			delete(s.subs, userID)
		}
		s.mu.Unlock()
		return fmt.Errorf("redis subscribe %s: %w", userID, err)
	}
	return nil
}

// Unsubscribe is reference-counted; only the last call per user hits
// Redis. Calls without a prior Subscribe are no-ops (defensive — a
// double-Unregister mustn't issue spurious UNSUBSCRIBE).
func (s *Subscriber) Unsubscribe(ctx context.Context, userID string) error {
	s.mu.Lock()
	cnt := s.subs[userID]
	if cnt == 0 {
		s.mu.Unlock()
		return nil
	}
	if cnt == 1 {
		delete(s.subs, userID)
	} else {
		s.subs[userID] = cnt - 1
	}
	isLast := cnt == 1
	s.mu.Unlock()

	if !isLast {
		return nil
	}

	if err := s.dc.Do(ctx, s.dc.B().Unsubscribe().Channel(channelFor(userID)).Build()).Error(); err != nil {
		// Best-effort — Redis side will eventually time out the
		// subscription, and our local refcount is already cleared.
		s.log.Warn().Err(err).Str("user_id", userID).Msg("redis unsubscribe")
		return err
	}
	return nil
}

// ActiveSubscriptions returns the count of distinct user channels this
// node is currently subscribed to. Useful for /healthz reporting.
func (s *Subscriber) ActiveSubscriptions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs)
}

func (s *Subscriber) handleMessage(ctx context.Context, msg rueidis.PubSubMessage) {
	userID := strings.TrimPrefix(msg.Channel, channelPrefix)
	if userID == "" {
		return
	}
	var frame websocket.Frame
	if err := json.Unmarshal([]byte(msg.Message), &frame); err != nil {
		s.log.Warn().Err(err).Msg("decode frame from redis")
		return
	}

	carrier := propagation.MapCarrier{}
	if frame.TraceParent != "" {
		carrier["traceparent"] = frame.TraceParent
	}
	parentCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)

	_, span := tracer.Start(parentCtx, "redis.subscribe",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "redis"),
			attribute.String("messaging.operation.name", "deliver"),
			attribute.String("messaging.destination.name", msg.Channel),
			attribute.String("user_id", userID),
			attribute.Int64("notification.seq", frame.Seq),
		),
	)
	defer span.End()

	delivered := s.hub.SendToUser(userID, frame)
	span.SetAttributes(attribute.Int("hub.delivered", delivered))
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
