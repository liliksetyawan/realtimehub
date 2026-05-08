// Package auth holds the JWT authenticator used at handshake. Phase 1
// ships a permissive Anonymous authenticator so the WebSocket plumbing
// can be exercised end-to-end before JWT lands in Phase 2.
package auth

import (
	"errors"
	"net/http"
)

// Anonymous reads ?user_id= from the query and accepts whatever it finds.
// USE ONLY IN DEV. Real auth lands in Phase 2 (JWT).
type Anonymous struct{}

func (Anonymous) Authenticate(r *http.Request) (string, error) {
	uid := r.URL.Query().Get("user_id")
	if uid == "" {
		return "", errors.New("user_id query param required (anonymous dev auth)")
	}
	return uid, nil
}
