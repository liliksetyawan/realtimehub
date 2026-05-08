package usecase

import (
	"context"

	"github.com/liliksetyawan/realtimehub/internal/app/port"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

type ListNotifications struct {
	repo port.NotificationRepository
}

func NewListNotifications(repo port.NotificationRepository) *ListNotifications {
	return &ListNotifications{repo: repo}
}

type ListNotificationsInput struct {
	UserID     string
	UnreadOnly bool
	Limit      int
	Offset     int
}

type ListNotificationsOutput struct {
	Items       []*domain.Notification
	Total       int
	UnreadCount int
}

func (uc *ListNotifications) Execute(ctx context.Context, in ListNotificationsInput) (*ListNotificationsOutput, error) {
	items, total, err := uc.repo.ListByUser(ctx, in.UserID, in.UnreadOnly, in.Limit, in.Offset)
	if err != nil {
		return nil, err
	}
	unread, err := uc.repo.UnreadCount(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	return &ListNotificationsOutput{Items: items, Total: total, UnreadCount: unread}, nil
}

type MarkRead struct {
	repo port.NotificationRepository
}

func NewMarkRead(repo port.NotificationRepository) *MarkRead { return &MarkRead{repo: repo} }

func (uc *MarkRead) Execute(ctx context.Context, userID, notificationID string) error {
	return uc.repo.MarkRead(ctx, userID, notificationID)
}
