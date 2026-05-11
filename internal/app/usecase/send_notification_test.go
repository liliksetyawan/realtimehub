package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/liliksetyawan/realtimehub/internal/app/port/mocks"
	"github.com/liliksetyawan/realtimehub/internal/app/usecase"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

func newSendUC(t *testing.T) (*usecase.SendNotification, *mocks.MockNotificationRepository, *mocks.MockPublisher) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockNotificationRepository(ctrl)
	pub := mocks.NewMockPublisher(ctrl)
	return usecase.NewSendNotification(repo, pub, zerolog.Nop()), repo, pub
}

func TestSendNotification_HappyPath(t *testing.T) {
	uc, repo, pub := newSendUC(t)

	repo.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, n *domain.Notification) error {
			assert.Equal(t, "u_alice", n.UserID)
			assert.Equal(t, "hi", n.Title)
			assert.Equal(t, "world", n.Body)
			assert.NotEmpty(t, n.ID)
			n.Seq = 7 // pretend the repo allocated seq
			return nil
		})

	pub.EXPECT().
		SendNotification(gomock.Any(), "u_alice", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, n *domain.Notification) error {
			assert.Equal(t, int64(7), n.Seq, "publisher must see the seq the repo just assigned")
			return nil
		})

	out, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{"u_alice"},
		Title:   "hi",
		Body:    "world",
	})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "u_alice", out[0].UserID)
	assert.Equal(t, int64(7), out[0].Seq)
}

func TestSendNotification_MultipleRecipientsFanOut(t *testing.T) {
	uc, repo, pub := newSendUC(t)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(3).
		DoAndReturn(func(_ context.Context, n *domain.Notification) error {
			n.Seq = 1
			return nil
		})
	pub.EXPECT().SendNotification(gomock.Any(), gomock.Any(), gomock.Any()).Times(3).Return(nil)

	out, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{"u_alice", "u_bob", "u_charlie"},
		Title:   "ping",
	})
	require.NoError(t, err)
	assert.Len(t, out, 3)
}

func TestSendNotification_PublishFailureIsBestEffort(t *testing.T) {
	uc, repo, pub := newSendUC(t)

	repo.EXPECT().Create(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, n *domain.Notification) error {
			n.Seq = 1
			return nil
		})
	pub.EXPECT().SendNotification(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("redis down"))

	out, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{"u_alice"},
		Title:   "hi",
	})
	// Persistence is the source of truth — a publish failure must not
	// fail the use case. The user catches up on next reconnect.
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestSendNotification_RejectsEmptyTitle(t *testing.T) {
	uc, _, _ := newSendUC(t)
	_, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{"u_alice"},
		Title:   "   ",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSendNotification_RejectsNoRecipients(t *testing.T) {
	uc, _, _ := newSendUC(t)
	_, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{},
		Title:   "hi",
	})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestSendNotification_TrimsWhitespaceFromUserIDs(t *testing.T) {
	uc, repo, pub := newSendUC(t)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(_ context.Context, n *domain.Notification) error {
			assert.Equal(t, "u_alice", n.UserID)
			n.Seq = 1
			return nil
		})
	pub.EXPECT().SendNotification(gomock.Any(), "u_alice", gomock.Any()).Return(nil)

	out, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{"", "  ", "u_alice"}, // only u_alice is valid after trim
		Title:   "hi",
	})
	require.NoError(t, err)
	assert.Len(t, out, 1)
}

func TestSendNotification_RepoErrorPropagates(t *testing.T) {
	uc, repo, _ := newSendUC(t)
	boom := errors.New("db down")
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(boom)

	_, err := uc.Execute(context.Background(), usecase.SendNotificationInput{
		UserIDs: []string{"u_alice"},
		Title:   "hi",
	})
	assert.ErrorIs(t, err, boom)
}
