// Package usecase contains the application services that compose the
// domain over the ports.
package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/liliksetyawan/realtimehub/internal/app/port"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

type SendNotification struct {
	repo      port.NotificationRepository
	publisher port.Publisher
	log       zerolog.Logger
}

func NewSendNotification(repo port.NotificationRepository, publisher port.Publisher, log zerolog.Logger) *SendNotification {
	return &SendNotification{
		repo:      repo,
		publisher: publisher,
		log:       log.With().Str("usecase", "send_notification").Logger(),
	}
}

type SendNotificationInput struct {
	UserIDs []string
	Title   string
	Body    string
	Data    map[string]string
}

type SentNotification struct {
	ID     string
	UserID string
	Seq    int64
}

// Execute persists a notification per recipient (each gets its own seq,
// because seq is monotonic per user) and fans them out via the publisher.
// The publisher call is best-effort — if a user has no live conn, the
// row stays in postgres and they pick it up on next reconnect.
func (uc *SendNotification) Execute(ctx context.Context, in SendNotificationInput) ([]SentNotification, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Body = strings.TrimSpace(in.Body)
	if in.Title == "" || len(in.UserIDs) == 0 {
		return nil, domain.ErrInvalidInput
	}

	out := make([]SentNotification, 0, len(in.UserIDs))
	for _, uid := range in.UserIDs {
		uid = strings.TrimSpace(uid)
		if uid == "" {
			continue
		}
		n := &domain.Notification{
			ID:        uuid.NewString(),
			UserID:    uid,
			Title:     in.Title,
			Body:      in.Body,
			Data:      in.Data,
			CreatedAt: time.Now().UTC(),
		}
		if err := uc.repo.Create(ctx, n); err != nil {
			uc.log.Error().Err(err).Str("user_id", uid).Msg("persist failed")
			return nil, err
		}
		// Best-effort fan-out. Persistence is the source of truth; if the
		// user is offline, they catch up via SinceSeq on reconnect.
		if err := uc.publisher.SendNotification(uid, n); err != nil {
			uc.log.Warn().Err(err).Str("user_id", uid).Msg("publish failed; will catch up on reconnect")
		}
		out = append(out, SentNotification{ID: n.ID, UserID: uid, Seq: n.Seq})
	}
	return out, nil
}
