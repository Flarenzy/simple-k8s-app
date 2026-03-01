package api

import (
	"context"
	"testing"
)

func TestNewAuthenticatorDisabledReturnsNil(t *testing.T) {
	authenticator, err := newAuthenticator(context.Background(), Config{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if authenticator != nil {
		t.Fatal("expected nil authenticator when auth is disabled")
	}
}

func TestNewAuthenticatorEnabledWithoutIssuerFails(t *testing.T) {
	_, err := newAuthenticator(context.Background(), Config{AuthEnabled: true})
	if err == nil {
		t.Fatal("expected error when auth is enabled without issuer")
	}
}
