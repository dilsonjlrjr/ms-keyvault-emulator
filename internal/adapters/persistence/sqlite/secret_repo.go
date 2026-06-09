package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type SecretRepo struct {
	db  *sql.DB
	key []byte // AES-256 at-rest
}

func NewSecretRepo(db *sql.DB, masterKey string) *SecretRepo {
	return &SecretRepo{db: db, key: kvCrypto.DeriveKey(masterKey)}
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func newVersion() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func encodeSkipToken(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodeSkipToken(tok string) int {
	if tok == "" {
		return 0
	}
	b, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(string(b))
	return n
}

func (r *SecretRepo) sealValue(v string) (enc, nonce []byte, err error) {
	return kvCrypto.Seal(r.key, []byte(v))
}

func (r *SecretRepo) openValue(enc, nonce []byte) (string, error) {
	b, err := kvCrypto.Open(r.key, enc, nonce)
	return string(b), err
}

func marshalTags(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	b, _ := json.Marshal(tags)
	return string(b)
}

func unmarshalTags(s string) map[string]string {
	if s == "" {
		return nil
	}
	var m map[string]string
	json.Unmarshal([]byte(s), &m)
	return m
}

// ─── ensureSecret garante que o registro lógico existe ────────────────────────

func (r *SecretRepo) ensureSecret(ctx context.Context, tx *sql.Tx, vaultID, name string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NULL`, vaultID, name,
	).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO secret(vault_id,name,recovery_level) VALUES(?,?,?)`,
			vaultID, name, domain.RecoveryLevelPurgeable,
		)
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	return id, nil
}

// ─── Upsert ────────────────────────────────────────────────────────────────────

func (r *SecretRepo) Upsert(ctx context.Context, vaultID string, s *domain.SecretVersion) error {
	enc, nonce, err := r.sealValue(s.Value)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	ver := newVersion()
	s.Version = ver
	s.Attributes.Created = now
	s.Attributes.Updated = now
	if s.Attributes.RecoveryLevel == "" {
		s.Attributes.RecoveryLevel = domain.RecoveryLevelPurgeable
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	secretID, err := r.ensureSecret(ctx, tx, vaultID, s.Name)
	if err != nil {
		return err
	}

	enabled := 1
	if !s.Attributes.Enabled {
		enabled = 0
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO secret_version(secret_id,version,value_enc,value_nonce,content_type,tags_json,enabled,nbf,exp,created,updated)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		secretID, ver, enc, nonce,
		nullStr(s.ContentType), nullStr(marshalTags(s.Tags)),
		enabled, nullInt64(s.Attributes.NotBefore), nullInt64(s.Attributes.Expires),
		now, now,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE secret SET current_ver=? WHERE id=?`, ver, secretID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ─── scanVersion ──────────────────────────────────────────────────────────────

func (r *SecretRepo) scanVersion(row *sql.Row) (*domain.SecretVersion, error) {
	var (
		name, version     string
		enc, nonce        []byte
		ct, tags          sql.NullString
		enabled           int
		nbf, exp          sql.NullInt64
		created, updated  int64
		recoveryLevel     string
		managed           int
	)
	err := row.Scan(&name, &version, &enc, &nonce, &ct, &tags,
		&enabled, &nbf, &exp, &created, &updated, &recoveryLevel, &managed)
	if err != nil {
		return nil, err
	}
	value, err := r.openValue(enc, nonce)
	if err != nil {
		return nil, err
	}

	sv := &domain.SecretVersion{
		Name:        name,
		Version:     version,
		Value:       value,
		ContentType: ct.String,
		Tags:        unmarshalTags(tags.String),
		Managed:     managed == 1,
		Attributes: domain.Attributes{
			Enabled:       enabled == 1,
			Created:       created,
			Updated:       updated,
			RecoveryLevel: recoveryLevel,
		},
	}
	if nbf.Valid {
		sv.Attributes.NotBefore = &nbf.Int64
	}
	if exp.Valid {
		sv.Attributes.Expires = &exp.Int64
	}
	return sv, nil
}

// ─── Get ───────────────────────────────────────────────────────────────────────

const selectVersionFields = `
	s.name, sv.version, sv.value_enc, sv.value_nonce, sv.content_type, sv.tags_json,
	sv.enabled, sv.nbf, sv.exp, sv.created, sv.updated, s.recovery_level, s.managed`

func (r *SecretRepo) GetCurrent(ctx context.Context, vaultID, name string) (*domain.SecretVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectVersionFields+`
		FROM secret s
		JOIN secret_version sv ON sv.secret_id=s.id AND sv.version=s.current_ver
		WHERE s.vault_id=? AND s.name=? AND s.deleted_at IS NULL`,
		vaultID, name,
	)
	sv, err := r.scanVersion(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Secret", Name: name}
	}
	return sv, err
}

func (r *SecretRepo) Get(ctx context.Context, vaultID, name, version string) (*domain.SecretVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectVersionFields+`
		FROM secret s
		JOIN secret_version sv ON sv.secret_id=s.id
		WHERE s.vault_id=? AND s.name=? AND sv.version=? AND s.deleted_at IS NULL`,
		vaultID, name, version,
	)
	sv, err := r.scanVersion(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Secret", Name: name + "/" + version}
	}
	return sv, err
}

// ─── List ──────────────────────────────────────────────────────────────────────

func (r *SecretRepo) List(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.SecretVersion, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectVersionFields+`
		FROM secret s
		JOIN secret_version sv ON sv.secret_id=s.id AND sv.version=s.current_ver
		WHERE s.vault_id=? AND s.deleted_at IS NULL
		ORDER BY s.name
		LIMIT ? OFFSET ?`,
		vaultID, max+1, offset,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	return r.scanRows(rows, max, offset)
}

func (r *SecretRepo) ListVersions(ctx context.Context, vaultID, name string, max int, skipToken string) ([]*domain.SecretVersion, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)

	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectVersionFields+`
		FROM secret s
		JOIN secret_version sv ON sv.secret_id=s.id
		WHERE s.vault_id=? AND s.name=? AND s.deleted_at IS NULL
		ORDER BY sv.created DESC
		LIMIT ? OFFSET ?`,
		vaultID, name, max+1, offset,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	list, next, err := r.scanRows(rows, max, offset)
	if err != nil {
		return nil, "", err
	}
	if len(list) == 0 {
		// valida se secret existe
		var exists int
		r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NULL`, vaultID, name).Scan(&exists)
		if exists == 0 {
			return nil, "", domain.ErrNotFound{Kind: "Secret", Name: name}
		}
	}
	return list, next, nil
}

func (r *SecretRepo) scanRows(rows *sql.Rows, max, offset int) ([]*domain.SecretVersion, string, error) {
	var list []*domain.SecretVersion
	for rows.Next() {
		var (
			name, version    string
			enc, nonce       []byte
			ct, tags         sql.NullString
			enabled          int
			nbf, exp         sql.NullInt64
			created, updated int64
			recoveryLevel    string
			managed          int
		)
		if err := rows.Scan(&name, &version, &enc, &nonce, &ct, &tags,
			&enabled, &nbf, &exp, &created, &updated, &recoveryLevel, &managed); err != nil {
			return nil, "", err
		}
		value, err := r.openValue(enc, nonce)
		if err != nil {
			return nil, "", err
		}
		sv := &domain.SecretVersion{
			Name:    name, Version: version, Value: value,
			ContentType: ct.String, Tags: unmarshalTags(tags.String), Managed: managed == 1,
			Attributes: domain.Attributes{
				Enabled: enabled == 1, Created: created, Updated: updated,
				RecoveryLevel: recoveryLevel,
			},
		}
		if nbf.Valid {
			sv.Attributes.NotBefore = &nbf.Int64
		}
		if exp.Valid {
			sv.Attributes.Expires = &exp.Int64
		}
		list = append(list, sv)
	}

	var nextToken string
	if len(list) > max {
		list = list[:max]
		nextToken = encodeSkipToken(offset + max)
	}
	return list, nextToken, rows.Err()
}

// ─── UpdateAttributes ─────────────────────────────────────────────────────────

func (r *SecretRepo) UpdateAttributes(ctx context.Context, vaultID, name, version string, attrs domain.Attributes, contentType *string, tags map[string]string) error {
	now := time.Now().Unix()
	enabled := 1
	if !attrs.Enabled {
		enabled = 0
	}

	parts := []string{"enabled=?", "updated=?"}
	args := []any{enabled, now}

	if contentType != nil {
		parts = append(parts, "content_type=?")
		args = append(args, nullStr(*contentType))
	}
	if tags != nil {
		parts = append(parts, "tags_json=?")
		args = append(args, nullStr(marshalTags(tags)))
	}
	if attrs.NotBefore != nil {
		parts = append(parts, "nbf=?")
		args = append(args, *attrs.NotBefore)
	}
	if attrs.Expires != nil {
		parts = append(parts, "exp=?")
		args = append(args, *attrs.Expires)
	}

	args = append(args, vaultID, name, version)
	q := `UPDATE secret_version
		  SET ` + strings.Join(parts, ", ") + `
		  WHERE secret_id=(SELECT id FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NULL)
		  AND version=?`

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound{Kind: "Secret", Name: name + "/" + version}
	}
	return nil
}

// ─── SoftDelete ───────────────────────────────────────────────────────────────

func (r *SecretRepo) SoftDelete(ctx context.Context, vaultID, name string, schedPurge int64) (*domain.DeletedSecret, error) {
	now := time.Now().Unix()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var secretID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NULL`,
		vaultID, name).Scan(&secretID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Secret", Name: name}
	}
	if err != nil {
		return nil, err
	}

	recoveryID := fmt.Sprintf("https://%s/deletedsecrets/%s", vaultID, name)
	_, err = tx.ExecContext(ctx,
		`UPDATE secret SET deleted_at=?, scheduled_purge=?, recovery_id=? WHERE id=?`,
		now, schedPurge, recoveryID, secretID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	sv, err := r.getCurrentVersion(ctx, secretID)
	if err != nil {
		return nil, err
	}
	return &domain.DeletedSecret{
		SecretVersion:  *sv,
		DeletedDate:    now,
		ScheduledPurge: schedPurge,
		RecoveryID:     recoveryID,
	}, nil
}

func (r *SecretRepo) getCurrentVersion(ctx context.Context, secretID int64) (*domain.SecretVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT s.name, sv.version, sv.value_enc, sv.value_nonce, sv.content_type, sv.tags_json,
		       sv.enabled, sv.nbf, sv.exp, sv.created, sv.updated, s.recovery_level, s.managed
		FROM secret s
		JOIN secret_version sv ON sv.secret_id=s.id AND sv.version=s.current_ver
		WHERE s.id=?`, secretID)
	return r.scanVersion(row)
}

// ─── GetDeleted ───────────────────────────────────────────────────────────────

func (r *SecretRepo) GetDeleted(ctx context.Context, vaultID, name string) (*domain.DeletedSecret, error) {
	var (
		secretID                          int64
		deletedAt, schedPurge             int64
		recoveryID, recoveryLevel         string
		managed                           int
		currentVer                        sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT id, deleted_at, scheduled_purge, recovery_id, recovery_level, managed, current_ver
		FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`,
		vaultID, name,
	).Scan(&secretID, &deletedAt, &schedPurge, &recoveryID, &recoveryLevel, &managed, &currentVer)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "DeletedSecret", Name: name}
	}
	if err != nil {
		return nil, err
	}

	var sv *domain.SecretVersion
	if currentVer.Valid {
		sv, _ = r.getCurrentVersion(ctx, secretID)
	}
	if sv == nil {
		sv = &domain.SecretVersion{Name: name}
	}
	sv.Attributes.RecoveryLevel = recoveryLevel
	return &domain.DeletedSecret{
		SecretVersion:  *sv,
		DeletedDate:    deletedAt,
		ScheduledPurge: schedPurge,
		RecoveryID:     recoveryID,
	}, nil
}

// ─── ListDeleted ──────────────────────────────────────────────────────────────

func (r *SecretRepo) ListDeleted(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.DeletedSecret, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, deleted_at, scheduled_purge, recovery_id
		FROM secret WHERE vault_id=? AND deleted_at IS NOT NULL
		ORDER BY name LIMIT ? OFFSET ?`,
		vaultID, max+1, offset,
	)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var list []*domain.DeletedSecret
	for rows.Next() {
		var id int64
		var name, recoveryID string
		var del, purge int64
		if err := rows.Scan(&id, &name, &del, &purge, &recoveryID); err != nil {
			return nil, "", err
		}
		sv, _ := r.getCurrentVersion(ctx, id)
		if sv == nil {
			sv = &domain.SecretVersion{Name: name}
		}
		list = append(list, &domain.DeletedSecret{
			SecretVersion:  *sv,
			DeletedDate:    del,
			ScheduledPurge: purge,
			RecoveryID:     recoveryID,
		})
	}

	var nextToken string
	if len(list) > max {
		list = list[:max]
		nextToken = encodeSkipToken(offset + max)
	}
	return list, nextToken, rows.Err()
}

// ─── Recover ──────────────────────────────────────────────────────────────────

func (r *SecretRepo) Recover(ctx context.Context, vaultID, name string) (*domain.SecretVersion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`,
		vaultID, name).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "DeletedSecret", Name: name}
	}
	if err != nil {
		return nil, err
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE secret SET deleted_at=NULL, scheduled_purge=NULL, recovery_id=NULL WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.getCurrentVersion(ctx, id)
}

// ─── Purge ────────────────────────────────────────────────────────────────────

func (r *SecretRepo) Purge(ctx context.Context, vaultID, name string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM secret WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`,
		vaultID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound{Kind: "DeletedSecret", Name: name}
	}
	return nil
}

// ─── IsDeleted ────────────────────────────────────────────────────────────────

func (r *SecretRepo) IsDeleted(ctx context.Context, vaultID, name string) (bool, error) {
	var del sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT deleted_at FROM secret WHERE vault_id=? AND name=?`, vaultID, name).Scan(&del)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return del.Valid, err
}

// ─── helpers SQL ──────────────────────────────────────────────────────────────

func nullStr(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func nullInt64(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}
