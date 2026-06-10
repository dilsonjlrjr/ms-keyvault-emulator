package sqlite_test

import (
	"context"
	"os"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
)

func newTestAuditDB(t *testing.T) (*sqlite.AuditRepo, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "kvemu-audit-test-*.db")
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
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY, ts INTEGER NOT NULL,
		actor TEXT, op TEXT NOT NULL, resource TEXT,
		status INTEGER NOT NULL, detail TEXT
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	repo := sqlite.NewAuditRepo(db)
	cleanup := func() {
		db.Close()
		os.Remove(f.Name())
	}
	return repo, cleanup
}

func TestAuditRepo_Log(t *testing.T) {
	repo, cleanup := newTestAuditDB(t)
	defer cleanup()
	ctx := context.Background()

	repo.Log(ctx, "test-actor", "GET", "/secrets/my-secret", 200, "success")
}

func TestAuditRepo_LogDoesNotPanic(t *testing.T) {
	repo, cleanup := newTestAuditDB(t)
	defer cleanup()
	ctx := context.Background()

	repo.Log(ctx, "", "", "", 0, "")
	repo.Log(ctx, "actor", "PUT", "/keys/test", 201, "")
	repo.Log(ctx, "actor", "DELETE", "/secrets/test", 204, "deleted")
}
