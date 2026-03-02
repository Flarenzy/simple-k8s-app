package api

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
)

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

type stubListener struct {
	acceptFn func() (net.Conn, error)
	closeFn  func() error
	addr     net.Addr
}

func (l stubListener) Accept() (net.Conn, error) {
	return l.acceptFn()
}

func (l stubListener) Close() error {
	if l.closeFn == nil {
		return nil
	}
	return l.closeFn()
}

func (l stubListener) Addr() net.Addr {
	if l.addr == nil {
		return stubAddr("127.0.0.1:0")
	}
	return l.addr
}

func TestLoadConfigUsesDefaultPortAndReadsAuthSettings(t *testing.T) {
	t.Setenv("DB_CONN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("PORT", "")
	t.Setenv("AUTH_ENABLED", "true")
	t.Setenv("KEYCLOAK_ISSUER", "https://issuer.example")
	t.Setenv("KEYCLOAK_AUDIENCE", "ipam-api")
	t.Setenv("KEYCLOAK_JWKS_URL", "https://issuer.example/jwks")

	cfg := LoadConfig()

	if cfg.Port != "4040" {
		t.Fatalf("expected default port 4040, got %q", cfg.Port)
	}
	if !cfg.AuthEnabled {
		t.Fatal("expected auth to be enabled")
	}
	if cfg.Issuer != "https://issuer.example" || cfg.Audience != "ipam-api" || cfg.JWKSURL != "https://issuer.example/jwks" {
		t.Fatalf("unexpected auth config: %+v", cfg)
	}
}

func TestLoadConfigHonorsExplicitPort(t *testing.T) {
	t.Setenv("DB_CONN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("PORT", "9090")

	cfg := LoadConfig()

	if cfg.Port != "9090" {
		t.Fatalf("expected explicit port, got %q", cfg.Port)
	}
}

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

func TestRunReturnsListenErrorForInvalidPort(t *testing.T) {
	err := Run(context.Background(), Config{Port: "invalid-port"})
	if err == nil {
		t.Fatal("expected listen error")
	}
	if !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCallsServeAndClosesListener(t *testing.T) {
	origListenFn := listenFn
	origServeFn := serveFn
	defer func() {
		listenFn = origListenFn
		serveFn = origServeFn
	}()

	listener := stubListener{
		acceptFn: func() (net.Conn, error) {
			return nil, net.ErrClosed
		},
		addr: stubAddr("127.0.0.1:4040"),
	}

	listenCalled := false
	closeCalled := false
	serveCalled := false
	listener.closeFn = func() error {
		closeCalled = true
		return nil
	}

	listenFn = func(network, address string) (net.Listener, error) {
		listenCalled = true
		if network != "tcp" || address != ":4040" {
			t.Fatalf("unexpected listen args: %s %s", network, address)
		}
		return listener, nil
	}
	serveFn = func(_ context.Context, cfg Config, got net.Listener) error {
		serveCalled = true
		if cfg.Port != "4040" {
			t.Fatalf("unexpected cfg: %+v", cfg)
		}
		if got.Addr().String() != listener.Addr().String() {
			t.Fatalf("unexpected listener addr: %s", got.Addr())
		}
		return nil
	}

	if err := Run(context.Background(), Config{Port: "4040"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !listenCalled {
		t.Fatal("expected listen to be called")
	}
	if !serveCalled {
		t.Fatal("expected serve to be called")
	}
	if !closeCalled {
		t.Fatal("expected listener to be closed")
	}
}

func TestRunReturnsServeError(t *testing.T) {
	origListenFn := listenFn
	origServeFn := serveFn
	defer func() {
		listenFn = origListenFn
		serveFn = origServeFn
	}()

	wantErr := errors.New("serve failed")
	listenFn = func(string, string) (net.Listener, error) {
		return stubListener{
			acceptFn: func() (net.Conn, error) {
				return nil, net.ErrClosed
			},
			addr: stubAddr("127.0.0.1:4040"),
		}, nil
	}
	serveFn = func(context.Context, Config, net.Listener) error {
		return wantErr
	}

	err := Run(context.Background(), Config{Port: "4040"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected serve error, got %v", err)
	}
}

func TestRunIgnoresListenerCloseError(t *testing.T) {
	origListenFn := listenFn
	origServeFn := serveFn
	defer func() {
		listenFn = origListenFn
		serveFn = origServeFn
	}()

	closeCalled := false
	listenFn = func(string, string) (net.Listener, error) {
		return stubListener{
			acceptFn: func() (net.Conn, error) {
				return nil, net.ErrClosed
			},
			closeFn: func() error {
				closeCalled = true
				return errors.New("close failed")
			},
			addr: stubAddr("127.0.0.1:4040"),
		}, nil
	}
	serveFn = func(context.Context, Config, net.Listener) error {
		return nil
	}

	if err := Run(context.Background(), Config{Port: "4040"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !closeCalled {
		t.Fatal("expected close to be called")
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

func TestServeReturnsEarlyServerError(t *testing.T) {
	serveErr := errors.New("accept failed")
	listener := stubListener{
		acceptFn: func() (net.Conn, error) {
			return nil, serveErr
		},
		addr: stubAddr("127.0.0.1:0"),
	}

	err := Serve(context.Background(), Config{
		DSN: "postgres://ipam:ipam@127.0.0.1:5432/ipam?sslmode=disable",
	}, listener)
	if !errors.Is(err, serveErr) {
		t.Fatalf("expected serve error, got %v", err)
	}
}

func TestServeReturnsNilOnContextCancellation(t *testing.T) {
	closeCh := make(chan struct{})
	var once sync.Once

	listener := stubListener{
		acceptFn: func() (net.Conn, error) {
			<-closeCh
			return nil, net.ErrClosed
		},
		closeFn: func() error {
			once.Do(func() {
				close(closeCh)
			})
			return nil
		},
		addr: stubAddr("127.0.0.1:0"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Serve(ctx, Config{
		DSN: "postgres://ipam:ipam@127.0.0.1:5432/ipam?sslmode=disable",
	}, listener)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
