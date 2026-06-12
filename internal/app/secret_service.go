package app

import (
	"context"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
	"github.com/dilsonrabelo/kvemu/internal/vaultctx"
)

const defaultRetentionDays = 90

type SecretService struct {
	repo    ports.SecretRepository
	vaultID string
}

func NewSecretService(repo ports.SecretRepository, vaultID string) *SecretService {
	return &SecretService{repo: repo, vaultID: vaultID}
}

// vid resolve o vault da request (injetado pelo VaultResolver via context),
// caindo no vault default do boot quando o context não traz vault (ex.: chamadas
// internas como import/export que constroem o service com vaultID explícito).
func (s *SecretService) vid(ctx context.Context) string {
	if v := vaultctx.NameFrom(ctx); v != "" {
		return v
	}
	return s.vaultID
}

// Set cria ou atualiza um secret (nova versão sempre).
func (s *SecretService) Set(ctx context.Context, name, value, contentType string,
	tags map[string]string, attrs domain.Attributes) (*domain.SecretVersion, error) {

	sv := &domain.SecretVersion{
		Name:        name,
		Value:       value,
		ContentType: contentType,
		Tags:        tags,
		Attributes:  attrs,
	}
	if err := s.repo.Upsert(ctx, s.vid(ctx), sv); err != nil {
		return nil, err
	}
	return sv, nil
}

// Get retorna a versão corrente (version="") ou uma versão específica.
func (s *SecretService) Get(ctx context.Context, name, version string) (*domain.SecretVersion, error) {
	if version == "" {
		return s.repo.GetCurrent(ctx, s.vid(ctx), name)
	}
	return s.repo.Get(ctx, s.vid(ctx), name, version)
}

// List lista secrets (sem value — só metadados).
func (s *SecretService) List(ctx context.Context, max int, skipToken string) ([]*domain.SecretVersion, string, error) {
	list, next, err := s.repo.List(ctx, s.vid(ctx), max, skipToken)
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
	list, next, err := s.repo.ListVersions(ctx, s.vid(ctx), name, max, skipToken)
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

	if err := s.repo.UpdateAttributes(ctx, s.vid(ctx), name, version, attrs, contentType, tags); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, s.vid(ctx), name, version)
}

// Delete faz soft-delete.
func (s *SecretService) Delete(ctx context.Context, name string) (*domain.DeletedSecret, error) {
	schedPurge := time.Now().AddDate(0, 0, defaultRetentionDays).Unix()
	return s.repo.SoftDelete(ctx, s.vid(ctx), name, schedPurge)
}

// GetDeleted retorna um secret deletado.
func (s *SecretService) GetDeleted(ctx context.Context, name string) (*domain.DeletedSecret, error) {
	return s.repo.GetDeleted(ctx, s.vid(ctx), name)
}

// ListDeleted lista secrets deletados.
func (s *SecretService) ListDeleted(ctx context.Context, max int, skipToken string) ([]*domain.DeletedSecret, string, error) {
	return s.repo.ListDeleted(ctx, s.vid(ctx), max, skipToken)
}

// Recover recupera um secret deletado.
func (s *SecretService) Recover(ctx context.Context, name string) (*domain.SecretVersion, error) {
	return s.repo.Recover(ctx, s.vid(ctx), name)
}

// Purge remove permanentemente um secret deletado.
func (s *SecretService) Purge(ctx context.Context, name string) error {
	return s.repo.Purge(ctx, s.vid(ctx), name)
}
