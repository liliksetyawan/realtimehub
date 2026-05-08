package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/liliksetyawan/realtimehub/internal/domain"
)

func TestNotification_IsRead(t *testing.T) {
	t.Run("unread when ReadAt is nil", func(t *testing.T) {
		n := &domain.Notification{}
		assert.False(t, n.IsRead())
	})
	t.Run("read when ReadAt is set", func(t *testing.T) {
		now := time.Now()
		n := &domain.Notification{ReadAt: &now}
		assert.True(t, n.IsRead())
	})
}
