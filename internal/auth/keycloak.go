package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type keycloakAuthenticator struct {
	issuer   string
	audience string
	jwks     keyfunc.Keyfunc
}

func NewKeycloakAuthenticator(ctx context.Context, cfg Config) (Authenticator, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("auth enabled but issuer is empty")
	}

	jwksURL := cfg.JWKSURL
	if jwksURL == "" {
		jwksURL = cfg.Issuer + "/protocol/openid-connect/certs"
	}

	if err := ensureJWKSReachable(ctx, jwksURL); err != nil {
		return nil, fmt.Errorf("fetch jwks from %s: %w", jwksURL, err)
	}

	kf, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("fetch jwks from %s: %w", jwksURL, err)
	}

	return &keycloakAuthenticator{
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
		jwks:     kf,
	}, nil
}

func ensureJWKSReachable(ctx context.Context, jwksURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_, err = io.Copy(io.Discard, resp.Body)
		if err != nil {
			return
		}
		err = resp.Body.Close()
		if err != nil {
			return
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned %d", resp.StatusCode)
	}

	return nil
}

func (a *keycloakAuthenticator) Authenticate(_ context.Context, bearerToken string) (Principal, error) {
	claims := jwt.MapClaims{}
	opts := []jwt.ParserOption{jwt.WithLeeway(5 * time.Second)}
	if a.issuer != "" {
		opts = append(opts, jwt.WithIssuer(a.issuer))
	}
	if a.audience != "" {
		opts = append(opts, jwt.WithAudience(a.audience))
	}

	token, err := jwt.ParseWithClaims(bearerToken, claims, a.jwks.Keyfunc, opts...)
	if err != nil || !token.Valid {
		return Principal{}, ErrInvalidToken
	}

	return Principal{
		Issuer:   stringClaim(claims, "iss"),
		Subject:  stringClaim(claims, "sub"),
		Audience: claims["aud"],
		Claims:   claims,
	}, nil
}

func stringClaim(claims jwt.MapClaims, key string) string {
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}
	return value
}
