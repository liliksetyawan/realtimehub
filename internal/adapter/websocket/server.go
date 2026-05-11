package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Server wires nbio's epoll engine to our Hub. Construct via NewServer,
// register handlers on the embedded mux, then call Start / Stop.
type Server struct {
	hub     *Hub
	mux     *http.ServeMux
	eng     *nbhttp.Engine
	log     zerolog.Logger
	cfg     ServerConfig
	auth    Authenticator
	history History
}

// Authenticator validates the credential supplied at handshake time
// (typically a JWT in the ?token= query param) and returns the user id.
// Returning an error fails the upgrade with 401.
type Authenticator interface {
	Authenticate(r *http.Request) (userID string, err error)
}

// History gives the server read access to past notifications: the
// per-user current seq (sent in WelcomePayload so the client can decide
// whether to resume) and a bulk fetch from a sequence checkpoint
// (used to replay missed notifications on reconnect).
//
// Implemented by postgres.NotificationRepo — defined here as a small
// interface so the websocket package keeps a narrow surface.
type History interface {
	CurrentSeq(ctx context.Context, userID string) (int64, error)
	SinceSeq(ctx context.Context, userID string, fromSeq int64, limit int) ([]*Replayable, error)
}

// Replayable is a transport-shaped DTO instead of *domain.Notification
// to keep this package free of the domain import. Adapters at the
// boundary (postgres ↔ websocket) translate.
type Replayable struct {
	ID        string
	Seq       int64
	Title     string
	Body      string
	Data      map[string]string
	CreatedAt time.Time
}

type ServerConfig struct {
	Addr         string        // ":8090"
	WriteBuffer  int           // per-connection bounded write buffer (default 64)
	PingInterval time.Duration // hub reaper period
	PongTimeout  time.Duration // evict-after-no-pong duration
	HubShards    int
}

func NewServer(cfg ServerConfig, auth Authenticator, history History, log zerolog.Logger) *Server {
	mux := http.NewServeMux()

	hub := NewHub(HubConfig{
		Shards:       cfg.HubShards,
		PingInterval: cfg.PingInterval,
		PongTimeout:  cfg.PongTimeout,
	}, log)

	s := &Server{
		hub:     hub,
		mux:     mux,
		log:     log.With().Str("component", "ws-server").Logger(),
		cfg:     cfg,
		auth:    auth,
		history: history,
	}

	mux.HandleFunc("/ws", s.handleUpgrade)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":      "ok",
			"connections": hub.ConnectionCount(),
		})
	})

	// Wrap the mux with otelhttp so every REST request becomes a root
	// span and the WS upgrade carries `traceparent` through to handleUpgrade.
	// Per-route span names come from the route formatter — gives Jaeger
	// "POST /v1/admin/notifications" instead of just "HTTP POST".
	instrumented := otelhttp.NewHandler(mux, "realtimehub",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)

	s.eng = nbhttp.NewEngine(nbhttp.Config{
		Network:        "tcp",
		Addrs:          []string{cfg.Addr},
		MaxLoad:        1_000_000,
		Handler:        instrumented,
		ReadBufferSize: 4096,
		ReadLimit:      1 << 20, // 1 MiB max frame
	})

	return s
}

// Hub exposes the registry so application use cases (SendNotification)
// can fan out via SendToUser / Broadcast.
func (s *Server) Hub() *Hub { return s.hub }

// Mux returns the embedded mux so the composition root can register
// additional REST routes (admin API, login endpoint, etc.) on the same
// nbio engine.
func (s *Server) Mux() *http.ServeMux { return s.mux }

// Start spins up nbio's listener loop and the hub reaper. Non-blocking.
func (s *Server) Start(ctx context.Context) error {
	if err := s.eng.Start(); err != nil {
		return fmt.Errorf("nbio start: %w", err)
	}
	s.hub.StartReaper(ctx)
	s.log.Info().Str("addr", s.cfg.Addr).Msg("websocket server listening")
	return nil
}

// Stop drains in-flight connections and shuts down the engine. The
// shutdown error is logged rather than returned — the caller is the
// composition root mid-shutdown, there's nothing useful it can do.
func (s *Server) Stop(ctx context.Context) {
	if err := s.eng.Shutdown(ctx); err != nil {
		s.log.Warn().Err(err).Msg("nbio shutdown")
	}
}

// handleUpgrade authenticates, upgrades, registers the connection, and
// wires the read loop. nbio fires OnMessage / OnClose from its IO goroutines.
func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	userID, err := s.auth.Authenticate(r)
	if err != nil {
		s.log.Debug().Err(err).Msg("auth failed at handshake")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.NewUpgrader()
	connID := uuid.NewString()

	upgrader.OnMessage(func(c *websocket.Conn, mt websocket.MessageType, data []byte) {
		conn, ok := c.Session().(*Connection)
		if !ok {
			return
		}
		s.handleMessage(conn, data)
	})
	upgrader.OnClose(func(c *websocket.Conn, _ error) {
		conn, ok := c.Session().(*Connection)
		if !ok {
			return
		}
		s.hub.Unregister(conn)
		conn.Close()
	})

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn().Err(err).Msg("upgrade failed")
		return
	}

	conn := newConnection(connID, userID, wsConn, s.cfg.WriteBuffer, s.log)
	wsConn.SetSession(conn)
	conn.startWriter()
	s.hub.Register(conn)

	// Look up the user's current seq so the client can decide whether
	// it has missed notifications. Best-effort — if the lookup fails we
	// still let the connection in with seq=0 and the client just won't
	// resume on this attempt.
	currentSeq := int64(0)
	if s.history != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if seq, err := s.history.CurrentSeq(ctx, userID); err == nil {
			currentSeq = seq
		}
		cancel()
	}

	conn.SendFrame(Frame{
		Type: MsgWelcome,
		Payload: mustMarshal(WelcomePayload{
			UserID:     userID,
			ConnID:     connID,
			CurrentSeq: currentSeq,
			ServerTime: time.Now().UTC(),
		}),
	})
}

// handleMessage decodes one inbound frame and routes it. Per-message
// failures log + continue; we don't tear the connection down for a single
// bad frame from a chatty client.
func (s *Server) handleMessage(c *Connection, data []byte) {
	var f Frame
	if err := json.Unmarshal(data, &f); err != nil {
		c.log.Debug().Err(err).Msg("bad frame")
		return
	}
	switch f.Type {
	case MsgPing, MsgType("server_ping"):
		c.MarkPong()
		c.SendFrame(Frame{Type: MsgPong})
	case MsgPong:
		c.MarkPong()
	case MsgAck:
		// We don't currently use ack for delivery — the client marks-read
		// via REST. Reserved for future at-most-once delivery semantics.
	case MsgResume:
		s.handleResume(c, f.Payload)
	default:
		c.log.Debug().Str("type", string(f.Type)).Msg("unknown frame type")
	}
}

// handleResume replays every notification with seq > FromSeq for this
// connection's user, in seq-ascending order. Bounded to 200 per call —
// a client that's been offline for a week can keep asking until it
// catches up.
func (s *Server) handleResume(c *Connection, raw json.RawMessage) {
	if s.history == nil {
		return
	}
	var p ResumePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		c.log.Debug().Err(err).Msg("bad resume payload")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	missed, err := s.history.SinceSeq(ctx, c.UserID, p.FromSeq, 200)
	if err != nil {
		c.log.Warn().Err(err).Msg("resume fetch failed")
		return
	}
	for _, n := range missed {
		c.SendFrame(Frame{
			Type: MsgNotification,
			Seq:  n.Seq,
			Payload: mustMarshal(NotificationPayload{
				ID:        n.ID,
				Title:     n.Title,
				Body:      n.Body,
				Data:      n.Data,
				CreatedAt: n.CreatedAt,
			}),
		})
	}
	c.log.Info().Int64("from_seq", p.FromSeq).Int("replayed", len(missed)).Msg("resume served")
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
