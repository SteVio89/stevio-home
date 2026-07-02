package middleware

import "net/http"

// SecurityHeaders sets common security response headers on every response.
// Focused on API security. Apps serving HTML should add Strict-Transport-Security
// and Content-Security-Policy via additional middleware.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
