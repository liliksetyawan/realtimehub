package websocket

import (
	"context"

	"github.com/liliksetyawan/realtimehub/internal/domain"
)

// HubPublisher adapts the in-process Hub to port.Publisher for the
// single-node case (or tests). Multi-node deploys use the Redis-backed
// publisher under internal/adapter/redis; this one stays as a small,
// dependency-free implementation.
type HubPublisher struct {
	hub *Hub
}

func NewHubPublisher(h *Hub) *HubPublisher { return &HubPublisher{hub: h} }

func (p *HubPublisher) SendNotification(_ context.Context, userID string, n *domain.Notification) error {
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
