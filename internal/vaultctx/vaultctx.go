// Package vaultctx carrega o vault resolvido por request no context.Context.
// Vive numa camada neutra para que tanto os adapters HTTP (que resolvem o vault
// a partir do Host) quanto os services de aplicação (que filtram dados por vault)
// usem a mesma chave, sem criar dependência do core nos adapters.
package vaultctx

import (
	"context"

	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type ctxKey int

const vaultKey ctxKey = 1

// With retorna um context derivado com o vault resolvido anexado.
func With(ctx context.Context, v *domain.Vault) context.Context {
	return context.WithValue(ctx, vaultKey, v)
}

// From retorna o vault anexado ao context, ou nil quando ausente.
func From(ctx context.Context) *domain.Vault {
	v, _ := ctx.Value(vaultKey).(*domain.Vault)
	return v
}

// NameFrom retorna o nome do vault do context, ou "" quando ausente.
func NameFrom(ctx context.Context) string {
	if v := From(ctx); v != nil {
		return v.Name
	}
	return ""
}
