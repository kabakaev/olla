package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thushan/olla/internal/config"
	"github.com/thushan/olla/internal/core/constants"
)

func TestClientAuthPolicyAllow(t *testing.T) {
	t.Parallel()

	policy := newClientAuthPolicy(config.ClientAuthConfig{
		Enabled:              true,
		AuthorizationHeaders: []string{"Bearer allowed-1", "Bearer allowed-2"},
	})

	tests := []struct {
		name  string
		auth  string
		allow bool
	}{
		{name: "matching header allowed", auth: "Bearer allowed-1", allow: true},
		{name: "wrong header rejected", auth: "Bearer wrong", allow: false},
		{name: "missing header rejected", auth: "", allow: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/olla/proxy/v1/chat/completions", nil)
			if tc.auth != "" {
				req.Header.Set(constants.HeaderAuthorization, tc.auth)
			}
			if got := policy.allow(req); got != tc.allow {
				t.Fatalf("allow() = %v, want %v", got, tc.allow)
			}
		})
	}
}

func TestSecurityAdaptersCreateChainMiddleware_ClientAuth(t *testing.T) {
	t.Parallel()

	t.Run("unauthorized", func(t *testing.T) {
		t.Parallel()
		adapters := &SecurityAdapters{
			logger:     &mockStyledLogger{},
			clientAuth: newClientAuthPolicy(config.ClientAuthConfig{Enabled: true, AuthorizationHeaders: []string{"Bearer allowed"}}),
		}
		handler := adapters.CreateChainMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		req := httptest.NewRequest(http.MethodPost, "/olla/proxy/v1/chat/completions", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("authorized", func(t *testing.T) {
		t.Parallel()
		nextCalled := false
		adapters := &SecurityAdapters{
			logger:     &mockStyledLogger{},
			clientAuth: newClientAuthPolicy(config.ClientAuthConfig{Enabled: true, AuthorizationHeaders: []string{"Bearer allowed"}}),
		}
		handler := adapters.CreateChainMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		}))
		req := httptest.NewRequest(http.MethodPost, "/olla/proxy/v1/chat/completions", nil)
		req.Header.Set(constants.HeaderAuthorization, "Bearer allowed")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if !nextCalled {
			t.Fatal("expected downstream handler call")
		}
	})
}
