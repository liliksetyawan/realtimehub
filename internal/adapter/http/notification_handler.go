package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	"github.com/liliksetyawan/realtimehub/internal/app/usecase"
	"github.com/liliksetyawan/realtimehub/internal/domain"
)

type NotificationHandler struct {
	send     *usecase.SendNotification
	list     *usecase.ListNotifications
	markRead *usecase.MarkRead
	log      zerolog.Logger
}

func NewNotificationHandler(send *usecase.SendNotification, list *usecase.ListNotifications, markRead *usecase.MarkRead, log zerolog.Logger) *NotificationHandler {
	return &NotificationHandler{
		send:     send,
		list:     list,
		markRead: markRead,
		log:      log.With().Str("component", "notif-handler").Logger(),
	}
}

type sendReq struct {
	UserIDs []string          `json:"user_ids"`
	Title   string            `json:"title"`
	Body    string            `json:"body"`
	Data    map[string]string `json:"data,omitempty"`
}

type sentDTO struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Seq    int64  `json:"seq"`
}

// AdminSend is admin-only. Wired with RequireAdmin middleware.
func (h *NotificationHandler) AdminSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req sendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", err.Error())
		return
	}

	out, err := h.send.Execute(r.Context(), usecase.SendNotificationInput{
		UserIDs: req.UserIDs,
		Title:   req.Title,
		Body:    req.Body,
		Data:    req.Data,
	})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			writeErr(w, http.StatusBadRequest, "invalid_input", "title + user_ids required")
			return
		}
		h.log.Error().Err(err).Msg("send")
		writeErr(w, http.StatusInternalServerError, "internal", "could not send")
		return
	}
	dtos := make([]sentDTO, len(out))
	for i, s := range out {
		dtos[i] = sentDTO{ID: s.ID, UserID: s.UserID, Seq: s.Seq}
	}
	writeJSON(w, http.StatusCreated, map[string]any{"sent": dtos})
}

type notifDTO struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	Seq       int64             `json:"seq"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data,omitempty"`
	CreatedAt string            `json:"created_at"`
	ReadAt    string            `json:"read_at,omitempty"`
}

// ListMine returns notifications for the authenticated user.
// Wired with RequireAuth middleware so user_id comes from JWT.
func (h *NotificationHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(CtxUserID).(string)
	q := r.URL.Query()
	unread := q.Get("unread") == "true"
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	out, err := h.list.Execute(r.Context(), usecase.ListNotificationsInput{
		UserID: userID, UnreadOnly: unread, Limit: limit, Offset: offset,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("list")
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	items := make([]notifDTO, len(out.Items))
	for i, n := range out.Items {
		read := ""
		if n.ReadAt != nil {
			read = n.ReadAt.UTC().Format("2006-01-02T15:04:05.999999999Z")
		}
		items[i] = notifDTO{
			ID: n.ID, UserID: n.UserID, Seq: n.Seq,
			Title: n.Title, Body: n.Body, Data: n.Data,
			CreatedAt: n.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z"),
			ReadAt:    read,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":         items,
		"total":        out.Total,
		"unread_count": out.UnreadCount,
	})
}

// MarkRead marks /v1/notifications/:id/read for the authenticated user.
func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(CtxUserID).(string)
	// Path: /v1/notifications/{id}/read
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[3] == "" {
		writeErr(w, http.StatusBadRequest, "bad_path", "missing notification id")
		return
	}
	notifID := parts[2]

	if err := h.markRead.Execute(r.Context(), userID, notifID); err != nil {
		if errors.Is(err, domain.ErrNotFoundOrAlreadyRead) {
			writeErr(w, http.StatusNotFound, "not_found", "notification not found or already read")
			return
		}
		h.log.Error().Err(err).Msg("mark read")
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
