package websocket

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lesismal/nbio/nbhttp/websocket"
	"github.com/rs/zerolog"
)

// Connection wraps a single client websocket. We keep the *nbio.Conn
// reference plus our own bookkeeping (user id, write buffer, last pong)
// so the hub can broadcast without touching the underlying conn directly.
//
// Write path: anything that wants to send goes through the bounded `out`
// channel. A dedicated writer goroutine drains it. If the channel fills
// (slow consumer), the connection is closed — backpressure beats blocking
// the broadcaster.
type Connection struct {
	ID     string
	UserID string
	wsConn *websocket.Conn
	log    zerolog.Logger

	out      chan []byte
	closed   atomic.Bool
	lastPong atomic.Int64 // unix nanos; updated on every pong
	lastSeq  atomic.Int64 // last seq successfully written to wire
	mu       sync.Mutex   // serializes wsConn writes (nbio is goroutine-safe but predictable order matters)
}

const (
	defaultWriteBuf = 64
	writeTimeout    = 5 * time.Second
)

func newConnection(id, userID string, wsConn *websocket.Conn, bufSize int, log zerolog.Logger) *Connection {
	if bufSize <= 0 {
		bufSize = defaultWriteBuf
	}
	c := &Connection{
		ID:     id,
		UserID: userID,
		wsConn: wsConn,
		log:    log.With().Str("conn_id", id).Str("user_id", userID).Logger(),
		out:    make(chan []byte, bufSize),
	}
	c.lastPong.Store(time.Now().UnixNano())
	return c
}

// EnqueueRaw queues already-marshaled bytes. Returns false if the buffer
// is full (slow consumer) — caller decides whether to drop or close.
func (c *Connection) EnqueueRaw(b []byte) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.out <- b:
		return true
	default:
		return false
	}
}

// SendFrame is a convenience wrapper for typed messages. Same semantics
// as EnqueueRaw — non-blocking, returns false on full buffer.
func (c *Connection) SendFrame(f Frame) bool {
	b, err := json.Marshal(f)
	if err != nil {
		c.log.Error().Err(err).Msg("marshal frame")
		return false
	}
	return c.EnqueueRaw(b)
}

// startWriter drains `out` to the wire. One goroutine per connection.
// It exits when the connection closes or the channel is closed.
func (c *Connection) startWriter() {
	go func() {
		for msg := range c.out {
			c.mu.Lock()
			_ = c.wsConn.SetWriteDeadline(time.Now().Add(writeTimeout))
			err := c.wsConn.WriteMessage(websocket.TextMessage, msg)
			c.mu.Unlock()
			if err != nil {
				c.log.Warn().Err(err).Msg("write failed; closing")
				c.Close()
				return
			}
		}
	}()
}

// MarkPong is called from the read loop on every pong. Used by the hub
// reaper to evict stale connections.
func (c *Connection) MarkPong() {
	c.lastPong.Store(time.Now().UnixNano())
}

// LastSeq returns the seq of the last notification this connection has
// successfully been queued. (Whether the client has *acked* it is a
// separate state tracked at the application layer.)
func (c *Connection) LastSeq() int64 { return c.lastSeq.Load() }

// SetLastSeq is called by the broadcaster after enqueue.
func (c *Connection) SetLastSeq(s int64) { c.lastSeq.Store(s) }

// Close idempotently shuts the connection down. Safe to call from any
// goroutine; multiple calls are no-ops. The wsConn nil-check exists so
// unit tests can construct a Connection without a real upgraded socket.
func (c *Connection) Close() {
	if !c.closed.CompareAndSwap(false, true) {
		return
	}
	close(c.out)
	c.mu.Lock()
	if c.wsConn != nil {
		_ = c.wsConn.Close()
	}
	c.mu.Unlock()
}

// IsClosed returns whether the connection has been shut down.
func (c *Connection) IsClosed() bool { return c.closed.Load() }
