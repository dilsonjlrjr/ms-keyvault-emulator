package app

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type mockVaultRepo struct {
	mu     sync.RWMutex
	vaults map[string]*domain.Vault
}

func newMockVaultRepo() *mockVaultRepo {
	return &mockVaultRepo{vaults: make(map[string]*domain.Vault)}
}

var _ ports.VaultRepository = (*mockVaultRepo)(nil)

func (m *mockVaultRepo) Create(ctx context.Context, v *domain.Vault) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vaults[v.Name]; ok {
		return fmt.Errorf("%w: vault %q", domain.ErrVaultExists, v.Name)
	}
	copy := *v
	m.vaults[v.Name] = &copy
	return nil
}

func (m *mockVaultRepo) List(ctx context.Context) ([]*domain.Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.vaults))
	for n := range m.vaults {
		names = append(names, n)
	}
	sort.Strings(names)
	result := make([]*domain.Vault, len(names))
	for i, n := range names {
		copy := *m.vaults[n]
		result[i] = &copy
	}
	return result, nil
}

func (m *mockVaultRepo) GetByName(ctx context.Context, name string) (*domain.Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.vaults[name]
	if !ok {
		return nil, fmt.Errorf("%w: vault %q", domain.ErrVaultNotFound, name)
	}
	copy := *v
	return &copy, nil
}

func (m *mockVaultRepo) GetByHost(ctx context.Context, host string) (*domain.Vault, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range m.vaults {
		if v.Host == host {
			copy := *v
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("%w: vault host %q", domain.ErrVaultNotFound, host)
}

func (m *mockVaultRepo) Delete(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vaults[name]; !ok {
		return fmt.Errorf("%w: vault %q", domain.ErrVaultNotFound, name)
	}
	delete(m.vaults, name)
	return nil
}

func (m *mockVaultRepo) Update(ctx context.Context, name string, v *domain.Vault) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.vaults[name]
	if !ok {
		return fmt.Errorf("%w: vault %q", domain.ErrVaultNotFound, name)
	}
	existing.DisplayName = v.DisplayName
	existing.Updated = time.Now().Unix()
	return nil
}
