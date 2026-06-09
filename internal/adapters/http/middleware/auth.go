package middleware

import (
	"context"
	"net/http"
	"strings"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
)

type ctxKey int

const ctxKeyActor ctxKey = 0

// Auth valida o Bearer token. Modo leniente (strict=false): verifica só estrutura/exp.
// Modo strict: valida assinatura RS256 + aud + exp via JWKS interno.
func Auth(aadKey *kvCrypto.AADKey, strict bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearer := r.Header.Get("Authorization")
			token, ok := extractBearer(bearer)
			if !ok {
				writeKVError(w, http.StatusUnauthorized, "Unauthorized", "missing or invalid Authorization header")
				return
			}
			if err := aadKey.ValidateToken(token, strict); err != nil {
				writeKVError(w, http.StatusForbidden, "Forbidden", err.Error())
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyActor, token[:min(16, len(token))])
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearer(header string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(header[7:])
	if token == "" {
		return "", false
	}
	return token, true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
