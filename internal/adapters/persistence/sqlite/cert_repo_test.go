package sqlite_test

import (
	"context"
	"os"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

func newTestCertDB(t *testing.T) (*sqlite.CertRepo, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "kvemu-cert-test-*.db")
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
	CREATE TABLE IF NOT EXISTS vcert (
		id INTEGER PRIMARY KEY, vault_id TEXT NOT NULL REFERENCES vault(id),
		name TEXT NOT NULL, current_ver TEXT, policy_json TEXT,
		deleted_at INTEGER, scheduled_purge INTEGER, recovery_id TEXT,
		recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable', UNIQUE (vault_id, name)
	);
	CREATE TABLE IF NOT EXISTS vcert_version (
		id INTEGER PRIMARY KEY, cert_id INTEGER NOT NULL REFERENCES vcert(id) ON DELETE CASCADE,
		version TEXT NOT NULL, cer BLOB, x5t TEXT, x5t_s256 TEXT,
		backing_secret_ver TEXT, backing_key_ver TEXT,
		enabled INTEGER NOT NULL DEFAULT 1, nbf INTEGER, exp INTEGER,
		created INTEGER NOT NULL, updated INTEGER NOT NULL,
		UNIQUE (cert_id, version)
	);
	CREATE TABLE IF NOT EXISTS vcert_contacts (
		vault_id TEXT PRIMARY KEY, contacts_json TEXT, created INTEGER NOT NULL, updated INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS vcert_issuer (
		id INTEGER PRIMARY KEY, vault_id TEXT NOT NULL, name TEXT NOT NULL,
		provider TEXT, credentials_json TEXT, org_details_json TEXT, attributes_json TEXT,
		created INTEGER NOT NULL, updated INTEGER NOT NULL,
		UNIQUE (vault_id, name)
	);
	INSERT OR IGNORE INTO vault(id,dns_name,tenant_id,created) VALUES('test-vault','https://test-vault','tenant-1',0);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	repo := sqlite.NewCertRepo(db)
	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}
	return repo, cleanup
}

func TestCertRepo_UpsertAndGet(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	cv := &domain.CertVersion{
		Name:   "test-cert",
		CerDER: []byte{0x30, 0x82, 0x01, 0x00},
		X5T:    "abc123",
		Attributes: domain.Attributes{
			Enabled:       true,
			RecoveryLevel: domain.RecoveryLevelPurgeable,
		},
	}
	policy := map[string]any{"issuer": map[string]any{"name": "Self"}}
	if err := repo.Upsert(ctx, vaultID, cv, policy); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if cv.Version == "" {
		t.Fatal("version deve ser gerada no Upsert")
	}

	got, gotPolicy, err := repo.GetCurrent(ctx, vaultID, "test-cert")
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if got.X5T != "abc123" {
		t.Errorf("x5t: got %q, want %q", got.X5T, "abc123")
	}
	if got.Version != cv.Version {
		t.Errorf("version mismatch: got %q, want %q", got.Version, cv.Version)
	}
	if gotPolicy == nil {
		t.Fatal("policy não deve ser nil")
	}
}

func TestCertRepo_MultipleVersions(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	for i, x5t := range []string{"v1", "v2", "v3"} {
		cv := &domain.CertVersion{
			Name:   "rotating-cert",
			CerDER: []byte{0x30},
			X5T:    x5t,
			Attributes: domain.Attributes{
				Enabled:       true,
				RecoveryLevel: domain.RecoveryLevelPurgeable,
			},
		}
		if err := repo.Upsert(ctx, vaultID, cv, nil); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}

	got, _, err := repo.GetCurrent(ctx, vaultID, "rotating-cert")
	if err != nil {
		t.Fatal(err)
	}
	if got.X5T != "v3" {
		t.Errorf("current x5t: got %q, want v3", got.X5T)
	}

	list, _, err := repo.ListVersions(ctx, vaultID, "rotating-cert", 25, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("versions: got %d, want 3", len(list))
	}
}

func TestCertRepo_SoftDeleteAndRecover(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	cv := &domain.CertVersion{
		Name: "to-delete", CerDER: []byte{0x30},
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, cv, nil)

	dc, err := repo.SoftDelete(ctx, vaultID, "to-delete", 9999999999)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if dc.DeletedDate == 0 {
		t.Fatal("deletedDate deve ser setado")
	}

	_, _, err = repo.GetCurrent(ctx, vaultID, "to-delete")
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
	if rec.CerDER == nil {
		t.Error("recovered cer não deve ser nil")
	}

	got2, _, err := repo.GetCurrent(ctx, vaultID, "to-delete")
	if err != nil {
		t.Fatalf("get after recover: %v", err)
	}
	if got2.CerDER == nil {
		t.Error("cer after recover não deve ser nil")
	}
}

func TestCertRepo_Purge(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	cv := &domain.CertVersion{
		Name: "to-purge", CerDER: []byte{0x30},
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	repo.Upsert(ctx, vaultID, cv, nil)
	repo.SoftDelete(ctx, vaultID, "to-purge", 9999999999)

	if err := repo.Purge(ctx, vaultID, "to-purge"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	_, err := repo.GetDeleted(ctx, vaultID, "to-purge")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("após purge deve ser ErrNotFound, got %v", err)
	}
}

func TestCertRepo_NotFound(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	_, _, err := repo.GetCurrent(ctx, vaultID, "nao-existe")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("esperado ErrNotFound, got %T: %v", err, err)
	}
}

func TestCertRepo_List(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	for _, name := range []string{"cert-a", "cert-b", "cert-c"} {
		cv := &domain.CertVersion{
			Name: name, CerDER: []byte{0x30},
			Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
		}
		repo.Upsert(ctx, vaultID, cv, nil)
	}

	list, _, err := repo.List(ctx, vaultID, 25, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("list: got %d, want 3", len(list))
	}
}

func TestCertRepo_Policy(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	cv := &domain.CertVersion{
		Name: "policy-cert", CerDER: []byte{0x30},
		Attributes: domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable},
	}
	policy := map[string]any{
		"issuer":  map[string]any{"name": "Self"},
		"keyType": "RSA",
	}
	repo.Upsert(ctx, vaultID, cv, policy)

	got, err := repo.GetPolicy(ctx, vaultID, "policy-cert")
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if got["keyType"] != "RSA" {
		t.Errorf("keyType: got %v", got["keyType"])
	}

	newPolicy := map[string]any{"keyType": "EC"}
	if err := repo.UpdatePolicy(ctx, vaultID, "policy-cert", newPolicy); err != nil {
		t.Fatalf("update policy: %v", err)
	}

	got2, err := repo.GetPolicy(ctx, vaultID, "policy-cert")
	if err != nil {
		t.Fatal(err)
	}
	if got2["keyType"] != "EC" {
		t.Errorf("updated keyType: got %v", got2["keyType"])
	}
}

func TestCertRepo_Contacts(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	contacts, err := repo.GetContacts(ctx, vaultID)
	if err != nil {
		t.Fatalf("get empty contacts: %v", err)
	}
	if len(contacts) != 0 {
		t.Errorf("empty contacts: got %d, want 0", len(contacts))
	}

	newContacts := []map[string]any{
		{"email": "admin@example.com", "name": "Admin"},
	}
	if err := repo.SetContacts(ctx, vaultID, newContacts); err != nil {
		t.Fatalf("set contacts: %v", err)
	}

	got, err := repo.GetContacts(ctx, vaultID)
	if err != nil {
		t.Fatalf("get contacts: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("contacts: got %d, want 1", len(got))
	}
	if got[0]["email"] != "admin@example.com" {
		t.Errorf("email: got %v", got[0]["email"])
	}

	if err := repo.DeleteContacts(ctx, vaultID); err != nil {
		t.Fatalf("delete contacts: %v", err)
	}

	got2, err := repo.GetContacts(ctx, vaultID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 0 {
		t.Errorf("after delete: got %d, want 0", len(got2))
	}
}

func TestCertRepo_Issuers(t *testing.T) {
	repo, cleanup := newTestCertDB(t)
	defer cleanup()
	ctx := context.Background()

	iss := &sqlite.IssuerData{
		Name:     "test-issuer",
		Provider: "Self",
		Attributes: map[string]any{
			"enabled": true,
		},
	}
	if err := repo.SetIssuer(ctx, vaultID, iss); err != nil {
		t.Fatalf("set issuer: %v", err)
	}

	got, err := repo.GetIssuer(ctx, vaultID, "test-issuer")
	if err != nil {
		t.Fatalf("get issuer: %v", err)
	}
	if got.Provider != "Self" {
		t.Errorf("provider: got %q, want %q", got.Provider, "Self")
	}
	if got.Created == 0 {
		t.Fatal("created deve ser setado")
	}

	list, err := repo.ListIssuers(ctx, vaultID)
	if err != nil {
		t.Fatalf("list issuers: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list: got %d, want 1", len(list))
	}

	if err := repo.DeleteIssuer(ctx, vaultID, "test-issuer"); err != nil {
		t.Fatalf("delete issuer: %v", err)
	}

	_, err = repo.GetIssuer(ctx, vaultID, "test-issuer")
	if _, ok := err.(domain.ErrNotFound); !ok {
		t.Fatalf("after delete: esperado ErrNotFound, got %v", err)
	}
}
