package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/rs/zerolog"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	return NewHub(HubConfig{Shards: 4}, zerolog.Nop())
}

func TestHub_RegisterUnregisterCount(t *testing.T) {
	h := newTestHub(t)
	c1 := newTestConnection(t, "c1", "u_alice", 4)
	c2 := newTestConnection(t, "c2", "u_alice", 4) // alice has 2 tabs
	c3 := newTestConnection(t, "c3", "u_bob", 4)

	h.Register(c1)
	h.Register(c2)
	h.Register(c3)
	assert.Equal(t, int64(3), h.ConnectionCount())

	h.Unregister(c2)
	assert.Equal(t, int64(2), h.ConnectionCount())

	// Unregistering an unknown conn must be a no-op.
	h.Unregister(newTestConnection(t, "stranger", "u_zoe", 4))
	assert.Equal(t, int64(2), h.ConnectionCount())

	h.Unregister(c1)
	h.Unregister(c3)
	assert.Equal(t, int64(0), h.ConnectionCount())
}

func TestHub_SendToUserDeliversToAllOfThatUsersConnections(t *testing.T) {
	h := newTestHub(t)
	c1 := newTestConnection(t, "c1", "u_alice", 4)
	c2 := newTestConnection(t, "c2", "u_alice", 4)
	c3 := newTestConnection(t, "c3", "u_bob", 4)
	h.Register(c1)
	h.Register(c2)
	h.Register(c3)

	delivered := h.SendToUser("u_alice", Frame{Type: MsgNotification, Seq: 1})
	assert.Equal(t, 2, delivered, "both alice tabs receive")

	assert.Len(t, drain(t, c1, 1), 1)
	assert.Len(t, drain(t, c2, 1), 1)
	assert.Len(t, drain(t, c3, 0), 0, "bob must not receive alice's notification")
}

func TestHub_SendToUserUnknownUserReturnsZero(t *testing.T) {
	h := newTestHub(t)
	assert.Equal(t, 0, h.SendToUser("u_nobody", Frame{Type: MsgPong}))
}

func TestHub_SendToUserClosesSlowConsumer(t *testing.T) {
	h := newTestHub(t)
	// buf=1: first frame fits, second fails → conn closed.
	c := newTestConnection(t, "c1", "u_alice", 1)
	h.Register(c)

	// Pre-fill the buffer so the next SendToUser sees a full channel.
	require.True(t, c.EnqueueRaw([]byte("blocker")))
	delivered := h.SendToUser("u_alice", Frame{Type: MsgNotification, Seq: 1})
	assert.Equal(t, 0, delivered, "slow consumer was dropped instead of delivered")
	assert.True(t, c.IsClosed(), "slow consumer must be closed by the hub")
}

func TestHub_BroadcastReachesEveryUser(t *testing.T) {
	h := newTestHub(t)
	conns := []*Connection{
		newTestConnection(t, "a", "u_alice", 4),
		newTestConnection(t, "b", "u_bob", 4),
		newTestConnection(t, "c", "u_charlie", 4),
	}
	for _, c := range conns {
		h.Register(c)
	}

	delivered := h.Broadcast(Frame{Type: MsgNotification, Seq: 1})
	assert.Equal(t, 3, delivered)
	for _, c := range conns {
		assert.Len(t, drain(t, c, 1), 1, "every connection received the broadcast")
	}
}

func TestHub_ShardingIsDeterministic(t *testing.T) {
	h := newTestHub(t)
	// Same user_id always maps to the same shard pointer; different ids
	// generally map to different shards (at 4 shards collisions exist
	// but not for these well-spread strings).
	a := h.shardOf("u_alice")
	a2 := h.shardOf("u_alice")
	assert.Same(t, a, a2)
}

func TestHub_RegisterTracksLastSeqOnDelivery(t *testing.T) {
	h := newTestHub(t)
	c := newTestConnection(t, "c1", "u_alice", 4)
	h.Register(c)
	h.SendToUser("u_alice", Frame{Type: MsgNotification, Seq: 7})
	assert.Equal(t, int64(7), c.LastSeq())
}
