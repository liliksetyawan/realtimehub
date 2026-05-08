package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/liliksetyawan/realtimehub/internal/app/port"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

type NotificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

var _ port.NotificationRepository = (*NotificationRepo)(nil)

// Create allocates next seq + inserts the notification atomically. The
// UPSERT on user_offsets serialises concurrent inserts for the same
// user, so two SendNotification calls can't issue the same seq.
func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var nextSeq int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO user_offsets (user_id, last_seq)
		     VALUES ($1, 1)
		ON CONFLICT (user_id) DO UPDATE
		     SET last_seq = user_offsets.last_seq + 1
		     RETURNING last_seq
	`, n.UserID).Scan(&nextSeq); err != nil {
		return fmt.Errorf("upsert offset: %w", err)
	}

	dataJSON, err := json.Marshal(n.Data)
	if err != nil {
		return fmt.Errorf("marshal data: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (id, user_id, seq, title, body, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, n.ID, n.UserID, nextSeq, n.Title, n.Body, dataJSON, n.CreatedAt); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	n.Seq = nextSeq
	return tx.Commit(ctx)
}

func (r *NotificationRepo) ListByUser(ctx context.Context, userID string, unreadOnly bool, limit, offset int) ([]*domain.Notification, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	const baseList = `
		SELECT id, user_id, seq, title, body, data, created_at, read_at
		  FROM notifications
		 WHERE user_id = $1
		   AND ($2::BOOLEAN = FALSE OR read_at IS NULL)
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4
	`
	const baseCount = `
		SELECT COUNT(*) FROM notifications
		 WHERE user_id = $1
		   AND ($2::BOOLEAN = FALSE OR read_at IS NULL)
	`

	var total int
	if err := r.pool.QueryRow(ctx, baseCount, userID, unreadOnly).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	rows, err := r.pool.Query(ctx, baseList, userID, unreadOnly, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()

	out, err := scanNotifications(rows)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *NotificationRepo) SinceSeq(ctx context.Context, userID string, fromSeq int64, limit int) ([]*domain.Notification, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, seq, title, body, data, created_at, read_at
		  FROM notifications
		 WHERE user_id = $1 AND seq > $2
		 ORDER BY seq ASC
		 LIMIT $3
	`, userID, fromSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("query since: %w", err)
	}
	defer rows.Close()
	return scanNotifications(rows)
}

func (r *NotificationRepo) MarkRead(ctx context.Context, userID, notificationID string) error {
	cmd, err := r.pool.Exec(ctx, `
		UPDATE notifications SET read_at = now()
		 WHERE id = $1 AND user_id = $2 AND read_at IS NULL
	`, notificationID, userID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFoundOrAlreadyRead
	}
	return nil
}

func (r *NotificationRepo) CurrentSeq(ctx context.Context, userID string) (int64, error) {
	var seq int64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(last_seq, 0) FROM user_offsets WHERE user_id = $1
	`, userID).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return seq, err
}

func (r *NotificationRepo) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL
	`, userID).Scan(&n)
	return n, err
}

func scanNotifications(rows pgx.Rows) ([]*domain.Notification, error) {
	var out []*domain.Notification
	for rows.Next() {
		n := &domain.Notification{}
		var dataJSON []byte
		if err := rows.Scan(
			&n.ID, &n.UserID, &n.Seq, &n.Title, &n.Body,
			&dataJSON, &n.CreatedAt, &n.ReadAt,
		); err != nil {
			return nil, err
		}
		if len(dataJSON) > 0 {
			_ = json.Unmarshal(dataJSON, &n.Data)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
