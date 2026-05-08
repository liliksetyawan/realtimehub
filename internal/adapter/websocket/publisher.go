package websocket

import (
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

// HubPublisher adapts the in-process Hub to port.Publisher so the
// SendNotification use case can fan out without depending on the
// websocket package directly.
//
// Phase 4 will replace this with a Redis-backed publisher that broadcasts
// across nodes; the local Hub becomes a *subscriber* of the Redis channel.
type HubPublisher struct {
	hub *Hub
}

func NewHubPublisher(h *Hub) *HubPublisher { return &HubPublisher{hub: h} }

func (p *HubPublisher) SendNotification(userID string, n *domain.Notification) error {
	payload := NotificationPayload{
		ID:        n.ID,
		Title:     n.Title,
		Body:      n.Body,
		Data:      n.Data,
		CreatedAt: n.CreatedAt,
	}
	frame := Frame{
		Type:    MsgNotification,
		Seq:     n.Seq,
		Payload: mustMarshal(payload),
	}
	p.hub.SendToUser(userID, frame)
	return nil
}
