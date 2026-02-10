package api

import "net/http"

// AuthMode controls how write endpoints are authenticated.
type AuthMode string

const (
	AuthModeNone   AuthMode = "none"
	AuthModeAPIKey AuthMode = "api-key"
	AuthModeOIDC   AuthMode = "oidc-proxy"
)

// CORS wraps an http.Handler with CORS headers for cross-origin requests.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// APIKeyAuth returns middleware that validates the X-API-Key header.
// If key is empty, the middleware is a no-op (all requests pass through).
func APIKeyAuth(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-API-Key") != key {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// WriteAuth returns middleware that protects write endpoints based on the configured auth mode.
func WriteAuth(mode AuthMode, apiKey string) func(http.Handler) http.Handler {
	switch mode {
	case AuthModeNone:
		return func(next http.Handler) http.Handler { return next }
	case AuthModeOIDC:
		return OIDCProxyAuth
	default: // api-key
		return APIKeyAuth(apiKey)
	}
}

// OIDCProxyAuth returns middleware that validates headers set by an upstream OIDC proxy
// (IAP, OAuth2 Proxy, Pomerium, Authelia).
func OIDCProxyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-Email") == "" && r.Header.Get("X-Forwarded-User") == "" {
			http.Error(w, "unauthorized: missing proxy auth headers", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
