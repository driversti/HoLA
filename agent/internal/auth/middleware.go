package auth

import (
	"net/http"
	"strings"

	"github.com/driversti/hola/internal/api/respond"
)

// Middleware validates Bearer tokens on protected endpoints.
type Middleware struct {
	token string
}

func NewMiddleware(token string) *Middleware {
	return &Middleware{token: token}
}

// Wrap returns a handler that checks the Authorization header before
// delegating to the next handler. Endpoints listed in the skip set
// are passed through without authentication.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		header := r.Header.Get("Authorization")
		if header == "" {
			respond.Error(w, http.StatusUnauthorized, "missing authorization header", "UNAUTHORIZED")
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] != m.token {
			respond.Error(w, http.StatusUnauthorized, "invalid or missing bearer token", "UNAUTHORIZED")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) isPublic(path string) bool {
	return path == "/api/v1/health"
}
