package sqlite_test

import (
	"context"
	"os"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

func newTestDB(t *testing.T) (*sqlite.SecretRepo, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "kvemu-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	db, err := sqlite.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	// migrations inline (não depende do diretório relativo)
	schema := `
	PRAGMA foreign_keys = ON;
	CREATE TABLE IF NOT EXISTS vault (
		id TEXT PRIMARY KEY, dns_name TEXT NOT NULL, tenant_id TEXT NOT NULL,
		soft_delete INTEGER NOT NULL DEFAULT 1, purge_protect INTEGER NOT NULL DEFAULT 0,
		retention_days INTEGER NOT NULL DEFAULT 90, created INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS secret (
		id INTEGER PRIMARY KEY, vault_id TEXT NOT NULL REFERENCES vault(id),
		name TEXT NOT NULL, current_ver TEXT, managed INTEGER NOT NULL DEFAULT 0,
		deleted_at INTEGER, scheduled_purge INTEGER, recovery_id TEXT,
		recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable', UNIQUE (vault_id, name)
	);
	CREATE TABLE IF NOT EXISTS secret_version (
		id INTEGER PRIMARY KEY, secret_id INTEGER NOT NULL REFERENCES secret(id) ON DELETE CASCADE,
		version TEXT NOT NULL, value_enc BLOB NOT NULL, value_nonce BLOB NOT NULL,
		content_type TEXT, tags_json TEXT, enabled INTEGER NOT NULL DEFAULT 1,
		nbf INTEGER, exp INTEGER, created INTEGER NOT NULL, updated INTEGER NOT NULL,
		UNIQUE (secret_id, version)
	);
	INSERT OR IGNORE INTO vault(id,dns_name,tenant_id,created) VALUES('test-vault','https://test-vault','tenant-1',0);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	repo := sqlite.NewSecretRepo(db, "test-master-key")
	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}
	return repo, cleanup
}

const vaultID = "test-vault"

func TestSecretRepo_UpsertAndGet(t *testing.T) {
	repo, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	sv := &domain.SecretVersion{
		Name:  "db-password",
		Value: "S3nh@Forte",
		Attributes: domain.Attributes{
			Enabled:       true,
			RecoveryLevel: domain.RecoveryLevelPurgeable,
		},
	}
	if err := repo.Upsert(ctx, vaultID, sv); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if sv.Version == "" {
		t.Fatal("version deve ser gerada no Upsert")
	}

	got, err := repo.GetCurrent(ctx, vaultID, "db-password")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if got.Value != "S3nh@Forte" {
		t.Errorf("value: got %q, want %q", got.Value, "S3nh@Forte")
	}
	if got.Version != sv.Version {
		t.Errorf("version mismatch: got %q, want %q", got.Version, sv.Version)
	}
}

func TestSecretRepo_MultipleVersions(t *testing.T) {
	repo, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	for _, val := range []string{"v1", "v2", "v3"} {
		sv := &domain.SecretVersion{
			Name:  "rotating",
			Value: val,
			Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
		}
		if err := repo.Upsert(ctx, vaultID, sv); err != nil {
			t.Fatalf("upsert %s: %v", val, err)
		}
	}

	// current deve ser v3
	got, err := repo.GetCurrent(ctx, vaultID, "rotating")
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "v3" {
		t.Errorf("current: got %q, want v3", got.Value)
	}

	// list versions deve ter 3
	list, _, err := repo.ListVersions(ctx, vaultID, "rotating", 25, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("versions: got %d, want 3", len(list))
	}
}

func TestSecretRepo_SoftDeleteAndRecover(t *testing.T) {
	repo, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	sv := &domain.SecretVersion{
		Name: "to-delete", Value: "x",
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, sv)

	ds, err := repo.SoftDelete(ctx, vaultID, "to-delete", 9999999999)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ds.DeletedDate == 0 {
		t.Fatal("deletedDate deve ser setado")
	}

	// não pode mais ser lido normalmente
	_, err = repo.GetCurrent(ctx, vaultID, "to-delete")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("esperado ErrNotFound após delete, got %v", err)
	}

	// pode ser lido como deletado
	got, err := repo.GetDeleted(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("get deleted: %v", err)
	}
	if got.Name != "to-delete" {
		t.Errorf("name: got %q", got.Name)
	}

	// recover
	rec, err := repo.Recover(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if rec.Value != "x" {
		t.Errorf("recovered value: got %q", rec.Value)
	}

	// leitura normal voltou a funcionar
	got2, err := repo.GetCurrent(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("get after recover: %v", err)
	}
	if got2.Value != "x" {
		t.Errorf("value after recover: %q", got2.Value)
	}
}

func TestSecretRepo_Purge(t *testing.T) {
	repo, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	sv := &domain.SecretVersion{
		Name: "to-purge", Value: "bye",
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, sv)
	repo.SoftDelete(ctx, vaultID, "to-purge", 9999999999)

	if err := repo.Purge(ctx, vaultID, "to-purge"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	_, err := repo.GetDeleted(ctx, vaultID, "to-purge")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("após purge deve ser ErrNotFound, got %v", err)
	}
}

func TestSecretRepo_NotFound(t *testing.T) {
	repo, cleanup := newTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := repo.GetCurrent(ctx, vaultID, "nao-existe")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("esperado ErrNotFound, got %T: %v", err, err)
	}
}

func TestSecretRepo_AtRestEncryption(t *testing.T) {
	f, _ := os.CreateTemp("", "kvemu-enc-*.db")
	f.Close()
	defer os.Remove(f.Name())

	db, _ := sqlite.Open(f.Name())
	defer db.Close()
	schema := `PRAGMA foreign_keys=ON;
	CREATE TABLE IF NOT EXISTS vault(id TEXT PRIMARY KEY,dns_name TEXT NOT NULL,tenant_id TEXT NOT NULL,soft_delete INTEGER NOT NULL DEFAULT 1,purge_protect INTEGER NOT NULL DEFAULT 0,retention_days INTEGER NOT NULL DEFAULT 90,created INTEGER NOT NULL);
	CREATE TABLE IF NOT EXISTS secret(id INTEGER PRIMARY KEY,vault_id TEXT NOT NULL REFERENCES vault(id),name TEXT NOT NULL,current_ver TEXT,managed INTEGER NOT NULL DEFAULT 0,deleted_at INTEGER,scheduled_purge INTEGER,recovery_id TEXT,recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',UNIQUE(vault_id,name));
	CREATE TABLE IF NOT EXISTS secret_version(id INTEGER PRIMARY KEY,secret_id INTEGER NOT NULL REFERENCES secret(id) ON DELETE CASCADE,version TEXT NOT NULL,value_enc BLOB NOT NULL,value_nonce BLOB NOT NULL,content_type TEXT,tags_json TEXT,enabled INTEGER NOT NULL DEFAULT 1,nbf INTEGER,exp INTEGER,created INTEGER NOT NULL,updated INTEGER NOT NULL,UNIQUE(secret_id,version));
	INSERT OR IGNORE INTO vault(id,dns_name,tenant_id,created) VALUES('v','https://v','t',0);`
	db.Exec(schema)

	repo := sqlite.NewSecretRepo(db, "my-master-key")
	ctx := context.Background()
	sv := &domain.SecretVersion{
		Name: "enc-test", Value: "SuperSecret",
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, "v", sv)

	// valor raw no banco NÃO deve ser "SuperSecret" em texto claro
	var rawEnc []byte
	db.QueryRowContext(ctx, `SELECT value_enc FROM secret_version WHERE version=?`, sv.Version).Scan(&rawEnc)
	if string(rawEnc) == "SuperSecret" {
		t.Fatal("valor não está cifrado em repouso")
	}

	// leitura via repo decifra corretamente
	got, _ := repo.GetCurrent(ctx, "v", "enc-test")
	if got.Value != "SuperSecret" {
		t.Errorf("decifração: got %q", got.Value)
	}
}
