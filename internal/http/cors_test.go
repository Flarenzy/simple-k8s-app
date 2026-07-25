package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddlewareAllowsConfiguredOriginAndPreflight(t *testing.T) {
	api := &API{CORSAllowedOrigins: []string{"http://simplek8sapp.lan"}}
	handler := api.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/subnets", nil)
	req.Header.Set("Origin", "http://simplek8sapp.lan")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://simplek8sapp.lan" {
		t.Fatalf("unexpected allow origin %q", got)
	}
}

func TestCORSMiddlewareRejectsDisallowedPreflight(t *testing.T) {
	api := &API{CORSAllowedOrigins: []string{"http://simplek8sapp.lan"}}
	handler := api.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/subnets", nil)
	req.Header.Set("Origin", "http://evil.example")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}
