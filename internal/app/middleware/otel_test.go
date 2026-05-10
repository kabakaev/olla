package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thushan/olla/internal/core/constants"
)

func TestTelemetryMiddleware_SkipHealthTracesFlagDoesNotAffectHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		path             string
		skipHealthTraces bool
	}{
		{
			name:             "health endpoint allowed when filtering enabled",
			path:             constants.DefaultHealthCheckEndpoint,
			skipHealthTraces: true,
		},
		{
			name:             "health endpoint allowed when filtering disabled",
			path:             constants.DefaultHealthCheckEndpoint,
			skipHealthTraces: false,
		},
		{
			name:             "non health endpoint allowed when filtering enabled",
			path:             "/api/models",
			skipHealthTraces: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false
			handler := TelemetryMiddleware(false, nil, tt.skipHealthTraces)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if !called {
				t.Fatal("expected wrapped handler to be called")
			}
			if rr.Code != http.StatusNoContent {
				t.Fatalf("expected status %d, got %d", http.StatusNoContent, rr.Code)
			}
		})
	}
}
