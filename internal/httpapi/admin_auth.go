package httpapi

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

func withBearerAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			writeError(w, http.StatusForbidden, nil)
			return
		}

		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="booky-admin"`)
			writeError(w, http.StatusUnauthorized, nil)
			return
		}

		provided := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if subtle.ConstantTimeCompare([]byte(token), []byte(provided)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="booky-admin"`)
			writeError(w, http.StatusUnauthorized, nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}
