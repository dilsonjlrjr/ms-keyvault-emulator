package ports

import (
	"context"

	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type SecretRepository interface {
	Upsert(ctx context.Context, vaultID string, s *domain.SecretVersion) error
	Get(ctx context.Context, vaultID, name, version string) (*domain.SecretVersion, error)
	GetCurrent(ctx context.Context, vaultID, name string) (*domain.SecretVersion, error)
	List(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.SecretVersion, string, error)
	ListVersions(ctx context.Context, vaultID, name string, max int, skipToken string) ([]*domain.SecretVersion, string, error)
	UpdateAttributes(ctx context.Context, vaultID, name, version string, attrs domain.Attributes, contentType *string, tags map[string]string) error
	SoftDelete(ctx context.Context, vaultID, name string, schedPurge int64) (*domain.DeletedSecret, error)
	GetDeleted(ctx context.Context, vaultID, name string) (*domain.DeletedSecret, error)
	ListDeleted(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.DeletedSecret, string, error)
	Recover(ctx context.Context, vaultID, name string) (*domain.SecretVersion, error)
	Purge(ctx context.Context, vaultID, name string) error
	IsDeleted(ctx context.Context, vaultID, name string) (bool, error)
}
