package sqlite_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

func newVaultTestDB(t *testing.T) (*sqlite.VaultRepo, func()) {
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

	schema := `
	PRAGMA foreign_keys = ON;
	CREATE TABLE IF NOT EXISTS vault (
		name         TEXT NOT NULL PRIMARY KEY,
		host         TEXT NOT NULL UNIQUE,
		dns_name     TEXT NOT NULL,
		tenant_id    TEXT NOT NULL,
		display_name TEXT NOT NULL DEFAULT '',
		created      INTEGER NOT NULL,
		updated      INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS secret (
		id INTEGER PRIMARY KEY, vault_id TEXT NOT NULL REFERENCES vault(name),
		name TEXT NOT NULL, current_ver TEXT, managed INTEGER NOT NULL DEFAULT 0,
		deleted_at INTEGER, scheduled_purge INTEGER, recovery_id TEXT,
		recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable', UNIQUE (vault_id, name)
	);
	CREATE TABLE IF NOT EXISTS vkey (
		id INTEGER PRIMARY KEY, vault_id TEXT NOT NULL REFERENCES vault(name),
		name TEXT NOT NULL, current_ver TEXT, deleted_at INTEGER,
		scheduled_purge INTEGER, recovery_id TEXT,
		recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable', UNIQUE (vault_id, name)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}
	return sqlite.NewVaultRepo(db), cleanup
}

func TestVaultRepo_Create(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	v := &domain.Vault{
		Name:        "test-vault",
		Host:        "test-vault.kvemu.local:13000",
		DisplayName: "Test Vault",
		TenantID:    "tenant-1",
		Created:     1000,
		Updated:     2000,
	}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByName(ctx, "test-vault")
	if err != nil {
		t.Fatalf("GetByName: %v", err)
	}
	if got.Name != v.Name {
		t.Errorf("name = %q, want %q", got.Name, v.Name)
	}
	if got.Host != v.Host {
		t.Errorf("host = %q, want %q", got.Host, v.Host)
	}
}

func TestVaultRepo_Create_Duplicate(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	v := &domain.Vault{Name: "dup", Host: "dup.kvemu.local:13000", DisplayName: "D", TenantID: "t", Created: 1, Updated: 1}
	repo.Create(ctx, v)
	err := repo.Create(ctx, v)
	if err == nil {
		t.Fatal("expected error on duplicate")
	}
	if !errors.Is(err, domain.ErrVaultExists) {
		t.Errorf("expected ErrVaultExists, got %v", err)
	}
}

func TestVaultRepo_List(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	repo.Create(ctx, &domain.Vault{Name: "prod", Host: "prod.kvemu.local:13000", DisplayName: "P", TenantID: "t", Created: 1, Updated: 1})
	repo.Create(ctx, &domain.Vault{Name: "staging", Host: "staging.kvemu.local:13000", DisplayName: "S", TenantID: "t", Created: 2, Updated: 2})

	vaults, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(vaults))
	}
	if vaults[0].Name != "prod" || vaults[1].Name != "staging" {
		t.Errorf("wrong order: %v, %v", vaults[0].Name, vaults[1].Name)
	}
}

func TestVaultRepo_GetByName_NotFound(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := repo.GetByName(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, domain.ErrVaultNotFound) {
		t.Errorf("expected ErrVaultNotFound, got %v", err)
	}
}

func TestVaultRepo_GetByHost(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	v := &domain.Vault{Name: "prod", Host: "prod.kvemu.local:13000", DisplayName: "P", TenantID: "t", Created: 1, Updated: 1}
	repo.Create(ctx, v)

	got, err := repo.GetByHost(ctx, "prod.kvemu.local:13000")
	if err != nil {
		t.Fatalf("GetByHost: %v", err)
	}
	if got.Name != "prod" {
		t.Errorf("name = %q, want %q", got.Name, "prod")
	}
}

func TestVaultRepo_GetByHost_NotFound(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	_, err := repo.GetByHost(ctx, "ghost.kvemu.local:13000")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVaultRepo_Delete(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	v := &domain.Vault{Name: "temp", Host: "temp.kvemu.local:13000", DisplayName: "T", TenantID: "t", Created: 1, Updated: 1}
	repo.Create(ctx, v)

	if err := repo.Delete(ctx, "temp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.GetByName(ctx, "temp")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestVaultRepo_Delete_NotFound(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVaultRepo_Update(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	v := &domain.Vault{Name: "test", Host: "test.kvemu.local:13000", DisplayName: "Old", TenantID: "t", Created: 1, Updated: 1}
	repo.Create(ctx, v)

	err := repo.Update(ctx, "test", &domain.Vault{DisplayName: "New"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := repo.GetByName(ctx, "test")
	if got.DisplayName != "New" {
		t.Errorf("display_name = %q, want %q", got.DisplayName, "New")
	}
}

func TestVaultRepo_Update_NotFound(t *testing.T) {
	repo, cleanup := newVaultTestDB(t)
	defer cleanup()
	ctx := context.Background()

	err := repo.Update(ctx, "nonexistent", &domain.Vault{DisplayName: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
