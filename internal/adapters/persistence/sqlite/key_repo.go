package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type KeyRepo struct {
	db  *sql.DB
	key []byte
}

func NewKeyRepo(db *sql.DB, masterKey string) *KeyRepo {
	return &KeyRepo{db: db, key: kvCrypto.DeriveKey(masterKey)}
}

func (r *KeyRepo) sealJSON(v map[string]any) (enc, nonce []byte, err error) {
	if len(v) == 0 {
		return nil, nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return kvCrypto.Seal(r.key, b)
}

func (r *KeyRepo) openJSON(enc, nonce []byte) (map[string]any, error) {
	if len(enc) == 0 {
		return nil, nil
	}
	b, err := kvCrypto.Open(r.key, enc, nonce)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(b, &m)
}

func (r *KeyRepo) ensureKey(ctx context.Context, tx *sql.Tx, vaultID, name string) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM vkey WHERE vault_id=? AND name=? AND deleted_at IS NULL`, vaultID, name,
	).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO vkey(vault_id,name,recovery_level) VALUES(?,?,?)`,
			vaultID, name, domain.RecoveryLevelPurgeable)
		if err != nil {
			return 0, err
		}
		id, _ = res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *KeyRepo) Upsert(ctx context.Context, vaultID string, k *domain.KeyVersion, privJWK map[string]any) error {
	enc, nonce, err := r.sealJSON(privJWK)
	if err != nil {
		return err
	}
	pubJSON, _ := json.Marshal(k.PubJWK)
	opsJSON, _ := json.Marshal(k.KeyOps)

	now := time.Now().Unix()
	ver := newVersion()
	k.Version = ver
	k.Attributes.Created = now
	k.Attributes.Updated = now
	if k.Attributes.RecoveryLevel == "" {
		k.Attributes.RecoveryLevel = domain.RecoveryLevelPurgeable
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keyID, err := r.ensureKey(ctx, tx, vaultID, k.Name)
	if err != nil {
		return err
	}

	enabled := 1
	if !k.Attributes.Enabled {
		enabled = 0
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO vkey_version(key_id,version,kty,crv,key_size,key_ops_json,jwk_pub_json,jwk_priv_enc,jwk_priv_nonce,enabled,nbf,exp,created,updated)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		keyID, ver, k.Kty, nullStr(k.Crv), nullInt(k.KeySize),
		string(opsJSON), string(pubJSON),
		enc, nonce,
		enabled, nullInt64(k.Attributes.NotBefore), nullInt64(k.Attributes.Expires),
		now, now,
	)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE vkey SET current_ver=? WHERE id=?`, ver, keyID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

const selectKeyFields = `
	vk.name, kv.version, kv.kty, kv.crv, kv.key_size, kv.key_ops_json, kv.jwk_pub_json,
	kv.enabled, kv.nbf, kv.exp, kv.created, kv.updated, vk.recovery_level`

func (r *KeyRepo) scanKeyVersion(row *sql.Row) (*domain.KeyVersion, error) {
	var (
		name, version, kty string
		crv                sql.NullString
		keySize            sql.NullInt64
		opsJSON, pubJSON   string
		enabled            int
		nbf, exp           sql.NullInt64
		created, updated   int64
		recoveryLevel      string
	)
	err := row.Scan(&name, &version, &kty, &crv, &keySize, &opsJSON, &pubJSON,
		&enabled, &nbf, &exp, &created, &updated, &recoveryLevel)
	if err != nil {
		return nil, err
	}

	var ops []string
	json.Unmarshal([]byte(opsJSON), &ops)
	var pub map[string]any
	json.Unmarshal([]byte(pubJSON), &pub)
	if pub == nil {
		pub = map[string]any{}
	}

	kv := &domain.KeyVersion{
		Name: name, Version: version, Kty: kty,
		Crv: crv.String, KeyOps: ops, PubJWK: pub,
		Attributes: domain.Attributes{
			Enabled: enabled == 1, Created: created, Updated: updated,
			RecoveryLevel: recoveryLevel,
		},
	}
	if keySize.Valid {
		kv.KeySize = int(keySize.Int64)
	}
	if nbf.Valid {
		kv.Attributes.NotBefore = &nbf.Int64
	}
	if exp.Valid {
		kv.Attributes.Expires = &exp.Int64
	}
	return kv, nil
}

func (r *KeyRepo) GetCurrent(ctx context.Context, vaultID, name string) (*domain.KeyVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectKeyFields+`
		FROM vkey vk JOIN vkey_version kv ON kv.key_id=vk.id AND kv.version=vk.current_ver
		WHERE vk.vault_id=? AND vk.name=? AND vk.deleted_at IS NULL`, vaultID, name)
	kv, err := r.scanKeyVersion(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Key", Name: name}
	}
	return kv, err
}

func (r *KeyRepo) Get(ctx context.Context, vaultID, name, version string) (*domain.KeyVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT `+selectKeyFields+`
		FROM vkey vk JOIN vkey_version kv ON kv.key_id=vk.id
		WHERE vk.vault_id=? AND vk.name=? AND kv.version=? AND vk.deleted_at IS NULL`, vaultID, name, version)
	kv, err := r.scanKeyVersion(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Key", Name: name + "/" + version}
	}
	return kv, err
}

func (r *KeyRepo) GetPriv(ctx context.Context, vaultID, name, version string) (map[string]any, error) {
	var enc, nonce []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT kv.jwk_priv_enc, kv.jwk_priv_nonce
		FROM vkey vk JOIN vkey_version kv ON kv.key_id=vk.id
		WHERE vk.vault_id=? AND vk.name=? AND kv.version=?`, vaultID, name, version,
	).Scan(&enc, &nonce)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Key", Name: name + "/" + version}
	}
	if err != nil {
		return nil, err
	}
	return r.openJSON(enc, nonce)
}

func (r *KeyRepo) List(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.KeyVersion, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectKeyFields+`
		FROM vkey vk JOIN vkey_version kv ON kv.key_id=vk.id AND kv.version=vk.current_ver
		WHERE vk.vault_id=? AND vk.deleted_at IS NULL ORDER BY vk.name LIMIT ? OFFSET ?`,
		vaultID, max+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	return r.scanKeyRows(rows, max, offset)
}

func (r *KeyRepo) ListVersions(ctx context.Context, vaultID, name string, max int, skipToken string) ([]*domain.KeyVersion, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectKeyFields+`
		FROM vkey vk JOIN vkey_version kv ON kv.key_id=vk.id
		WHERE vk.vault_id=? AND vk.name=? AND vk.deleted_at IS NULL
		ORDER BY kv.created DESC LIMIT ? OFFSET ?`,
		vaultID, name, max+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	return r.scanKeyRows(rows, max, offset)
}

func (r *KeyRepo) scanKeyRows(rows *sql.Rows, max, offset int) ([]*domain.KeyVersion, string, error) {
	var list []*domain.KeyVersion
	for rows.Next() {
		var (
			name, version, kty string
			crv                sql.NullString
			keySize            sql.NullInt64
			opsJSON, pubJSON   string
			enabled            int
			nbf, exp           sql.NullInt64
			created, updated   int64
			recoveryLevel      string
		)
		if err := rows.Scan(&name, &version, &kty, &crv, &keySize, &opsJSON, &pubJSON,
			&enabled, &nbf, &exp, &created, &updated, &recoveryLevel); err != nil {
			return nil, "", err
		}
		var ops []string
		json.Unmarshal([]byte(opsJSON), &ops)
		var pub map[string]any
		json.Unmarshal([]byte(pubJSON), &pub)
		kv := &domain.KeyVersion{
			Name: name, Version: version, Kty: kty, Crv: crv.String, KeyOps: ops, PubJWK: pub,
			Attributes: domain.Attributes{Enabled: enabled == 1, Created: created, Updated: updated, RecoveryLevel: recoveryLevel},
		}
		if keySize.Valid {
			kv.KeySize = int(keySize.Int64)
		}
		if nbf.Valid {
			kv.Attributes.NotBefore = &nbf.Int64
		}
		if exp.Valid {
			kv.Attributes.Expires = &exp.Int64
		}
		list = append(list, kv)
	}
	var next string
	if len(list) > max {
		list = list[:max]
		next = encodeSkipToken(offset + max)
	}
	return list, next, rows.Err()
}

func (r *KeyRepo) UpdateAttributes(ctx context.Context, vaultID, name, version string, attrs domain.Attributes, keyOps []string, tags map[string]string) error {
	now := time.Now().Unix()
	enabled := 1
	if !attrs.Enabled {
		enabled = 0
	}
	args := []any{enabled, now}
	set := "kv.enabled=?, kv.updated=?"
	if len(keyOps) > 0 {
		b, _ := json.Marshal(keyOps)
		set += ", kv.key_ops_json=?"
		args = append(args, string(b))
	}
	if attrs.NotBefore != nil {
		set += ", kv.nbf=?"
		args = append(args, *attrs.NotBefore)
	}
	if attrs.Expires != nil {
		set += ", kv.exp=?"
		args = append(args, *attrs.Expires)
	}
	args = append(args, vaultID, name, version)
	res, err := r.db.ExecContext(ctx,
		`UPDATE vkey_version kv SET `+set+
			` WHERE kv.key_id=(SELECT id FROM vkey WHERE vault_id=? AND name=? AND deleted_at IS NULL) AND kv.version=?`,
		args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound{Kind: "Key", Name: name + "/" + version}
	}
	return nil
}

func (r *KeyRepo) SoftDelete(ctx context.Context, vaultID, name string, schedPurge int64) (*domain.DeletedKey, error) {
	now := time.Now().Unix()
	tx, _ := r.db.BeginTx(ctx, nil)
	defer tx.Rollback()

	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM vkey WHERE vault_id=? AND name=? AND deleted_at IS NULL`, vaultID, name).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "Key", Name: name}
	}
	recovID := fmt.Sprintf("https://%s/deletedkeys/%s", vaultID, name)
	tx.ExecContext(ctx, `UPDATE vkey SET deleted_at=?,scheduled_purge=?,recovery_id=? WHERE id=?`, now, schedPurge, recovID, id)
	tx.Commit()

	kv, _ := r.getCurrentKeyVersion(ctx, id)
	if kv == nil {
		kv = &domain.KeyVersion{Name: name}
	}
	return &domain.DeletedKey{KeyVersion: *kv, DeletedDate: now, ScheduledPurge: schedPurge, RecoveryID: recovID}, nil
}

func (r *KeyRepo) getCurrentKeyVersion(ctx context.Context, keyID int64) (*domain.KeyVersion, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT vk.name, kv.version, kv.kty, kv.crv, kv.key_size, kv.key_ops_json, kv.jwk_pub_json,
		       kv.enabled, kv.nbf, kv.exp, kv.created, kv.updated, vk.recovery_level
		FROM vkey vk JOIN vkey_version kv ON kv.key_id=vk.id AND kv.version=vk.current_ver
		WHERE vk.id=?`, keyID)
	return r.scanKeyVersion(row)
}

func (r *KeyRepo) GetDeleted(ctx context.Context, vaultID, name string) (*domain.DeletedKey, error) {
	var id int64
	var del, purge int64
	var recovID string
	err := r.db.QueryRowContext(ctx,
		`SELECT id,deleted_at,scheduled_purge,recovery_id FROM vkey WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`,
		vaultID, name).Scan(&id, &del, &purge, &recovID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "DeletedKey", Name: name}
	}
	kv, _ := r.getCurrentKeyVersion(ctx, id)
	if kv == nil {
		kv = &domain.KeyVersion{Name: name}
	}
	return &domain.DeletedKey{KeyVersion: *kv, DeletedDate: del, ScheduledPurge: purge, RecoveryID: recovID}, nil
}

func (r *KeyRepo) ListDeleted(ctx context.Context, vaultID string, max int, skipToken string) ([]*domain.DeletedKey, string, error) {
	if max <= 0 || max > 25 {
		max = 25
	}
	offset := decodeSkipToken(skipToken)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,deleted_at,scheduled_purge,recovery_id FROM vkey WHERE vault_id=? AND deleted_at IS NOT NULL ORDER BY name LIMIT ? OFFSET ?`,
		vaultID, max+1, offset)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var list []*domain.DeletedKey
	for rows.Next() {
		var id int64
		var name, recovID string
		var del, purge int64
		rows.Scan(&id, &name, &del, &purge, &recovID)
		kv, _ := r.getCurrentKeyVersion(ctx, id)
		if kv == nil {
			kv = &domain.KeyVersion{Name: name}
		}
		list = append(list, &domain.DeletedKey{KeyVersion: *kv, DeletedDate: del, ScheduledPurge: purge, RecoveryID: recovID})
	}
	var next string
	if len(list) > max {
		list = list[:max]
		next = encodeSkipToken(offset + max)
	}
	return list, next, rows.Err()
}

func (r *KeyRepo) Recover(ctx context.Context, vaultID, name string) (*domain.KeyVersion, error) {
	tx, _ := r.db.BeginTx(ctx, nil)
	defer tx.Rollback()
	var id int64
	err := tx.QueryRowContext(ctx,
		`SELECT id FROM vkey WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`, vaultID, name).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound{Kind: "DeletedKey", Name: name}
	}
	tx.ExecContext(ctx, `UPDATE vkey SET deleted_at=NULL,scheduled_purge=NULL,recovery_id=NULL WHERE id=?`, id)
	tx.Commit()
	return r.getCurrentKeyVersion(ctx, id)
}

func (r *KeyRepo) Purge(ctx context.Context, vaultID, name string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM vkey WHERE vault_id=? AND name=? AND deleted_at IS NOT NULL`, vaultID, name)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound{Kind: "DeletedKey", Name: name}
	}
	return nil
}

func nullInt(n int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(n), Valid: n != 0}
}
