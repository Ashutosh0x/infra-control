package middleware

import (
	"net/http"
)

// RateLimit implements a token bucket rate limiter.
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement rate limiting
		next.ServeHTTP(w, r)
	})
}
