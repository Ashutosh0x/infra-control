package middleware

import (
	"net/http"
)

// RBAC checks if the authenticated user has the required roles.
func RBAC(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement role-based access control
			next.ServeHTTP(w, r)
		})
	}
}
