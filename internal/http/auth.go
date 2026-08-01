package http

import (
	"net/http"
	"strings"

	apiauth "github.com/Flarenzy/simple-k8s-app/internal/auth"
)

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		principal := apiauth.DefaultAdminPrincipal()
		if a.Authenticator != nil {
			authz := r.Header.Get("Authorization")
			if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(authz, "Bearer ")
			var err error
			principal, err = a.Authenticator.Authenticate(r.Context(), tokenStr)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
		}

		if !authorized(principal, r.Method) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		ctx := apiauth.WithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicPath(path string) bool {
	return path == "/healthz" || path == "/readyz" || strings.HasPrefix(path, "/swagger/")
}

func authorized(principal apiauth.Principal, method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead:
		return principal.CanRead()
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return principal.CanWrite()
	case http.MethodDelete:
		return principal.CanDelete()
	default:
		return false
	}
}
