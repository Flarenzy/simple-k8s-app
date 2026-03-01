package auth

import "context"

type Authenticator interface {
	Authenticate(ctx context.Context, bearerToken string) (Principal, error)
}
