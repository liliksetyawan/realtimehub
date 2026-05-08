package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liliksetyawan/realtimehub/internal/adapter/auth"
)

func TestLookup_HappyPath(t *testing.T) {
	u := auth.Lookup("alice", "password123")
	require.NotNil(t, u)
	assert.Equal(t, "u_alice", u.ID)
	assert.Equal(t, "user", u.Role)
}

func TestLookup_AdminUser(t *testing.T) {
	u := auth.Lookup("admin", "admin123")
	require.NotNil(t, u)
	assert.Equal(t, "admin", u.Role)
}

func TestLookup_WrongPassword(t *testing.T) {
	assert.Nil(t, auth.Lookup("alice", "wrong"))
}

func TestLookup_UnknownUser(t *testing.T) {
	assert.Nil(t, auth.Lookup("nobody", "x"))
}

func TestAllUserIDs_ExcludesAdmin(t *testing.T) {
	users := auth.AllUserIDs()
	assert.NotEmpty(t, users)
	for _, u := range users {
		assert.NotEqual(t, "admin", u.Role, "admin must be filtered out of recipient list")
	}
}
