// Package http holds the REST handlers (login, send-notification admin
// API, list/read endpoints) registered on the same nbio mux as the
// WebSocket upgrade.
package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/liliksetyawan/realtimehub/internal/adapter/auth"
)

type AuthHandler struct {
	jwt *auth.JWT
	log zerolog.Logger
}

func NewAuthHandler(jwt *auth.JWT, log zerolog.Logger) *AuthHandler {
	return &AuthHandler{jwt: jwt, log: log.With().Str("component", "auth-handler").Logger()}
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "invalid request body")
		return
	}
	user := auth.Lookup(req.Username, req.Password)
	if user == nil {
		writeErr(w, http.StatusUnauthorized, "invalid_credentials", "username or password incorrect")
		return
	}

	token, exp, err := h.jwt.Issue(user)
	if err != nil {
		h.log.Error().Err(err).Msg("issue jwt")
		writeErr(w, http.StatusInternalServerError, "internal", "could not issue token")
		return
	}

	writeJSON(w, http.StatusOK, loginResp{
		Token:     token,
		ExpiresAt: exp,
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
	})
}

// Me returns the current user's profile from a Bearer token. Useful for
// the SPA to validate cached tokens on boot.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, err := h.jwt.FromBearer(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"user_id":  claims.Subject,
		"username": claims.Username,
		"role":     claims.Role,
	})
}
