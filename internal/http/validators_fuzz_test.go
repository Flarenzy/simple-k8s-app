package http

import (
	"net/http/httptest"
	"strconv"
	"testing"
)

func FuzzParsePathInt64(f *testing.F) {
	f.Add("")
	f.Add("42")
	f.Add("-7")
	f.Add("not-a-number")

	f.Fuzz(func(t *testing.T, value string) {
		req := httptest.NewRequest("GET", "/", nil)
		req.SetPathValue("id", value)

		got, err := parsePathInt64(req, "id")
		want, parseErr := strconv.ParseInt(value, 10, 64)

		if value == "" {
			if err == nil {
				t.Fatal("expected error for empty value")
			}
			return
		}

		if parseErr != nil {
			if err == nil {
				t.Fatalf("expected parse error for %q", value)
			}
			return
		}

		if err != nil {
			t.Fatalf("unexpected error for %q: %v", value, err)
		}
		if got != want {
			t.Fatalf("expected %d, got %d", want, got)
		}
	})
}

func FuzzParseIPAddressID(f *testing.F) {
	f.Add("550e8400-e29b-41d4-a716-446655440000")
	f.Add("not-a-uuid")
	f.Add("")

	f.Fuzz(func(t *testing.T, value string) {
		got, err := parseIPAddressID(value)
		if err == nil && string(got) != value {
			t.Fatalf("expected parsed id %q, got %q", value, got)
		}
	})
}
