package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/driversti/hola/internal/auth"
)

func TestMiddleware(t *testing.T) {
	mw := auth.NewMiddleware("test-token")
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw.Wrap(ok)

	tests := []struct {
		name       string
		path       string
		authHeader string
		wantStatus int
	}{
		{"health is public", "/api/v1/health", "", http.StatusOK},
		{"missing header", "/api/v1/stacks", "", http.StatusUnauthorized},
		{"invalid token", "/api/v1/stacks", "Bearer wrong", http.StatusUnauthorized},
		{"valid token", "/api/v1/stacks", "Bearer test-token", http.StatusOK},
		{"no bearer prefix", "/api/v1/stacks", "test-token", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
