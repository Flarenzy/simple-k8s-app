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
	if cfg.Audience == "" {
		return nil, fmt.Errorf("auth enabled but audience is empty")
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
		Username: stringClaim(claims, "preferred_username"),
		Audience: claims["aud"],
		Roles:    clientRoles(claims, a.audience),
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

func clientRoles(claims jwt.MapClaims, clientID string) []Role {
	resourceAccess, ok := claims["resource_access"].(map[string]any)
	if !ok {
		return nil
	}
	clientAccess, ok := resourceAccess[clientID].(map[string]any)
	if !ok {
		return nil
	}
	values, ok := clientAccess["roles"].([]any)
	if !ok {
		return nil
	}

	roles := make([]Role, 0, len(values))
	for _, value := range values {
		name, ok := value.(string)
		if !ok {
			continue
		}
		switch Role(name) {
		case RoleAdmin, RoleReadOnly, RoleEditor:
			roles = append(roles, Role(name))
		}
	}
	return roles
}
