package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	apiauth "github.com/Flarenzy/simple-k8s-app/internal/auth"
)

type stubAuthenticator struct {
	principal apiauth.Principal
	err       error
}

func (s stubAuthenticator) Authenticate(_ context.Context, _ string) (apiauth.Principal, error) {
	if s.err != nil {
		return apiauth.Principal{}, s.err
	}
	return s.principal, nil
}

func newTestAPI() *API {
	return &API{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Authenticator: stubAuthenticator{
			principal: apiauth.Principal{
				Issuer: "http://keycloak.local/realms/ipam",
				Claims: map[string]any{
					"iss": "http://keycloak.local/realms/ipam",
				},
			},
		},
	}
}

func TestAuthMiddlewareAllowsHealthzWithoutToken(t *testing.T) {
	api := newTestAPI()
	called := false
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !called {
		t.Fatal("expected downstream handler to be called")
	}
}

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	api := newTestAPI()
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddlewareRejectsInvalidToken(t *testing.T) {
	api := &API{
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Authenticator: stubAuthenticator{err: errors.New("bad token")},
	}
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddlewareAllowsValidToken(t *testing.T) {
	api := newTestAPI()
	called := false
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		principal, ok := apiauth.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("expected principal in context")
		}
		if principal.Issuer != "http://keycloak.local/realms/ipam" {
			t.Fatalf("unexpected issuer claim: %v", principal.Issuer)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !called {
		t.Fatal("expected downstream handler to be called")
	}
}
