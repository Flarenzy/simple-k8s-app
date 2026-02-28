package http

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type AuthConfig struct {
	Enabled  bool
	Issuer   string
	Audience string
}

func (a *API) initAuth(cfg AuthConfig) {
	if !cfg.Enabled {
		return
	}
	if cfg.Issuer == "" {
		a.Logger.Warn("auth enabled but issuer is empty; disabling auth")
		return
	}

	jwksURL := cfg.Issuer + "/protocol/openid-connect/certs"
	kf, err := keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	if err != nil {
		a.Logger.Error("failed to fetch JWKS", "url", jwksURL, "err", err)
		return
	}

	a.authEnabled = true
	a.authIssuer = cfg.Issuer
	a.authAudience = cfg.Audience
	a.jwks = kf
	a.Logger.Info("auth enabled", "issuer", cfg.Issuer, "audience", cfg.Audience)
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	if !a.authEnabled || a.jwks == nil {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow unauthenticated endpoints
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" || strings.HasPrefix(r.URL.Path, "/swagger/") {
			next.ServeHTTP(w, r)
			return
		}

		authz := r.Header.Get("Authorization")
		if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(authz, "Bearer ")

		claims := jwt.MapClaims{}
		opts := []jwt.ParserOption{jwt.WithLeeway(5 * time.Second)}
		if a.authIssuer != "" {
			opts = append(opts, jwt.WithIssuer(a.authIssuer))
		}
		if a.authAudience != "" {
			opts = append(opts, jwt.WithAudience(a.authAudience))
		}

		token, err := jwt.ParseWithClaims(tokenStr, claims, a.jwks.Keyfunc, opts...)
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "claims", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
