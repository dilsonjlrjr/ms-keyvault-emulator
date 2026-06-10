package app

import (
	"context"
	"regexp"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type VaultService struct {
	repo       ports.VaultRepository
	baseDomain string
	port       string
}

func NewVaultService(repo ports.VaultRepository, baseDomain, port string) *VaultService {
	return &VaultService{repo: repo, baseDomain: baseDomain, port: port}
}

var vaultNameRE = regexp.MustCompile(domain.VaultNamePattern)

func (s *VaultService) Create(ctx context.Context, name, displayName, tenantID string) (*domain.Vault, error) {
	if !vaultNameRE.MatchString(name) {
		return nil, domain.ErrBadParam{Msg: "invalid vault name: use lowercase letters, numbers, and hyphens (1-24 chars)"}
	}

	if _, err := s.repo.GetByName(ctx, name); err == nil {
		return nil, domain.ErrVaultExists
	}

	if displayName == "" {
		displayName = name
	}

	host := domain.BuildVaultHost(name, s.baseDomain, s.port)
	now := time.Now().Unix()

	v := &domain.Vault{
		Name:        name,
		Host:        host,
		DisplayName: displayName,
		TenantID:    tenantID,
		Created:     now,
		Updated:     now,
	}

	if err := s.repo.Create(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *VaultService) List(ctx context.Context) ([]*domain.Vault, error) {
	return s.repo.List(ctx)
}

func (s *VaultService) GetByName(ctx context.Context, name string) (*domain.Vault, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *VaultService) GetByHost(ctx context.Context, host string) (*domain.Vault, error) {
	return s.repo.GetByHost(ctx, host)
}

func (s *VaultService) Delete(ctx context.Context, name string) error {
	if name == "vault" {
		return domain.ErrBadParam{Msg: "cannot delete the default vault"}
	}
	return s.repo.Delete(ctx, name)
}
