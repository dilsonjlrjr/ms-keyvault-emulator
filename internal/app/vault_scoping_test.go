package app

import (
	"context"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
	"github.com/dilsonrabelo/kvemu/internal/vaultctx"
)

// captureSecretRepo registra o vaultID recebido em List, sem tocar no banco.
// Os demais métodos da interface não são exercidos neste teste.
type captureSecretRepo struct {
	ports.SecretRepository
	gotVaultID string
}

func (r *captureSecretRepo) List(_ context.Context, vaultID string, _ int, _ string) ([]*domain.SecretVersion, string, error) {
	r.gotVaultID = vaultID
	return nil, "", nil
}

// Regressão: antes do fix multi-vault o service usava sempre o vaultID fixo do
// boot, ignorando o vault resolvido por request (VaultResolver). Isso fazia a UI
// exibir os secrets do vault default independente do vault selecionado.
func TestSecretService_ScopesByRequestVault(t *testing.T) {
	repo := &captureSecretRepo{}
	svc := NewSecretService(repo, "default-vault")

	// Sem vault no context (ex.: chamada interna) → fallback no vault do boot.
	if _, _, err := svc.List(context.Background(), 10, ""); err != nil {
		t.Fatalf("List sem context: %v", err)
	}
	if repo.gotVaultID != "default-vault" {
		t.Fatalf("fallback errado: got %q, want %q", repo.gotVaultID, "default-vault")
	}

	// Com vault no context (request a outro vault) → usa o vault da request.
	ctx := vaultctx.With(context.Background(), &domain.Vault{Name: "other-vault"})
	if _, _, err := svc.List(ctx, 10, ""); err != nil {
		t.Fatalf("List com context: %v", err)
	}
	if repo.gotVaultID != "other-vault" {
		t.Fatalf("vault do context ignorado: got %q, want %q", repo.gotVaultID, "other-vault")
	}
}
