package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessAndRouteBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ping   func(context.Context) error
		status int
	}{
		{"ready", func(context.Context) error { return nil }, 200},
		{"database down", func(context.Context) error { return errors.New("secret connection detail") }, 503},
		{"not configured", nil, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := New(Dependencies{Ping: tc.ping})
			r := httptest.NewRecorder()
			h.ServeHTTP(r, httptest.NewRequest("GET", "/api/health/ready", nil))
			if r.Code != tc.status {
				t.Fatalf("status=%d body=%s", r.Code, r.Body)
			}
			if strings.Contains(r.Body.String(), "secret connection detail") {
				t.Fatal("database error leaked")
			}
			if r.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("health must not be cached")
			}
		})
	}
	h, _ := New(Dependencies{})
	for _, path := range []string{"/api/v1/notes", "/auth/login", "/api/unknown"} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		if r.Code != 404 {
			t.Fatalf("unimplemented %s must not be exposed: %d", path, r.Code)
		}
	}
}
