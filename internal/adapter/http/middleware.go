package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/liliksetyawan/realtimehub/internal/adapter/auth"
)

// ctxKey scopes context values so they can't collide with std lib keys.
type ctxKey string

const (
	CtxUserID   ctxKey = "user_id"
	CtxRole     ctxKey = "role"
	CtxUsername ctxKey = "username"
)

// RequireAuth wraps a handler with JWT verification. Sets user_id / role
// / username on the request context for downstream handlers.
func RequireAuth(jwt *auth.JWT, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := jwt.FromBearer(r)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxUserID, claims.Subject)
		ctx = context.WithValue(ctx, CtxRole, claims.Role)
		ctx = context.WithValue(ctx, CtxUsername, claims.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin wraps a handler with both auth and a role check.
func RequireAdmin(jwt *auth.JWT, next http.Handler) http.Handler {
	return RequireAuth(jwt, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := r.Context().Value(CtxRole).(string)
		if role != "admin" {
			writeErr(w, http.StatusForbidden, "forbidden", "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// CORS adds permissive CORS for the dev React SPA. Production would
// narrow the origin list.
func CORS(origins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "300")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
