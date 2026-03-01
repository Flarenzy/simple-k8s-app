package auth

import (
	"context"
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
