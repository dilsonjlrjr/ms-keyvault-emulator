package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type fakeVaultRepo struct {
	vault *domain.Vault
}

func (f *fakeVaultRepo) Create(ctx context.Context, v *domain.Vault) error { return nil }
func (f *fakeVaultRepo) List(ctx context.Context) ([]*domain.Vault, error) {
	return []*domain.Vault{f.vault}, nil
}
func (f *fakeVaultRepo) GetByName(ctx context.Context, name string) (*domain.Vault, error) {
	return f.vault, nil
}
func (f *fakeVaultRepo) GetByHost(ctx context.Context, host string) (*domain.Vault, error) {
	return f.vault, nil
}
func (f *fakeVaultRepo) Delete(ctx context.Context, name string) error { return nil }
func (f *fakeVaultRepo) Update(ctx context.Context, name string, v *domain.Vault) error {
	return nil
}

func testRouter() http.Handler {
	repo := &fakeVaultRepo{vault: &domain.Vault{
		Name:     "gdr",
		Host:     "gdr.kvemu.local:13000",
		TenantID: "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f",
	}}
	return NewRouter(RouterConfig{
		VaultRepo:    repo,
		BaseDomain:   "kvemu.local",
		DefaultVault: "gdr",
	})
}

// TestRouter_TrailingSlashOnSecretRoute — o SDK Azure monta
// GET /secrets/{name}/{version} com version vazia (barra final) quando quer a
// versão mais recente. A rota tem que casar (challenge 401 sem token), nunca 404.
func TestRouter_TrailingSlashOnSecretRoute(t *testing.T) {
	router := testRouter()

	paths := []string{
		"/secrets/ENVIRONMENT?api-version=7.6",
		"/secrets/ENVIRONMENT/?api-version=7.6",
		"/keys/mykey/?api-version=7.6",
		"/certificates/mycert/?api-version=7.6",
	}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		req.Host = "gdr.kvemu.local:13000"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code == http.StatusNotFound {
			t.Errorf("%s: rota não casou (404) — barra final tem que ser aceita", p)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: esperado 401 (challenge sem token), veio %d", p, rec.Code)
		}
	}
}

// TestRouter_ChallengeResourceHasPort — o 401 do data-plane devolve resource=
// com domínio pai + porta do vault (validação de challenge dos SDKs).
func TestRouter_ChallengeResourceHasPort(t *testing.T) {
	router := testRouter()

	req := httptest.NewRequest(http.MethodGet, "/secrets/ENVIRONMENT/?api-version=7.6", nil)
	req.Host = "gdr.kvemu.local:13000"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	got := rec.Header().Get("WWW-Authenticate")
	want := `Bearer authorization="https://gdr.kvemu.local:13000/a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f", resource="https://kvemu.local:13000"`
	if got != want {
		t.Fatalf("challenge divergiu:\n got:  %s\n want: %s", got, want)
	}
}
