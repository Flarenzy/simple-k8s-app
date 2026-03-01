package api

import (
	"context"
	"net"
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

func TestServeReturnsDBErrorBeforeStartingServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Fatalf("close: %v", closeErr)
		}
	}()

	err = Serve(context.Background(), Config{
		DSN:         "postgres://ipam:ipam@127.0.0.1:5432/ipam?sslmode=disable",
		AuthEnabled: true,
	}, listener)
	if err == nil {
		t.Fatal("expected serve to fail")
	}
}
