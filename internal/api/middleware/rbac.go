package middleware

import (
	"net/http"
)

// RBAC checks if the authenticated user has the required roles.
// RBAC gates a handler on the caller holding one of the given roles.
//
// The roles are not yet enforced; the parameter is named _ until the
// authorisation layer exists, so that the signature callers already use
// does not have to change when it does.
func RBAC(_ ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: Implement role-based access control
			next.ServeHTTP(w, r)
		})
	}
}
