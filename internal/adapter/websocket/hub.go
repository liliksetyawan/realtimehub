package websocket

import (
	"context"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Hub holds every live connection grouped by user id. It is sharded
// (default 16) so registers / unregisters / broadcasts on different users
// don't contend for one big mutex — at 50k conns this matters.
//
// Each shard has its own RWMutex and connection map. Lookup hashes the
// user id to a shard.
type Hub struct {
	shards     []*hubShard
	shardCount uint32
	log        zerolog.Logger

	pingInterval time.Duration
	pongTimeout  time.Duration

	// metrics
	connCount atomic.Int64
}

type hubShard struct {
	mu          sync.RWMutex
	connections map[string]map[*Connection]struct{} // userID -> set of conns
}

type HubConfig struct {
	Shards       int
	PingInterval time.Duration
	PongTimeout  time.Duration
}

func NewHub(cfg HubConfig, log zerolog.Logger) *Hub {
	if cfg.Shards <= 0 {
		cfg.Shards = 16
	}
	if cfg.PingInterval <= 0 {
		cfg.PingInterval = 25 * time.Second
	}
	if cfg.PongTimeout <= 0 {
		cfg.PongTimeout = 60 * time.Second
	}
	h := &Hub{
		shards:       make([]*hubShard, cfg.Shards),
		shardCount:   uint32(cfg.Shards),
		log:          log.With().Str("component", "ws-hub").Logger(),
		pingInterval: cfg.PingInterval,
		pongTimeout:  cfg.PongTimeout,
	}
	for i := range h.shards {
		h.shards[i] = &hubShard{
			connections: make(map[string]map[*Connection]struct{}),
		}
	}
	return h
}

// shardOf returns the shard for a given user id. fnv32a is cheap and good
// enough for distribution.
func (h *Hub) shardOf(userID string) *hubShard {
	hh := fnv.New32a()
	_, _ = hh.Write([]byte(userID))
	return h.shards[hh.Sum32()%h.shardCount]
}

// Register adds a connection to the hub. Multiple connections per user
// are supported (think laptop + phone open at the same time).
func (h *Hub) Register(c *Connection) {
	s := h.shardOf(c.UserID)
	s.mu.Lock()
	set, ok := s.connections[c.UserID]
	if !ok {
		set = make(map[*Connection]struct{})
		s.connections[c.UserID] = set
	}
	set[c] = struct{}{}
	s.mu.Unlock()

	h.connCount.Add(1)
	h.log.Debug().Str("user_id", c.UserID).Str("conn_id", c.ID).
		Int64("total", h.connCount.Load()).Msg("registered")
}

// Unregister removes a connection. Safe to call multiple times.
func (h *Hub) Unregister(c *Connection) {
	s := h.shardOf(c.UserID)
	s.mu.Lock()
	if set, ok := s.connections[c.UserID]; ok {
		if _, present := set[c]; present {
			delete(set, c)
			if len(set) == 0 {
				delete(s.connections, c.UserID)
			}
			h.connCount.Add(-1)
		}
	}
	s.mu.Unlock()
}

// SendToUser fans `frame` out to every connection of `userID`. Returns
// the number of connections it reached. Slow consumers (full write
// buffer) are closed and counted as a miss.
func (h *Hub) SendToUser(userID string, frame Frame) int {
	s := h.shardOf(userID)
	s.mu.RLock()
	set := s.connections[userID]
	if len(set) == 0 {
		s.mu.RUnlock()
		return 0
	}
	// Snapshot the slice outside the lock so we don't hold RLock during writes.
	conns := make([]*Connection, 0, len(set))
	for c := range set {
		conns = append(conns, c)
	}
	s.mu.RUnlock()

	delivered := 0
	for _, c := range conns {
		if c.SendFrame(frame) {
			c.SetLastSeq(frame.Seq)
			delivered++
		} else {
			h.log.Warn().Str("conn_id", c.ID).Msg("write buffer full, closing slow consumer")
			c.Close()
		}
	}
	return delivered
}

// Broadcast sends `frame` to every connected user. Used for
// announcement-style channels. Heavy: walks every shard.
func (h *Hub) Broadcast(frame Frame) int {
	delivered := 0
	for _, s := range h.shards {
		s.mu.RLock()
		conns := make([]*Connection, 0)
		for _, set := range s.connections {
			for c := range set {
				conns = append(conns, c)
			}
		}
		s.mu.RUnlock()

		for _, c := range conns {
			if c.SendFrame(frame) {
				c.SetLastSeq(frame.Seq)
				delivered++
			} else {
				c.Close()
			}
		}
	}
	return delivered
}

// ConnectionCount returns the live count. Cheap atomic read.
func (h *Hub) ConnectionCount() int64 { return h.connCount.Load() }

// StartReaper walks every shard at PingInterval and evicts connections
// that haven't sent a pong inside PongTimeout.
//
// We send pings here too rather than from a per-connection ticker — that
// would be 50k tickers under load. One reaper goroutine, one timer.
func (h *Hub) StartReaper(ctx context.Context) {
	go func() {
		t := time.NewTicker(h.pingInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				h.tick()
			}
		}
	}()
}

func (h *Hub) tick() {
	cutoff := time.Now().Add(-h.pongTimeout).UnixNano()
	pingFrame := Frame{Type: MsgPong} // server's pongs are also called "pong" — clients reply with their own pong
	// Actually we want the SERVER to send a "ping" the client must echo.
	// But to keep the wire JSON-only, we just send a server-initiated frame
	// the client must answer with type=pong. Reuse the typed protocol:
	pingFrame.Type = MsgType("server_ping")

	stale := 0
	for _, s := range h.shards {
		s.mu.RLock()
		conns := make([]*Connection, 0)
		for _, set := range s.connections {
			for c := range set {
				conns = append(conns, c)
			}
		}
		s.mu.RUnlock()

		for _, c := range conns {
			if c.lastPong.Load() < cutoff {
				h.log.Info().Str("conn_id", c.ID).Msg("stale conn, evicting")
				c.Close()
				stale++
				continue
			}
			c.SendFrame(pingFrame)
		}
	}
	if stale > 0 {
		h.log.Info().Int("evicted", stale).Int64("remaining", h.connCount.Load()).Msg("reaper tick")
	}
}
