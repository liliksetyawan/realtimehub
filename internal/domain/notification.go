// Package domain holds RealtimeHub's core types. Pure: no SQL, no
// websocket library, no third-party state.
package domain

import "time"

type Notification struct {
	ID        string
	UserID    string // recipient; "" means broadcast (channel)
	Channel   string // optional: subscriber-based fan-out (e.g. "announcements")
	Title     string
	Body      string
	Data      map[string]string // arbitrary key/values for client routing
	Seq       int64             // monotonic per-recipient; used for resume-from-seq
	CreatedAt time.Time
	ReadAt    *time.Time
}

func (n *Notification) IsRead() bool { return n.ReadAt != nil }
