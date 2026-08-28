// Package middleware provides HTTP middlewares for the API.
package middleware

import (
	"net/http"
)

// Auth handles bearer token and API key authentication.
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement authentication
		next.ServeHTTP(w, r)
	})
}
