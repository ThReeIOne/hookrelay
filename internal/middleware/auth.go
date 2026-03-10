package middleware

import (
	"net/http"
	"strings"
)

// Auth returns a middleware that validates API key from Authorization or X-API-Key header.
func Auth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Authorization")
			if key != "" {
				key = strings.TrimPrefix(key, "Bearer ")
			} else {
				key = r.Header.Get("X-API-Key")
			}

			if key == "" || key != apiKey {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
