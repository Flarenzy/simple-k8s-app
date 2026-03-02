package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
		"sub": "user-1",
		"aud": audience,
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

func TestKeycloakAuthenticatorRejectsWrongAudience(t *testing.T) {
	authenticator := &keycloakAuthenticator{
		issuer:   "http://keycloak.local/realms/ipam",
		audience: "ipam-api",
		jwks:     staticKeyfunc{secret: []byte("test-secret")},
	}

	token := signToken(t, makeClaims("http://keycloak.local/realms/ipam", []string{"other-api"}), []byte("test-secret"))
	_, err := authenticator.Authenticate(context.Background(), token)
	if err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestKeycloakAuthenticatorReturnsPrincipal(t *testing.T) {
	authenticator := &keycloakAuthenticator{
		issuer:   "http://keycloak.local/realms/ipam",
		audience: "ipam-api",
		jwks:     staticKeyfunc{secret: []byte("test-secret")},
	}

	token := signToken(t, makeClaims("http://keycloak.local/realms/ipam", []string{"ipam-api"}), []byte("test-secret"))
	principal, err := authenticator.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if principal.Issuer != "http://keycloak.local/realms/ipam" {
		t.Fatalf("unexpected issuer: %v", principal.Issuer)
	}
	if principal.Subject != "user-1" {
		t.Fatalf("unexpected subject: %v", principal.Subject)
	}
}

func TestNewKeycloakAuthenticatorFailsWhenJWKSUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/certs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("no jwks"))
	}))
	defer server.Close()

	_, err := NewKeycloakAuthenticator(context.Background(), Config{
		Enabled:  true,
		Issuer:   "http://keycloak.local/realms/ipam",
		JWKSURL:  server.URL + "/certs",
		Audience: "ipam-api",
	})
	if err == nil {
		t.Fatal("expected error when jwks endpoint is unavailable")
	}
	if !strings.Contains(err.Error(), "jwks endpoint returned 502") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureJWKSReachableReturnsNilOnSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"keys":[]}`))
	}))
	defer server.Close()

	if err := ensureJWKSReachable(context.Background(), server.URL); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestStringClaimReturnsEmptyForNonStringValues(t *testing.T) {
	if got := stringClaim(jwt.MapClaims{"sub": 123}, "sub"); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	if got := stringClaim(jwt.MapClaims{}, "missing"); got != "" {
		t.Fatalf("expected empty string for missing claim, got %q", got)
	}
}

func TestNewKeycloakAuthenticatorSucceedsWithValidJWKS(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	jwk, err := jwkset.NewJWKFromKey(&privateKey.PublicKey, jwkset.JWKOptions{})
	if err != nil {
		t.Fatalf("create jwk: %v", err)
	}

	jwksPayload, err := json.Marshal(jwkset.JWKSMarshal{
		Keys: []jwkset.JWKMarshal{jwk.Marshal()},
	})
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jwksPayload)
	}))
	defer server.Close()

	authenticator, err := NewKeycloakAuthenticator(context.Background(), Config{
		Enabled:  true,
		Issuer:   "http://keycloak.local/realms/ipam",
		JWKSURL:  server.URL,
		Audience: "ipam-api",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if authenticator == nil {
		t.Fatal("expected authenticator")
	}
}

func TestNewKeycloakAuthenticatorUsesDefaultJWKSURL(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	jwk, err := jwkset.NewJWKFromKey(&privateKey.PublicKey, jwkset.JWKOptions{})
	if err != nil {
		t.Fatalf("create jwk: %v", err)
	}

	jwksPayload, err := json.Marshal(jwkset.JWKSMarshal{
		Keys: []jwkset.JWKMarshal{jwk.Marshal()},
	})
	if err != nil {
		t.Fatalf("marshal jwks: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/realms/ipam/protocol/openid-connect/certs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jwksPayload)
	}))
	defer server.Close()

	authenticator, err := NewKeycloakAuthenticator(context.Background(), Config{
		Enabled:  true,
		Issuer:   server.URL + "/realms/ipam",
		Audience: "ipam-api",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if authenticator == nil {
		t.Fatal("expected authenticator")
	}
}

func TestNewKeycloakAuthenticatorStillReturnsAuthenticatorWhenJWKSBodyIsInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"keys":[`))
	}))
	defer server.Close()

	authenticator, err := NewKeycloakAuthenticator(context.Background(), Config{
		Enabled:  true,
		Issuer:   "http://keycloak.local/realms/ipam",
		JWKSURL:  server.URL,
		Audience: "ipam-api",
	})
	if err != nil {
		t.Fatalf("expected no immediate error, got %v", err)
	}
	if authenticator == nil {
		t.Fatal("expected authenticator even when background JWKS refresh logs an error")
	}
}
