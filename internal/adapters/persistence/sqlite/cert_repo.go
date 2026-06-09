package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type CertRepo struct {
	db *sql.DB
}

func NewCertRepo(db *sql.DB) *CertRepo {
	return &CertRepo{db: db}
}

func (r *CertRepo) ensureCert(ctx context.Context, tx *sql.Tx, vaultID, name string, policy map[string]any) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM vcert WHERE vault_id=? AND name=? AND deleted_at IS NULL`, vaultID, name,
	).Scan(&id)
	if err == sql.ErrNoRows {
		policyJSON, _ := json.Marshal(policy)
		res, err := tx.ExecContext(ctx,
			`INSERT INTO vcert(vault_id,name,policy_json,recovery_level) VALUES(?,?,?,?)`,
			vaultID, name, string(policyJSON), domain.RecoveryLevelPurgeable)
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *CertRepo) Upsert(ctx context.Context, vaultID string, cv *domain.CertVersion, policy map[string]any) error {
	now := time.Now().Unix()
	ver := newVersion()
	cv.Version = ver
	cv.Attributes.Created = now
	cv.Attributes.Updated = now
	if cv.Attributes.RecoveryLevel == "" {
		cv.Attributes.RecoveryLevel = domain.RecoveryLevelPurgeable
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	certID, err := r.ensureCert(ctx, tx, vaultID, cv.Name, policy)
	if err != nil {
		return err
	}

	enabled := 1
	if !cv.Attributes.Enabled {
		enabled = 0
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO vcert_version(cert_id,version,cer,x5t,x5t_s256,backing_secret_ver,backing_key_ver,enabled,nbf,exp,created,updated)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		certID, ver, cv.CerDER, nullStr(cv.X5T), nullStr(cv.X5TS256),
		nullStr(cv.BackingSecretVer), nullStr(cv.BackingKeyVer),
		enabled, nullInt64(cv.Attributes.NotBefore), nullInt64(cv.Attributes.Expires),
		now, now,
	)
	if err != nil {
		return err
	}

	policyJSON, _ := json.Marshal(policy)
	_, err = tx.ExecContext(ctx,
		`UPDATE vcert SET current_ver=?, policy_json=? WHERE id=?`, ver, string(policyJSON), certID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

const selectCertFields = `
	vc.name, cv.version, cv.cer, cv.x5t, cv.x5t_s256,
	cv.backing_secret_ver, cv.backing_key_ver,
	cv.enabled, cv.nbf, cv.exp, cv.created, cv.updated, vc.recovery_level, vc.policy_json`

func (r *CertRepo) scanCertVersion(row *sql.Row) (*domain.CertVersion, map[string]any, error) {
	var (
		name, version                                string
		cer                                          []byte
		x5t, x5ts256, bsv, bkv                      sql.NullString
		enabled                                      int
		nbf, exp                                     sql.NullInt64
		created, updated                             int64
		recoveryLevel, policyJSON                    string
	)
	err := row.Scan(&name, &version, &cer, &x5t, &x5ts256, &bsv, &bkv,
		&enabled, &nbf, &exp, &created, &updated, &recoveryLevel, &policyJSON)
	if err != nil {
		return nil, nil, err
	}

	var policy map[string]any
	json.Unmarshal([]byte(policyJSON), &policy)

	cv := &domain.CertVersion{
		Name: name, Version: version, CerDER: cer,
		X5T: x5t.String, X5TS256: x5ts256.String,
		BackingSecretVer: bsv.String, BackingKeyVer: bkv.String,
		Attributes: domain.Attributes{
			Enabled: enabled == 1, Created: created, Updated: updated,
			RecoveryLevel: recoveryLevel,
		},
	}
	if nbf.Valid {
		cv.Attributes.NotBefore = &nbf.Int64
	}
	if exp.Valid {
		cv.Attributes.Expires = &exp.Int64
	}
	return cv, policy, nil
}

func (r *CertRepo) GetCurrent(ctx context.Context, vaultID, name string) (*domain.CertVersion, map[string]any, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectCertFields+`
		FROM vcert vc JOIN vcert_version cv ON cv.cert_id=vc.id AND cv.version=vc.current_ver
		WHERE vc.vault_id=? AND vc.name=? AND vc.deleted_at IS NULL`, vaultID, name)
	cv, policy, err := r.scanCertVersion(row)
	if err == sql.ErrNoRows {
		return nil, nil, domain.ErrNotFound{Kind: "Certificate", Name: name}
	}
	return cv, policy, err
}

func (r *CertRepo) Get(ctx context.Context, vaultID, name, version string) (*domain.CertVersion, map[string]any, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectCertFields+`
		FROM vcert vc JOIN vcert_version cv ON cv.cert_id=vc.id
		WHERE vc.vault_id=? AND vc.name=? AND cv.version=? AND vc.deleted_at IS NULL`, vaultID, name, version)
	cv, policy, err := r.scanCertVersion(row)
	if err == sql.ErrNoRows {
		return nil, nil, domain.ErrNotFound{Kind: "Certificate", Name: name + "/" + version}
	}
	return cv, policy, err
}

func (r *CertRepo) GetPolicy(ctx context.Context, vaultID, name string) (map[string]any, error) {
	var policyJSON string
	err := r.db.QueryRowContext(ctx,
		`SELECT policy_json FROM vcert WHERE vault_id=? AND name=? AND deleted_at IS NULL`,
		vaultID, name).Scan(&policyJSON)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Certificate", Name: name}
	}
	if err != nil {
		return nil, err
	}
	var policy map[string]any
	json.Unmarshal([]byte(policyJSON), &policy)
	return policy, nil
}

func (r *CertRepo) UpdatePolicy(ctx context.Context, vaultID, name string, policy map[string]any) error {
	b, _ := json.Marshal(policy)
	res, err := r.db.ExecContext(ctx,
		`UPDATE vcert SET policy_json=? WHERE vault_id=? AND name=? AND deleted_at IS NULL`,
		string(b), vaultID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound{Kind: "Certificate", Name: name}
	}
	return nil
}

func (r *CertRepo) List(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.CertVersion, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectCertFields+`
		FROM vcert vc JOIN vcert_version cv ON cv.cert_id=vc.id AND cv.version=vc.current_ver
		WHERE vc.vault_id=? AND vc.deleted_at IS NULL ORDER BY vc.name LIMIT ? OFFSET ?`,
		vaultID, max+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	return r.scanCertRows(rows, max, offset)
}

func (r *CertRepo) ListVersions(ctx context.Context, vaultID, name string, max int, skipToken string) ([]*domain.CertVersion, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectCertFields+`
		FROM vcert vc JOIN vcert_version cv ON cv.cert_id=vc.id
		WHERE vc.vault_id=? AND vc.name=? AND vc.deleted_at IS NULL
		ORDER BY cv.created DESC LIMIT ? OFFSET ?`, vaultID, name, max+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	return r.scanCertRows(rows, max, offset)
}

func (r *CertRepo) scanCertRows(rows *sql.Rows, max, offset int) ([]*domain.CertVersion, string, error) {
	var list []*domain.CertVersion
	for rows.Next() {
		var (
			name, version, recoveryLevel, policyJSON string
			cer                                       []byte
			x5t, x5ts256, bsv, bkv                   sql.NullString
			enabled                                   int
			nbf, exp                                  sql.NullInt64
			created, updated                          int64
		)
		if err := rows.Scan(&name, &version, &cer, &x5t, &x5ts256, &bsv, &bkv,
			&enabled, &nbf, &exp, &created, &updated, &recoveryLevel, &policyJSON); err != nil {
			return nil, "", err
		}
		cv := &domain.CertVersion{
			Name: name, Version: version, CerDER: cer,
			X5T: x5t.String, X5TS256: x5ts256.String,
			BackingSecretVer: bsv.String, BackingKeyVer: bkv.String,
			Attributes: domain.Attributes{Enabled: enabled == 1, Created: created, Updated: updated, RecoveryLevel: recoveryLevel},
		}
		if nbf.Valid {
			cv.Attributes.NotBefore = &nbf.Int64
		}
		if exp.Valid {
			cv.Attributes.Expires = &exp.Int64
		}
		list = append(list, cv)
	}
	var next string
	if len(list) > max {
		list = list[:max]
		next = encodeSkipToken(offset + max)
	}
	return list, next, rows.Err()
}

func (r *CertRepo) SoftDelete(ctx context.Context, vaultID, name string, schedPurge int64) (*domain.DeletedCert, error) {
	now := time.Now().Unix()
	tx, _ := r.db.BeginTx(ctx, nil)
	defer tx.Rollback()

	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM vcert WHERE vault_id=? AND name=? AND deleted_at IS NULL`, vaultID, name).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Certificate", Name: name}
	}
	recovID := fmt.Sprintf("https://%s/deletedcertificates/%s", vaultID, name)
	tx.ExecContext(ctx, `UPDATE vcert SET deleted_at=?,scheduled_purge=?,recovery_id=? WHERE id=?`, now, schedPurge, recovID, id)
	tx.Commit()

	cv, _, _ := r.getCurrentCertVersion(ctx, id)
	if cv == nil {
		cv = &domain.CertVersion{Name: name}
	}
	return &domain.DeletedCert{CertVersion: *cv, DeletedDate: now, ScheduledPurge: schedPurge, RecoveryID: recovID}, nil
}

func (r *CertRepo) getCurrentCertVersion(ctx context.Context, certID int64) (*domain.CertVersion, map[string]any, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT vc.name, cv.version, cv.cer, cv.x5t, cv.x5t_s256,
		       cv.backing_secret_ver, cv.backing_key_ver,
		       cv.enabled, cv.nbf, cv.exp, cv.created, cv.updated, vc.recovery_level, vc.policy_json
		FROM vcert vc JOIN vcert_version cv ON cv.cert_id=vc.id AND cv.version=vc.current_ver
		WHERE vc.id=?`, certID)
	return r.scanCertVersion(row)
}

func (r *CertRepo) GetDeleted(ctx context.Context, vaultID, name string) (*domain.DeletedCert, error) {
	var id int64
	var del, purge int64
	var recovID string
	err := r.db.QueryRowContext(ctx,
		`SELECT id,deleted_at,scheduled_purge,recovery_id FROM vcert WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`,
		vaultID, name).Scan(&id, &del, &purge, &recovID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "DeletedCertificate", Name: name}
	}
	cv, _, _ := r.getCurrentCertVersion(ctx, id)
	if cv == nil {
		cv = &domain.CertVersion{Name: name}
	}
	return &domain.DeletedCert{CertVersion: *cv, DeletedDate: del, ScheduledPurge: purge, RecoveryID: recovID}, nil
}

func (r *CertRepo) ListDeleted(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.DeletedCert, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,deleted_at,scheduled_purge,recovery_id FROM vcert WHERE vault_id=? AND deleted_at IS NOT NULL ORDER BY name LIMIT ? OFFSET ?`,
		vaultID, max+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var list []*domain.DeletedCert
	for rows.Next() {
		var id int64
		var name, recovID string
		var del, purge int64
		rows.Scan(&id, &name, &del, &purge, &recovID)
		cv, _, _ := r.getCurrentCertVersion(ctx, id)
		if cv == nil {
			cv = &domain.CertVersion{Name: name}
		}
		list = append(list, &domain.DeletedCert{CertVersion: *cv, DeletedDate: del, ScheduledPurge: purge, RecoveryID: recovID})
	}
	var next string
	if len(list) > max {
		list = list[:max]
		next = encodeSkipToken(offset + max)
	}
	return list, next, rows.Err()
}

func (r *CertRepo) Recover(ctx context.Context, vaultID, name string) (*domain.CertVersion, error) {
	tx, _ := r.db.BeginTx(ctx, nil)
	defer tx.Rollback()
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM vcert WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`, vaultID, name).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "DeletedCertificate", Name: name}
	}
	tx.ExecContext(ctx, `UPDATE vcert SET deleted_at=NULL,scheduled_purge=NULL,recovery_id=NULL WHERE id=?`, id)
	tx.Commit()
	cv, _, _ := r.getCurrentCertVersion(ctx, id)
	return cv, nil
}

func (r *CertRepo) Purge(ctx context.Context, vaultID, name string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM vcert WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`, vaultID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound{Kind: "DeletedCertificate", Name: name}
	}
	return nil
}
