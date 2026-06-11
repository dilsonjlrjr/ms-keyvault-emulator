package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
	"github.com/dilsonrabelo/kvemu/internal/vaultctx"
)

func VaultFromContext(ctx context.Context) *domain.Vault {
	return vaultctx.From(ctx)
}

// VaultFromPath resolve o vault a partir de um parâmetro de rota (ex.: {vault})
// e o injeta no context. Usado pelo plano /ui da kv-interface, onde o vault é
// explícito no path — independente de Host/DNS e fora da spec Azure data-plane.
func VaultFromPath(vaultRepo ports.VaultRepository, param string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := chi.URLParam(r, param)
			vault, err := vaultRepo.GetByName(r.Context(), name)
			if vault == nil || err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"error":{"code":"VaultNotFound","message":"vault not found: ` + name + `"}}`))
				return
			}
			ctx := vaultctx.With(r.Context(), vault)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
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

			ctx := vaultctx.With(r.Context(), vault)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
