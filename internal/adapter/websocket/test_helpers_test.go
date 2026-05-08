package websocket

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// newTestConnection builds a *Connection without a real upgraded
// websocket. Tests that exercise queueing / Close / sharding don't need
// the wire — Connection.Close() is wsConn-nil-safe specifically so this
// helper works.
func newTestConnection(t *testing.T, id, userID string, bufSize int) *Connection {
	t.Helper()
	if bufSize <= 0 {
		bufSize = 8
	}
	c := &Connection{
		ID:       id,
		UserID:   userID,
		out:      make(chan []byte, bufSize),
		log:      zerolog.Nop(),
		lastPong: atomic.Int64{},
	}
	c.lastPong.Store(time.Now().UnixNano())
	return c
}

// drain reads up to max messages from c.out, with a small per-poll
// timeout. Returns whatever it got.
func drain(t *testing.T, c *Connection, max int) [][]byte {
	t.Helper()
	out := make([][]byte, 0, max)
	deadline := time.After(100 * time.Millisecond)
	for len(out) < max {
		select {
		case msg, ok := <-c.out:
			if !ok {
				return out
			}
			out = append(out, msg)
		case <-deadline:
			return out
		}
	}
	return out
}
