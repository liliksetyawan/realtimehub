package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/liliksetyawan/realtimehub/internal/app/port/mocks"
	"github.com/liliksetyawan/realtimehub/internal/app/usecase"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

func TestListNotifications_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	uc := usecase.NewListNotifications(repo)

	want := []*domain.Notification{
		{ID: "n1", UserID: "u_alice", Seq: 2, Title: "hello"},
		{ID: "n2", UserID: "u_alice", Seq: 1, Title: "earlier"},
	}
	repo.EXPECT().ListByUser(gomock.Any(), "u_alice", false, 50, 0).
		Return(want, 2, nil)
	repo.EXPECT().UnreadCount(gomock.Any(), "u_alice").Return(1, nil)

	out, err := uc.Execute(context.Background(), usecase.ListNotificationsInput{
		UserID: "u_alice",
		Limit:  50,
	})
	require.NoError(t, err)
	assert.Equal(t, want, out.Items)
	assert.Equal(t, 2, out.Total)
	assert.Equal(t, 1, out.UnreadCount)
}

func TestListNotifications_PassesUnreadOnlyFlag(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	uc := usecase.NewListNotifications(repo)

	repo.EXPECT().ListByUser(gomock.Any(), "u_alice", true, 20, 0).
		Return(nil, 0, nil)
	repo.EXPECT().UnreadCount(gomock.Any(), "u_alice").Return(0, nil)

	_, err := uc.Execute(context.Background(), usecase.ListNotificationsInput{
		UserID:     "u_alice",
		UnreadOnly: true,
		Limit:      20,
	})
	require.NoError(t, err)
}

func TestListNotifications_PropagatesRepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	uc := usecase.NewListNotifications(repo)

	boom := errors.New("db down")
	repo.EXPECT().ListByUser(gomock.Any(), "u_alice", false, gomock.Any(), gomock.Any()).
		Return(nil, 0, boom)

	_, err := uc.Execute(context.Background(), usecase.ListNotificationsInput{UserID: "u_alice"})
	assert.ErrorIs(t, err, boom)
}

func TestMarkRead_Delegates(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	uc := usecase.NewMarkRead(repo)

	repo.EXPECT().MarkRead(gomock.Any(), "u_alice", "n1").Return(nil)
	require.NoError(t, uc.Execute(context.Background(), "u_alice", "n1"))
}

func TestMarkRead_PropagatesNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	uc := usecase.NewMarkRead(repo)

	repo.EXPECT().MarkRead(gomock.Any(), "u_alice", "n1").
		Return(domain.ErrNotFoundOrAlreadyRead)
	err := uc.Execute(context.Background(), "u_alice", "n1")
	assert.ErrorIs(t, err, domain.ErrNotFoundOrAlreadyRead)
}
