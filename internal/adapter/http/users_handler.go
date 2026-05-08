package http

import (
	"net/http"

	"github.com/liliksetyawan/realtimehub/internal/adapter/auth"
)

type UsersHandler struct{}

func NewUsersHandler() *UsersHandler { return &UsersHandler{} }

// List returns the demo users for the admin "send to" picker.
type userDTO struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (h *UsersHandler) List(w http.ResponseWriter, _ *http.Request) {
	all := auth.AllUserIDs()
	out := make([]userDTO, len(all))
	for i, u := range all {
		out[i] = userDTO{ID: u.ID, Username: u.Username, Role: u.Role}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}
