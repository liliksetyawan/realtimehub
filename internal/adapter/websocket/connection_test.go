package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnection_EnqueueRawAcceptsWhileBufferHasSpace(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 4)
	for i := 0; i < 4; i++ {
		assert.True(t, c.EnqueueRaw([]byte("ok")), "buffer not full yet")
	}
}

func TestConnection_EnqueueRawReturnsFalseOnFullBuffer(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 2)
	assert.True(t, c.EnqueueRaw([]byte("a")))
	assert.True(t, c.EnqueueRaw([]byte("b")))
	assert.False(t, c.EnqueueRaw([]byte("c")),
		"third enqueue must fail — slow consumer signal")
}

func TestConnection_EnqueueRawReturnsFalseAfterClose(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 2)
	c.Close()
	assert.False(t, c.EnqueueRaw([]byte("late")))
}

func TestConnection_SendFrameMarshalsAndQueues(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 4)
	require.True(t, c.SendFrame(Frame{Type: MsgPong}))

	got := drain(t, c, 1)
	require.Len(t, got, 1)
	assert.Contains(t, string(got[0]), `"type":"pong"`)
}

func TestConnection_CloseIsIdempotent(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 2)
	c.Close()
	c.Close() // second call must not panic on close-of-closed-channel
	assert.True(t, c.IsClosed())
}

func TestConnection_MarkPongUpdatesLastPong(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 2)
	before := c.lastPong.Load()
	time.Sleep(2 * time.Millisecond)
	c.MarkPong()
	assert.Greater(t, c.lastPong.Load(), before)
}

func TestConnection_SeqGetSet(t *testing.T) {
	c := newTestConnection(t, "c1", "u_alice", 2)
	assert.Equal(t, int64(0), c.LastSeq())
	c.SetLastSeq(99)
	assert.Equal(t, int64(99), c.LastSeq())
}
