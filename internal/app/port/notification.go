// Package port defines the interfaces the application layer depends on.
// Concrete implementations live under internal/adapter.
package port

import (
	"context"

	"github.com/liliksetyawan/realtimehub/internal/domain"
)

// NotificationRepository persists notifications and exposes the queries
// the use cases need.
type NotificationRepository interface {
	// Create persists a new notification, allocating a per-user
	// monotonic seq inside the same transaction. The returned
	// notification has Seq populated.
	Create(ctx context.Context, n *domain.Notification) error

	// ListByUser returns notifications for a user, newest first,
	// paginated. unreadOnly filters to read_at IS NULL.
	ListByUser(ctx context.Context, userID string, unreadOnly bool, limit, offset int) (items []*domain.Notification, total int, err error)

	// SinceSeq returns every notification for user_id with seq > fromSeq,
	// ordered ascending. Used for resume-from-seq replay on reconnect.
	SinceSeq(ctx context.Context, userID string, fromSeq int64, limit int) ([]*domain.Notification, error)

	// MarkRead marks a single notification read. Only succeeds if the
	// notification belongs to userID (defense against authz mistakes).
	MarkRead(ctx context.Context, userID, notificationID string) error

	// CurrentSeq returns the last issued seq for a user (0 if none).
	// Used to populate WelcomePayload on connect.
	CurrentSeq(ctx context.Context, userID string) (int64, error)

	// UnreadCount returns count of unread notifications for a user.
	UnreadCount(ctx context.Context, userID string) (int, error)
}

// Publisher is the outbound port the SendNotification use case calls
// after persisting. Implemented by the Redis adapter, which serializes
// + publishes — and injects the current OTel trace context so subscribing
// nodes can continue the same trace across the Redis hop.
type Publisher interface {
	SendNotification(ctx context.Context, userID string, n *domain.Notification) error
}
