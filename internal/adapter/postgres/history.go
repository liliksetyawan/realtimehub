package postgres

import (
	"context"

	wsadapter "github.com/liliksetyawan/realtimehub/internal/adapter/websocket"
)

// HistoryAdapter exposes the postgres notification repo as the smaller
// websocket.History interface — translating *domain.Notification to the
// websocket.Replayable DTO so the websocket package stays free of the
// domain import.
type HistoryAdapter struct {
	repo *NotificationRepo
}

func NewHistoryAdapter(repo *NotificationRepo) *HistoryAdapter {
	return &HistoryAdapter{repo: repo}
}

var _ wsadapter.History = (*HistoryAdapter)(nil)

func (h *HistoryAdapter) CurrentSeq(ctx context.Context, userID string) (int64, error) {
	return h.repo.CurrentSeq(ctx, userID)
}

func (h *HistoryAdapter) AckedSeq(ctx context.Context, userID string) (int64, error) {
	return h.repo.AckedSeq(ctx, userID)
}

func (h *HistoryAdapter) RecordAck(ctx context.Context, userID string, upToSeq int64) error {
	return h.repo.RecordAck(ctx, userID, upToSeq)
}

func (h *HistoryAdapter) SinceSeq(ctx context.Context, userID string, fromSeq int64, limit int) ([]*wsadapter.Replayable, error) {
	rows, err := h.repo.SinceSeq(ctx, userID, fromSeq, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*wsadapter.Replayable, len(rows))
	for i, n := range rows {
		out[i] = &wsadapter.Replayable{
			ID:        n.ID,
			Seq:       n.Seq,
			Title:     n.Title,
			Body:      n.Body,
			Data:      n.Data,
			CreatedAt: n.CreatedAt,
		}
	}
	return out, nil
}
