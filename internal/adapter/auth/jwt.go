package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT issues and verifies HS256 tokens. Used for both:
//   - WebSocket handshake (?token=...)
//   - REST API auth (Authorization: Bearer ...)
type JWT struct {
	secret []byte
	ttl    time.Duration
}

// Claims is the JWT body. Standard claims (sub, exp) plus our role.
type Claims struct {
	Role     string `json:"role"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func NewJWT(secret string, ttl time.Duration) *JWT {
	return &JWT{secret: []byte(secret), ttl: ttl}
}

// Issue creates a signed JWT for a user. Token is valid for ttl.
func (j *JWT) Issue(u *User) (string, time.Time, error) {
	exp := time.Now().Add(j.ttl)
	claims := Claims{
		Role:     u.Role,
		Username: u.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "realtimehub",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, exp, nil
}

// Verify parses + validates a token. Returns the claims on success.
func (j *JWT) Verify(token string) (*Claims, error) {
	c := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

// Authenticate satisfies wsadapter.Authenticator. Reads token from
// `?token=` query (browsers can't set headers during WS upgrade) and
// returns the user id (subject claim).
func (j *JWT) Authenticate(r *http.Request) (string, error) {
	tok := r.URL.Query().Get("token")
	if tok == "" {
		return "", errors.New("missing token")
	}
	claims, err := j.Verify(tok)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}
	return claims.Subject, nil
}

// FromBearer pulls the JWT out of an `Authorization: Bearer ...` header
// and verifies it. Used by REST handlers.
func (j *JWT) FromBearer(r *http.Request) (*Claims, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return nil, errors.New("missing Authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return nil, errors.New("malformed Authorization header")
	}
	return j.Verify(strings.TrimPrefix(h, prefix))
}
