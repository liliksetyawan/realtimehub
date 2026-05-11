// Package websocket holds the WebSocket transport adapter: nbio engine,
// connection state, and the sharded hub that routes notifications to
// connections.
package websocket

import (
	"encoding/json"
	"time"
)

// MsgType is the discriminator on every JSON wire message.
type MsgType string

const (
	// Server → client.
	MsgWelcome      MsgType = "welcome"
	MsgNotification MsgType = "notification"
	MsgPong         MsgType = "pong"
	MsgError        MsgType = "error"

	// Client → server.
	MsgPing   MsgType = "ping"
	MsgAck    MsgType = "ack"
	MsgResume MsgType = "resume"
)

// Frame is the envelope on the wire. Payload is left as RawMessage so the
// hub can route and replay without re-marshaling business data.
//
// TraceParent carries a W3C trace context across the Redis pub-sub hop —
// publisher injects, subscriber extracts, and the trace stays linked
// HTTP → use case → publish → subscribe → hub → client.
type Frame struct {
	Type        MsgType         `json:"type"`
	Seq         int64           `json:"seq,omitempty"`
	TraceParent string          `json:"traceparent,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// WelcomePayload is sent right after a successful upgrade. It carries:
//
//   - CurrentSeq: the highest seq this user has on file in postgres
//   - AckedSeq:   the highest seq the *server* has confirmed delivered
//     (from the delivery_offsets table)
//
// The client compares these against its own last-seen seq and decides
// whether to send a `resume` frame. Unread badge = CurrentSeq - max(AckedSeq, localLastSeen).
type WelcomePayload struct {
	UserID     string    `json:"user_id"`
	ConnID     string    `json:"conn_id"`
	CurrentSeq int64     `json:"current_seq"`
	AckedSeq   int64     `json:"acked_seq"`
	ServerTime time.Time `json:"server_time"`
}

// NotificationPayload is the body of a MsgNotification. Mirrors the
// fields the React client actually renders.
type NotificationPayload struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// AckPayload — client confirms it has rendered notifications up to Seq.
type AckPayload struct {
	UpToSeq int64 `json:"up_to_seq"`
}

// ResumePayload — client asks for replay of every notification with
// seq > FromSeq.
type ResumePayload struct {
	FromSeq int64 `json:"from_seq"`
}

// ErrorPayload is sent before close on a fatal handshake / protocol error.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
