package middleware

import (
	"net/http"
)

// Tracing implements OpenTelemetry trace propagation.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Implement OpenTelemetry tracing
		next.ServeHTTP(w, r)
	})
}
