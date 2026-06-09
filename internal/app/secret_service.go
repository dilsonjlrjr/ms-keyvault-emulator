package app

import (
	"context"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

const defaultRetentionDays = 90

type SecretService struct {
	repo    ports.SecretRepository
	vaultID string
}

func NewSecretService(repo ports.SecretRepository, vaultID string) *SecretService {
	return &SecretService{repo: repo, vaultID: vaultID}
}

// Set cria ou atualiza um secret (nova versão sempre).
func (s *SecretService) Set(ctx context.Context, name, value, contentType string,
	tags map[string]string, attrs domain.Attributes) (*domain.SecretVersion, error) {

	deleted, err := s.repo.IsDeleted(ctx, s.vaultID, name)
	if err != nil {
		return nil, err
	}
	if deleted {
		return nil, domain.ErrDeleted{Kind: "Secret", Name: name}
	}

	sv := &domain.SecretVersion{
		Name:        name,
		Value:       value,
		ContentType: contentType,
		Tags:        tags,
		Attributes:  attrs,
	}
	if err := s.repo.Upsert(ctx, s.vaultID, sv); err != nil {
		return nil, err
	}
	return sv, nil
}

// Get retorna a versão corrente (version="") ou uma versão específica.
func (s *SecretService) Get(ctx context.Context, name, version string) (*domain.SecretVersion, error) {
	if version == "" {
		return s.repo.GetCurrent(ctx, s.vaultID, name)
	}
	return s.repo.Get(ctx, s.vaultID, name, version)
}

// List lista secrets (sem value — só metadados).
func (s *SecretService) List(ctx context.Context, max int, skipToken string) ([]*domain.SecretVersion, string, error) {
	list, next, err := s.repo.List(ctx, s.vaultID, max, skipToken)
	if err != nil {
		return nil, "", err
	}
	// lista não expõe value
	for _, sv := range list {
		sv.Value = ""
	}
	return list, next, nil
}

// ListVersions lista versões de um secret (sem value).
func (s *SecretService) ListVersions(ctx context.Context, name string, max int, skipToken string) ([]*domain.SecretVersion, string, error) {
	list, next, err := s.repo.ListVersions(ctx, s.vaultID, name, max, skipToken)
	if err != nil {
		return nil, "", err
	}
	for _, sv := range list {
		sv.Value = ""
	}
	return list, next, nil
}

// Update atualiza atributos de uma versão específica (PATCH).
func (s *SecretService) Update(ctx context.Context, name, version string,
	attrs domain.Attributes, contentType *string, tags map[string]string) (*domain.SecretVersion, error) {

	if err := s.repo.UpdateAttributes(ctx, s.vaultID, name, version, attrs, contentType, tags); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, s.vaultID, name, version)
}

// Delete faz soft-delete.
func (s *SecretService) Delete(ctx context.Context, name string) (*domain.DeletedSecret, error) {
	schedPurge := time.Now().AddDate(0, 0, defaultRetentionDays).Unix()
	return s.repo.SoftDelete(ctx, s.vaultID, name, schedPurge)
}

// GetDeleted retorna um secret deletado.
func (s *SecretService) GetDeleted(ctx context.Context, name string) (*domain.DeletedSecret, error) {
	return s.repo.GetDeleted(ctx, s.vaultID, name)
}

// ListDeleted lista secrets deletados.
func (s *SecretService) ListDeleted(ctx context.Context, max int, skipToken string) ([]*domain.DeletedSecret, string, error) {
	return s.repo.ListDeleted(ctx, s.vaultID, max, skipToken)
}

// Recover recupera um secret deletado.
func (s *SecretService) Recover(ctx context.Context, name string) (*domain.SecretVersion, error) {
	return s.repo.Recover(ctx, s.vaultID, name)
}

// Purge remove permanentemente um secret deletado.
func (s *SecretService) Purge(ctx context.Context, name string) error {
	return s.repo.Purge(ctx, s.vaultID, name)
}
