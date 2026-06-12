package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type sharedMockVaultRepo struct {
	vaults map[string]*domain.Vault
}

func newSharedMockVaultRepo() *sharedMockVaultRepo {
	return &sharedMockVaultRepo{vaults: make(map[string]*domain.Vault)}
}

var _ ports.VaultRepository = (*sharedMockVaultRepo)(nil)

func (m *sharedMockVaultRepo) Create(ctx context.Context, v *domain.Vault) error {
	if _, ok := m.vaults[v.Name]; ok {
		return fmt.Errorf("%w: vault %q", domain.ErrVaultExists, v.Name)
	}
	copy := *v
	m.vaults[v.Name] = &copy
	return nil
}

func (m *sharedMockVaultRepo) List(ctx context.Context) ([]*domain.Vault, error) {
	result := make([]*domain.Vault, 0, len(m.vaults))
	for _, v := range m.vaults {
		copy := *v
		result = append(result, &copy)
	}
	return result, nil
}

func (m *sharedMockVaultRepo) GetByName(ctx context.Context, name string) (*domain.Vault, error) {
	v, ok := m.vaults[name]
	if !ok {
		return nil, domain.ErrVaultNotFound
	}
	copy := *v
	return &copy, nil
}

func (m *sharedMockVaultRepo) GetByHost(ctx context.Context, host string) (*domain.Vault, error) {
	for _, v := range m.vaults {
		if v.Host == host {
			copy := *v
			return &copy, nil
		}
	}
	return nil, domain.ErrVaultNotFound
}

func (m *sharedMockVaultRepo) Delete(ctx context.Context, name string) error { return nil }
func (m *sharedMockVaultRepo) Update(ctx context.Context, name string, v *domain.Vault) error {
	return nil
}

func TestVaultResolver_VaultName(t *testing.T) {
	repo := newSharedMockVaultRepo()
	repo.Create(context.Background(), &domain.Vault{Name: "prod", Host: "prod.kvemu.local:13000", DisplayName: "Prod", TenantID: "t1"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := middleware.VaultFromContext(r.Context())
		if v == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(v.Name))
	})

	h := middleware.VaultResolver(repo, "kvemu.local", "vault")(handler)

	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Host = "prod.kvemu.local:13000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "prod" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "prod")
	}
}

func TestVaultResolver_LocalhostFallback(t *testing.T) {
	repo := newSharedMockVaultRepo()
	repo.Create(context.Background(), &domain.Vault{Name: "vault", Host: "vault.kvemu.local:13000", DisplayName: "Default", TenantID: "t1"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := middleware.VaultFromContext(r.Context())
		if v == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(v.Name))
	})

	h := middleware.VaultResolver(repo, "kvemu.local", "vault")(handler)

	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Host = "localhost:13000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "vault" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "vault")
	}
}

func TestVaultResolver_UnknownVault_404(t *testing.T) {
	repo := newSharedMockVaultRepo()
	repo.Create(context.Background(), &domain.Vault{Name: "vault", Host: "vault.kvemu.local:13000", DisplayName: "Default", TenantID: "t1"})

	h := middleware.VaultResolver(repo, "kvemu.local", "vault")(http.NotFoundHandler())

	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Host = "some-vault.kvemu.local:13000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// Plano /ui: o vault vem do path, não do Host. Vaults distintos devem resolver
// isolados — é o que garante que a UI veja os secrets do vault selecionado.
func TestVaultFromPath_ResolvesByPath(t *testing.T) {
	repo := newSharedMockVaultRepo()
	repo.Create(context.Background(), &domain.Vault{Name: "comum", Host: "comum.kvemu.local:13000"})
	repo.Create(context.Background(), &domain.Vault{Name: "configs", Host: "configs.kvemu.local:13000"})

	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(middleware.VaultFromContext(r.Context()).Name))
	})

	r := chi.NewRouter()
	r.Route("/ui/vaults/{vault}", func(r chi.Router) {
		r.Use(middleware.VaultFromPath(repo, "vault"))
		r.Get("/secrets", echo)
	})

	for _, want := range []string{"comum", "configs"} {
		req := httptest.NewRequest("GET", "/ui/vaults/"+want+"/secrets", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("vault %q: status = %d, want 200", want, rec.Code)
		}
		if rec.Body.String() != want {
			t.Errorf("vault do path ignorado: got %q, want %q", rec.Body.String(), want)
		}
	}
}

func TestVaultFromPath_UnknownVault_404(t *testing.T) {
	repo := newSharedMockVaultRepo()
	repo.Create(context.Background(), &domain.Vault{Name: "comum", Host: "comum.kvemu.local:13000"})

	r := chi.NewRouter()
	r.Route("/ui/vaults/{vault}", func(r chi.Router) {
		r.Use(middleware.VaultFromPath(repo, "vault"))
		r.Get("/secrets", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	})

	req := httptest.NewRequest("GET", "/ui/vaults/missing/secrets", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestVaultResolver_BaseDomainFallback(t *testing.T) {
	repo := newSharedMockVaultRepo()
	repo.Create(context.Background(), &domain.Vault{Name: "vault", Host: "vault.kvemu.local:13000", DisplayName: "Default", TenantID: "t1"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := middleware.VaultFromContext(r.Context())
		if v == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(v.Name))
	})

	h := middleware.VaultResolver(repo, "kvemu.local", "vault")(handler)

	req := httptest.NewRequest("GET", "/secrets", nil)
	req.Host = "kvemu.local:13000"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "vault" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "vault")
	}
}
