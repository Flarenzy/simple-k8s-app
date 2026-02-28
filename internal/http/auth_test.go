package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
)

type staticKeyfunc struct {
	secret []byte
}

func (s staticKeyfunc) Keyfunc(_ *jwt.Token) (any, error) {
	return s.secret, nil
}

func (s staticKeyfunc) KeyfuncCtx(_ context.Context) jwt.Keyfunc {
	return s.Keyfunc
}

func (s staticKeyfunc) Storage() jwkset.Storage {
	return nil
}

func (s staticKeyfunc) VerificationKeySet(_ context.Context) (jwt.VerificationKeySet, error) {
	return jwt.VerificationKeySet{}, nil
}

func newTestAPI() *API {
	return &API{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		authEnabled:  true,
		authIssuer:   "http://keycloak.local/realms/ipam",
		authAudience: "ipam-api",
		jwks:         staticKeyfunc{secret: []byte("test-secret")},
	}
}

func signToken(t *testing.T, claims jwt.MapClaims, secret []byte) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func makeClaims(issuer string, audience any) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss": issuer,
		"aud": audience,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
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
	api := newTestAPI()
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

func TestAuthMiddlewareRejectsWrongIssuer(t *testing.T) {
	api := newTestAPI()
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	token := signToken(t, makeClaims("http://wrong-issuer/realms/ipam", []string{"ipam-api"}), []byte("test-secret"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}

func TestAuthMiddlewareRejectsWrongAudience(t *testing.T) {
	api := newTestAPI()
	handler := api.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	token := signToken(t, makeClaims("http://keycloak.local/realms/ipam", []string{"other-api"}), []byte("test-secret"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
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
		claims, ok := r.Context().Value("claims").(jwt.MapClaims)
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims["iss"] != "http://keycloak.local/realms/ipam" {
			t.Fatalf("unexpected issuer claim: %v", claims["iss"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	token := signToken(t, makeClaims("http://keycloak.local/realms/ipam", []string{"ipam-api"}), []byte("test-secret"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/subnets", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if !called {
		t.Fatal("expected downstream handler to be called")
	}
}
