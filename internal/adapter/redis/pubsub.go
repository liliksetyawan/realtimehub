// Package redis bridges the application to Redis pub-sub for cross-node
// fan-out. Every running node both publishes to and subscribes from the
// same channel pattern, so a notification produced on node A reaches
// connections on node B without nodes needing to know about each other.
//
// Channel scheme: `notif:user:<user_id>`. We PSUBSCRIBE on the wildcard
// pattern so adding a new user requires no subscription change. For
// extreme scale (millions of users) you'd switch to per-user SUBSCRIBE,
// at the cost of a registration step on connect.
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

// Subscriber consumes the notif:user:* pattern and dispatches each
// message to the local Hub. Run once per process via Start; cancel ctx
// to stop.
type Subscriber struct {
	client rueidis.Client
	hub    *websocket.Hub
	log    zerolog.Logger
}

func NewSubscriber(client rueidis.Client, hub *websocket.Hub, log zerolog.Logger) *Subscriber {
	return &Subscriber{
		client: client,
		hub:    hub,
		log:    log.With().Str("component", "redis-sub").Logger(),
	}
}

// Start begins the subscription loop in a goroutine. Returns when the
// initial subscribe is acknowledged, or with an error if it fails.
//
// rueidis's Receive() blocks until ctx is canceled or the connection
// drops. On drop, rueidis auto-reconnects and re-subscribes — we don't
// have to re-implement reconnection here.
func (s *Subscriber) Start(ctx context.Context) error {
	pattern := channelPrefix + "*"
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			err := s.client.Receive(ctx, s.client.B().Psubscribe().Pattern(pattern).Build(),
				func(msg rueidis.PubSubMessage) { s.handleMessage(ctx, msg) })
			if err != nil && ctx.Err() == nil {
				s.log.Warn().Err(err).Msg("subscriber dropped; rueidis will reconnect")
			}
		}
	}()
	s.log.Info().Str("pattern", pattern).Msg("redis subscriber started")
	return nil
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

	// Extract the trace context from the frame and continue the trace.
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
