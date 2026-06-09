package sqlite

import (
	"context"
	"database/sql"
	"time"
)

// AuditRepo escreve entradas no audit_log.
type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(db *sql.DB) *AuditRepo { return &AuditRepo{db: db} }

// Log insere uma entrada no audit_log. Falhas são silenciosas (não quebram o fluxo principal).
func (a *AuditRepo) Log(ctx context.Context, actor, op, resource string, status int, detail string) {
	a.db.ExecContext(ctx,
		`INSERT INTO audit_log(ts, actor, op, resource, status, detail) VALUES (?,?,?,?,?,?)`,
		time.Now().Unix(), actor, op, resource, status, detail,
	)
}
