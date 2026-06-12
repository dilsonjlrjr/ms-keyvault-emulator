package http

import (
	"net/http"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
)

func vaultHostFromContext(r *http.Request, fallback string) string {
	if vault := middleware.VaultFromContext(r.Context()); vault != nil {
		return vault.Host
	}
	return fallback
}
