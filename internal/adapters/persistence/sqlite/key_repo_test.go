package sqlite_test

import (
	"context"
	"os"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

func newTestKeyDB(t *testing.T) (*sqlite.KeyRepo, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "kvemu-key-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	db, err := sqlite.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	schema := `
	PRAGMA foreign_keys = ON;
	CREATE TABLE IF NOT EXISTS vault (
		id TEXT PRIMARY KEY, dns_name TEXT NOT NULL, tenant_id TEXT NOT NULL,
		soft_delete INTEGER NOT NULL DEFAULT 1, purge_protect INTEGER NOT NULL DEFAULT 0,
		retention_days INTEGER NOT NULL DEFAULT 90, created INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS vkey (
		id INTEGER PRIMARY KEY, vault_id TEXT NOT NULL REFERENCES vault(id),
		name TEXT NOT NULL, current_ver TEXT,
		deleted_at INTEGER, scheduled_purge INTEGER, recovery_id TEXT,
		recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable', UNIQUE (vault_id, name)
	);
	CREATE TABLE IF NOT EXISTS vkey_version (
		id INTEGER PRIMARY KEY, key_id INTEGER NOT NULL REFERENCES vkey(id) ON DELETE CASCADE,
		version TEXT NOT NULL, kty TEXT NOT NULL, crv TEXT, key_size INTEGER,
		key_ops_json TEXT, jwk_pub_json TEXT, jwk_priv_enc BLOB, jwk_priv_nonce BLOB,
		enabled INTEGER NOT NULL DEFAULT 1, nbf INTEGER, exp INTEGER,
		created INTEGER NOT NULL, updated INTEGER NOT NULL,
		UNIQUE (key_id, version)
	);
	INSERT OR IGNORE INTO vault(id,dns_name,tenant_id,created) VALUES('test-vault','https://test-vault','tenant-1',0);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	repo := sqlite.NewKeyRepo(db, "test-master-key")
	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}
	return repo, cleanup
}

func TestKeyRepo_UpsertAndGet(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	kv := &domain.KeyVersion{
		Name:    "test-key",
		Kty:     "RSA",
		KeySize: 2048,
		KeyOps:  []string{"encrypt", "decrypt"},
		PubJWK:  map[string]any{"kty": "RSA", "n": "abc123", "e": "AQAB"},
		Attributes: domain.Attributes{
			Enabled:       true,
			RecoveryLevel: domain.RecoveryLevelPurgeable,
		},
	}
	if err := repo.Upsert(ctx, vaultID, kv, map[string]any{"d": "secret"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if kv.Version == "" {
		t.Fatal("version deve ser gerada no Upsert")
	}

	got, err := repo.GetCurrent(ctx, vaultID, "test-key")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if got.Kty != "RSA" {
		t.Errorf("kty: got %q, want %q", got.Kty, "RSA")
	}
	if got.KeySize != 2048 {
		t.Errorf("key_size: got %d, want 2048", got.KeySize)
	}
	if got.Version != kv.Version {
		t.Errorf("version mismatch: got %q, want %q", got.Version, kv.Version)
	}
}

func TestKeyRepo_MultipleVersions(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	for i, size := range []int{2048, 3072, 4096} {
		kv := &domain.KeyVersion{
			Name:    "rotating-key",
			Kty:     "RSA",
			KeySize: size,
			KeyOps:  []string{"encrypt", "decrypt"},
			PubJWK:  map[string]any{"kty": "RSA"},
			Attributes: domain.Attributes{
				Enabled:       true,
				RecoveryLevel: domain.RecoveryLevelPurgeable,
			},
		}
		if err := repo.Upsert(ctx, vaultID, kv, map[string]any{"d": "secret"}); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	got, err := repo.GetCurrent(ctx, vaultID, "rotating-key")
	if err != nil {
		t.Fatal(err)
	}
	if got.KeySize != 4096 {
		t.Errorf("current key_size: got %d, want 4096", got.KeySize)
	}

	list, _, err := repo.ListVersions(ctx, vaultID, "rotating-key", 25, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("versions: got %d, want 3", len(list))
	}
}

func TestKeyRepo_SoftDeleteAndRecover(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	kv := &domain.KeyVersion{
		Name: "to-delete", Kty: "RSA", KeySize: 2048,
		KeyOps: []string{"encrypt"}, PubJWK: map[string]any{"kty": "RSA"},
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, kv, nil)

	dk, err := repo.SoftDelete(ctx, vaultID, "to-delete", 9999999999)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if dk.DeletedDate == 0 {
		t.Fatal("deletedDate deve ser setado")
	}

	_, err = repo.GetCurrent(ctx, vaultID, "to-delete")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("esperado ErrNotFound após delete, got %v", err)
	}

	got, err := repo.GetDeleted(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("get deleted: %v", err)
	}
	if got.Name != "to-delete" {
		t.Errorf("name: got %q", got.Name)
	}

	rec, err := repo.Recover(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if rec.Kty != "RSA" {
		t.Errorf("recovered kty: got %q", rec.Kty)
	}

	got2, err := repo.GetCurrent(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("get after recover: %v", err)
	}
	if got2.Kty != "RSA" {
		t.Errorf("kty after recover: %q", got2.Kty)
	}
}

func TestKeyRepo_Purge(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	kv := &domain.KeyVersion{
		Name: "to-purge", Kty: "RSA", KeySize: 2048,
		KeyOps: []string{"encrypt"}, PubJWK: map[string]any{"kty": "RSA"},
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, kv, nil)
	repo.SoftDelete(ctx, vaultID, "to-purge", 9999999999)

	if err := repo.Purge(ctx, vaultID, "to-purge"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	_, err := repo.GetDeleted(ctx, vaultID, "to-purge")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("após purge deve ser ErrNotFound, got %v", err)
	}
}

func TestKeyRepo_NotFound(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := repo.GetCurrent(ctx, vaultID, "nao-existe")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("esperado ErrNotFound, got %T: %v", err, err)
	}
}

func TestKeyRepo_List(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	for _, name := range []string{"key-a", "key-b", "key-c"} {
		kv := &domain.KeyVersion{
			Name: name, Kty: "RSA", KeySize: 2048,
			KeyOps: []string{"encrypt"}, PubJWK: map[string]any{"kty": "RSA"},
			Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
		}
		repo.Upsert(ctx, vaultID, kv, nil)
	}

	list, _, err := repo.List(ctx, vaultID, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("list: got %d, want 3", len(list))
	}
}

func TestKeyRepo_GetPriv(t *testing.T) {
	repo, cleanup := newTestKeyDB(t)
	defer cleanup()
	ctx := context.Background()

	privJWK := map[string]any{"d": "secret-d", "p": "secret-p"}
	kv := &domain.KeyVersion{
		Name: "priv-key", Kty: "RSA", KeySize: 2048,
		KeyOps: []string{"encrypt", "decrypt"}, PubJWK: map[string]any{"kty": "RSA"},
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, kv, privJWK)

	got, err := repo.GetPriv(ctx, vaultID, "priv-key", kv.Version)
	if err != nil {
		t.Fatalf("get priv: %v", err)
	}
	if got["d"] != "secret-d" {
		t.Errorf("priv d: got %v", got["d"])
	}
}
