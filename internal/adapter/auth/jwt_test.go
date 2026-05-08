package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liliksetyawan/realtimehub/internal/adapter/auth"
)

func newJWT(t *testing.T) *auth.JWT {
	t.Helper()
	return auth.NewJWT("test-secret-256-bits", time.Hour)
}

var sampleUser = &auth.User{
	ID:       "u_test",
	Username: "testuser",
	Role:     "user",
}

func TestJWT_IssueVerifyRoundtrip(t *testing.T) {
	j := newJWT(t)
	tok, exp, err := j.Issue(sampleUser)
	require.NoError(t, err)
	require.NotEmpty(t, tok)
	assert.WithinDuration(t, time.Now().Add(time.Hour), exp, 5*time.Second)

	claims, err := j.Verify(tok)
	require.NoError(t, err)
	assert.Equal(t, "u_test", claims.Subject)
	assert.Equal(t, "testuser", claims.Username)
	assert.Equal(t, "user", claims.Role)
}

func TestJWT_RejectsTokenSignedWithDifferentSecret(t *testing.T) {
	signer := newJWT(t)
	tok, _, err := signer.Issue(sampleUser)
	require.NoError(t, err)

	verifier := auth.NewJWT("different-secret", time.Hour)
	_, err = verifier.Verify(tok)
	assert.Error(t, err)
}

func TestJWT_RejectsExpiredToken(t *testing.T) {
	short := auth.NewJWT("test-secret", time.Millisecond)
	tok, _, err := short.Issue(sampleUser)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	_, err = short.Verify(tok)
	assert.Error(t, err, "expired tokens must be rejected")
}

func TestJWT_RejectsMalformedToken(t *testing.T) {
	j := newJWT(t)
	_, err := j.Verify("this.is.not.a.jwt")
	assert.Error(t, err)
}

func TestJWT_AuthenticateReadsTokenFromQuery(t *testing.T) {
	j := newJWT(t)
	tok, _, err := j.Issue(sampleUser)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://x/ws?token="+tok, nil)
	uid, err := j.Authenticate(req)
	require.NoError(t, err)
	assert.Equal(t, "u_test", uid)
}

func TestJWT_AuthenticateRejectsMissingToken(t *testing.T) {
	j := newJWT(t)
	req := httptest.NewRequest(http.MethodGet, "http://x/ws", nil)
	_, err := j.Authenticate(req)
	assert.Error(t, err)
}

func TestJWT_FromBearerHappyPath(t *testing.T) {
	j := newJWT(t)
	tok, _, err := j.Issue(sampleUser)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://x/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	claims, err := j.FromBearer(req)
	require.NoError(t, err)
	assert.Equal(t, "u_test", claims.Subject)
}

func TestJWT_FromBearerRejectsBadHeaders(t *testing.T) {
	j := newJWT(t)
	cases := map[string]string{
		"missing":   "",
		"no-prefix": "Token abc",
		"wrong-prefix": "Basic abc",
	}
	for name, header := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://x/v1/me", nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			_, err := j.FromBearer(req)
			assert.Error(t, err)
		})
	}
}
