package websocket_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	wsadapter "github.com/liliksetyawan/realtimehub/internal/adapter/websocket"
)

func TestFrame_MarshalRoundtrip(t *testing.T) {
	original := wsadapter.Frame{
		Type:    wsadapter.MsgNotification,
		Seq:     42,
		Payload: json.RawMessage(`{"id":"n1","title":"hi"}`),
	}
	wire, err := json.Marshal(original)
	require.NoError(t, err)

	var got wsadapter.Frame
	require.NoError(t, json.Unmarshal(wire, &got))

	assert.Equal(t, original.Type, got.Type)
	assert.Equal(t, original.Seq, got.Seq)
	assert.JSONEq(t, string(original.Payload), string(got.Payload))
}

func TestFrame_OmitsSeqWhenZero(t *testing.T) {
	f := wsadapter.Frame{Type: wsadapter.MsgPong}
	b, err := json.Marshal(f)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"seq"`,
		"seq=0 should be omitted via omitempty so pings/pongs stay tiny")
}

func TestMsgTypeConstants(t *testing.T) {
	// These string values are part of the wire contract; downstream
	// clients (the React SPA, k6 load test) hard-code them.
	cases := map[wsadapter.MsgType]string{
		wsadapter.MsgWelcome:      "welcome",
		wsadapter.MsgNotification: "notification",
		wsadapter.MsgPong:         "pong",
		wsadapter.MsgError:        "error",
		wsadapter.MsgPing:         "ping",
		wsadapter.MsgAck:          "ack",
		wsadapter.MsgResume:       "resume",
	}
	for got, want := range cases {
		assert.Equal(t, want, string(got))
	}
}
