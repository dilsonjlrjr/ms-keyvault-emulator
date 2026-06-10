package middleware

import (
	"context"
	"net/http"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

const ctxKeyVault ctxKey = 1
var CtxKeyVault = ctxKeyVault

func VaultFromContext(ctx context.Context) *domain.Vault {
	v, _ := ctx.Value(ctxKeyVault).(*domain.Vault)
	return v
}

func VaultResolver(vaultRepo ports.VaultRepository, baseDomain, defaultVault string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host

			name := domain.VaultNameFromHost(host, baseDomain)

			var vault *domain.Vault
			var err error

			if name != "" {
				vault, err = vaultRepo.GetByName(r.Context(), name)
				if err != nil {
					name = ""
				}
			}

			if name == "" {
				vault, err = vaultRepo.GetByName(r.Context(), defaultVault)
				if err != nil {
					vault, err = vaultRepo.GetByHost(r.Context(), host)
				}
			}

			if vault == nil || err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":{"code":"VaultNotFound","message":"vault not found for host: ` + host + `"}}`))
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyVault, vault)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
