package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type KeyService struct {
	repo    ports.KeyRepository
	vaultID string
	// Repo é exposto para uso interno do CertService (geração de chave de suporte).
	Repo ports.KeyRepository
}

func NewKeyService(repo ports.KeyRepository, vaultID string) *KeyService {
	return &KeyService{repo: repo, Repo: repo, vaultID: vaultID}
}

func (s *KeyService) Create(ctx context.Context, name, kty, crv string, keySize int,
	keyOps []string, attrs domain.Attributes) (*domain.KeyVersion, error) {

	pubJWK, privJWK, err := kvCrypto.GenerateKeyPair(kty, crv, keySize)
	if err != nil {
		return nil, domain.ErrBadParam{Msg: fmt.Sprintf("key generation failed: %v", err)}
	}
	return s.store(ctx, name, kty, crv, keySize, keyOps, attrs, pubJWK, privJWK)
}

func (s *KeyService) Import(ctx context.Context, name string, jwk map[string]any,
	keyOps []string, attrs domain.Attributes) (*domain.KeyVersion, error) {

	kty, _ := jwk["kty"].(string)
	crv, _ := jwk["crv"].(string)
	pubJWK, privJWK, err := kvCrypto.ImportJWK(jwk)
	if err != nil {
		return nil, domain.ErrBadParam{Msg: fmt.Sprintf("invalid JWK: %v", err)}
	}
	return s.store(ctx, name, kty, crv, 0, keyOps, attrs, pubJWK, privJWK)
}

func (s *KeyService) store(ctx context.Context, name, kty, crv string, keySize int,
	keyOps []string, attrs domain.Attributes, pubJWK, privJWK map[string]any) (*domain.KeyVersion, error) {

	kv := &domain.KeyVersion{
		Name: name, Kty: kty, Crv: crv, KeySize: keySize,
		KeyOps: keyOps, PubJWK: pubJWK, Attributes: attrs,
	}
	if kv.Attributes.RecoveryLevel == "" {
		kv.Attributes.RecoveryLevel = domain.RecoveryLevelPurgeable
	}
	if err := s.repo.Upsert(ctx, s.vaultID, kv, privJWK); err != nil {
		return nil, err
	}
	kv.PubJWK["kid"] = domain.ID(s.vaultID, "keys", name, kv.Version)
	return kv, nil
}

func (s *KeyService) Get(ctx context.Context, name, version string) (*domain.KeyVersion, error) {
	var (
		kv  *domain.KeyVersion
		err error
	)
	if version == "" {
		kv, err = s.repo.GetCurrent(ctx, s.vaultID, name)
	} else {
		kv, err = s.repo.Get(ctx, s.vaultID, name, version)
	}
	if err != nil {
		return nil, err
	}
	kv.PubJWK["kid"] = domain.ID(s.vaultID, "keys", kv.Name, kv.Version)
	return kv, nil
}

func (s *KeyService) List(ctx context.Context, max int, skipToken string) ([]*domain.KeyVersion, string, error) {
	return s.repo.List(ctx, s.vaultID, max, skipToken)
}

func (s *KeyService) ListVersions(ctx context.Context, name string, max int, skipToken string) ([]*domain.KeyVersion, string, error) {
	return s.repo.ListVersions(ctx, s.vaultID, name, max, skipToken)
}

func (s *KeyService) Update(ctx context.Context, name, version string,
	attrs domain.Attributes, keyOps []string, tags map[string]string) (*domain.KeyVersion, error) {

	if err := s.repo.UpdateAttributes(ctx, s.vaultID, name, version, attrs, keyOps, tags); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, s.vaultID, name, version)
}

func (s *KeyService) Delete(ctx context.Context, name string) (*domain.DeletedKey, error) {
	schedPurge := time.Now().AddDate(0, 0, defaultRetentionDays).Unix()
	return s.repo.SoftDelete(ctx, s.vaultID, name, schedPurge)
}

func (s *KeyService) GetDeleted(ctx context.Context, name string) (*domain.DeletedKey, error) {
	return s.repo.GetDeleted(ctx, s.vaultID, name)
}

func (s *KeyService) ListDeleted(ctx context.Context, max int, skipToken string) ([]*domain.DeletedKey, string, error) {
	return s.repo.ListDeleted(ctx, s.vaultID, max, skipToken)
}

func (s *KeyService) Recover(ctx context.Context, name string) (*domain.KeyVersion, error) {
	return s.repo.Recover(ctx, s.vaultID, name)
}

func (s *KeyService) Purge(ctx context.Context, name string) error {
	return s.repo.Purge(ctx, s.vaultID, name)
}

// ─── operações criptográficas ─────────────────────────────────────────────────

func (s *KeyService) Encrypt(ctx context.Context, name, version, alg string, value []byte) ([]byte, string, error) {
	kv, err := s.Get(ctx, name, version)
	if err != nil {
		return nil, "", err
	}
	if !kv.Attributes.Enabled {
		return nil, "", domain.ErrForbidden{Msg: "key is disabled"}
	}
	ct, err := kvCrypto.EncryptData(kv.PubJWK, alg, value)
	if err != nil {
		return nil, "", domain.ErrBadParam{Msg: err.Error()}
	}
	return ct, kv.Version, nil
}

func (s *KeyService) Decrypt(ctx context.Context, name, version, alg string, ciphertext []byte) ([]byte, string, error) {
	kv, err := s.Get(ctx, name, version)
	if err != nil {
		return nil, "", err
	}
	if !kv.Attributes.Enabled {
		return nil, "", domain.ErrForbidden{Msg: "key is disabled"}
	}
	priv, err := s.repo.GetPriv(ctx, s.vaultID, kv.Name, kv.Version)
	if err != nil {
		return nil, "", err
	}
	merged := mergeMaps(kv.PubJWK, priv)
	plain, err := kvCrypto.DecryptData(merged, alg, ciphertext)
	if err != nil {
		return nil, "", domain.ErrBadParam{Msg: err.Error()}
	}
	return plain, kv.Version, nil
}

func (s *KeyService) Sign(ctx context.Context, name, version, alg string, data []byte) ([]byte, string, error) {
	kv, err := s.Get(ctx, name, version)
	if err != nil {
		return nil, "", err
	}
	if !kv.Attributes.Enabled {
		return nil, "", domain.ErrForbidden{Msg: "key is disabled"}
	}
	priv, err := s.repo.GetPriv(ctx, s.vaultID, kv.Name, kv.Version)
	if err != nil {
		return nil, "", err
	}
	merged := mergeMaps(kv.PubJWK, priv)
	sig, err := kvCrypto.SignData(merged, alg, data)
	if err != nil {
		return nil, "", domain.ErrBadParam{Msg: err.Error()}
	}
	return sig, kv.Version, nil
}

func (s *KeyService) Verify(ctx context.Context, name, version, alg string, data, sig []byte) (bool, string, error) {
	kv, err := s.Get(ctx, name, version)
	if err != nil {
		return false, "", err
	}
	ok, err := kvCrypto.VerifyData(kv.PubJWK, alg, data, sig)
	if err != nil {
		return false, "", domain.ErrBadParam{Msg: err.Error()}
	}
	return ok, kv.Version, nil
}

func (s *KeyService) WrapKey(ctx context.Context, name, version, alg string, keyMaterial []byte) ([]byte, string, error) {
	kv, err := s.Get(ctx, name, version)
	if err != nil {
		return nil, "", err
	}
	ct, err := kvCrypto.WrapKey(kv.PubJWK, alg, keyMaterial)
	if err != nil {
		return nil, "", domain.ErrBadParam{Msg: err.Error()}
	}
	return ct, kv.Version, nil
}

func (s *KeyService) UnwrapKey(ctx context.Context, name, version, alg string, wrapped []byte) ([]byte, string, error) {
	kv, err := s.Get(ctx, name, version)
	if err != nil {
		return nil, "", err
	}
	priv, err := s.repo.GetPriv(ctx, s.vaultID, kv.Name, kv.Version)
	if err != nil {
		return nil, "", err
	}
	merged := mergeMaps(kv.PubJWK, priv)
	plain, err := kvCrypto.UnwrapKey(merged, alg, wrapped)
	if err != nil {
		return nil, "", domain.ErrBadParam{Msg: err.Error()}
	}
	return plain, kv.Version, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func mergeMaps(a, b map[string]any) map[string]any {
	out := make(map[string]any, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
