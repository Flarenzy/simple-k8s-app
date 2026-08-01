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
				Roles:  []apiauth.Role{apiauth.RoleAdmin},
				Claims: map[string]any{
					"iss": "http://keycloak.local/realms/ipam",
				},
			},
		},
	}
}

func TestAuthMiddlewareUsesAdminIdentityWhenAuthenticationIsDisabled(t *testing.T) {
	api := &API{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := apiauth.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("expected default principal in context")
		}
		if principal.Username != "admin" || !principal.HasRole(apiauth.RoleAdmin) {
			t.Fatalf("unexpected default principal: %+v", principal)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/subnets/1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
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

func TestAuthMiddlewareEnforcesApplicationRoles(t *testing.T) {
	tests := []struct {
		name   string
		role   apiauth.Role
		method string
		want   int
	}{
		{name: "read-only reads", role: apiauth.RoleReadOnly, method: http.MethodGet, want: http.StatusNoContent},
		{name: "read-only cannot create", role: apiauth.RoleReadOnly, method: http.MethodPost, want: http.StatusForbidden},
		{name: "read-only cannot edit", role: apiauth.RoleReadOnly, method: http.MethodPatch, want: http.StatusForbidden},
		{name: "read-only cannot delete", role: apiauth.RoleReadOnly, method: http.MethodDelete, want: http.StatusForbidden},
		{name: "editor reads", role: apiauth.RoleEditor, method: http.MethodGet, want: http.StatusNoContent},
		{name: "editor creates", role: apiauth.RoleEditor, method: http.MethodPost, want: http.StatusNoContent},
		{name: "editor edits", role: apiauth.RoleEditor, method: http.MethodPatch, want: http.StatusNoContent},
		{name: "editor cannot delete", role: apiauth.RoleEditor, method: http.MethodDelete, want: http.StatusForbidden},
		{name: "admin deletes", role: apiauth.RoleAdmin, method: http.MethodDelete, want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &API{
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				Authenticator: stubAuthenticator{
					principal: apiauth.Principal{Roles: []apiauth.Role{tt.role}},
				},
			}
			handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))

			req := httptest.NewRequest(tt.method, "/api/v1/subnets", nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, rec.Code)
			}
		})
	}
}

func TestAuthMiddlewareRejectsAuthenticatedUserWithoutApplicationRole(t *testing.T) {
	api := &API{
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Authenticator: stubAuthenticator{principal: apiauth.Principal{}},
	}
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
}
