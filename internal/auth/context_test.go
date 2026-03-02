package auth

import (
	"context"
	"testing"
)

func TestWithPrincipalAndPrincipalFromContext(t *testing.T) {
	principal := Principal{
		Issuer:  "issuer",
		Subject: "user-1",
	}

	ctx := WithPrincipal(context.Background(), principal)
	got, ok := PrincipalFromContext(ctx)
	if !ok {
		t.Fatal("expected principal in context")
	}
	if got.Issuer != principal.Issuer || got.Subject != principal.Subject {
		t.Fatalf("unexpected principal: %+v", got)
	}
}

func TestPrincipalFromContextReturnsFalseWhenMissing(t *testing.T) {
	_, ok := PrincipalFromContext(context.Background())
	if ok {
		t.Fatal("expected no principal")
	}
}
