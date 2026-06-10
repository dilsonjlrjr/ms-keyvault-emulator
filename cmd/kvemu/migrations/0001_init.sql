PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS vault (
    id             TEXT PRIMARY KEY,
    dns_name       TEXT NOT NULL,
    tenant_id      TEXT NOT NULL,
    soft_delete    INTEGER NOT NULL DEFAULT 1,
    purge_protect  INTEGER NOT NULL DEFAULT 0,
    retention_days INTEGER NOT NULL DEFAULT 90,
    created        INTEGER NOT NULL
);

-- ─── SECRETS ───────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS secret (
    id             INTEGER PRIMARY KEY,
    vault_id       TEXT NOT NULL REFERENCES vault(id),
    name           TEXT NOT NULL,
    current_ver    TEXT,
    managed        INTEGER NOT NULL DEFAULT 0,
    deleted_at     INTEGER,
    scheduled_purge INTEGER,
    recovery_id    TEXT,
    recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',
    UNIQUE (vault_id, name)
);

CREATE TABLE IF NOT EXISTS secret_version (
    id            INTEGER PRIMARY KEY,
    secret_id     INTEGER NOT NULL REFERENCES secret(id) ON DELETE CASCADE,
    version       TEXT NOT NULL,
    value_enc     BLOB NOT NULL,
    value_nonce   BLOB NOT NULL,
    content_type  TEXT,
    tags_json     TEXT,
    enabled       INTEGER NOT NULL DEFAULT 1,
    nbf           INTEGER,
    exp           INTEGER,
    created       INTEGER NOT NULL,
    updated       INTEGER NOT NULL,
    UNIQUE (secret_id, version)
);

-- ─── KEYS ──────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS vkey (
    id              INTEGER PRIMARY KEY,
    vault_id        TEXT NOT NULL REFERENCES vault(id),
    name            TEXT NOT NULL,
    current_ver     TEXT,
    deleted_at      INTEGER,
    scheduled_purge INTEGER,
    recovery_id     TEXT,
    recovery_level  TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',
    UNIQUE (vault_id, name)
);

CREATE TABLE IF NOT EXISTS vkey_version (
    id              INTEGER PRIMARY KEY,
    key_id          INTEGER NOT NULL REFERENCES vkey(id) ON DELETE CASCADE,
    version         TEXT NOT NULL,
    kty             TEXT NOT NULL,
    crv             TEXT,
    key_size        INTEGER,
    key_ops_json    TEXT NOT NULL,
    jwk_pub_json    TEXT NOT NULL,
    jwk_priv_enc    BLOB,
    jwk_priv_nonce  BLOB,
    enabled         INTEGER NOT NULL DEFAULT 1,
    nbf             INTEGER,
    exp             INTEGER,
    created         INTEGER NOT NULL,
    updated         INTEGER NOT NULL,
    UNIQUE (key_id, version)
);

-- ─── CERTIFICATES ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS vcert (
    id              INTEGER PRIMARY KEY,
    vault_id        TEXT NOT NULL REFERENCES vault(id),
    name            TEXT NOT NULL,
    current_ver     TEXT,
    policy_json     TEXT NOT NULL DEFAULT '{}',
    deleted_at      INTEGER,
    scheduled_purge INTEGER,
    recovery_id     TEXT,
    recovery_level  TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',
    UNIQUE (vault_id, name)
);

CREATE TABLE IF NOT EXISTS vcert_version (
    id                  INTEGER PRIMARY KEY,
    cert_id             INTEGER NOT NULL REFERENCES vcert(id) ON DELETE CASCADE,
    version             TEXT NOT NULL,
    cer                 BLOB NOT NULL,
    x5t                 TEXT,
    x5t_s256            TEXT,
    backing_secret_ver  TEXT,
    backing_key_ver     TEXT,
    enabled             INTEGER NOT NULL DEFAULT 1,
    nbf                 INTEGER,
    exp                 INTEGER,
    created             INTEGER NOT NULL,
    updated             INTEGER NOT NULL,
    UNIQUE (cert_id, version)
);

-- ─── AUDITORIA ────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS audit_log (
    id       INTEGER PRIMARY KEY,
    ts       INTEGER NOT NULL,
    actor    TEXT,
    op       TEXT NOT NULL,
    resource TEXT,
    status   INTEGER,
    detail   TEXT
);

-- índices
CREATE INDEX IF NOT EXISTS idx_sv_secret  ON secret_version(secret_id, created DESC);
CREATE INDEX IF NOT EXISTS idx_kv_key     ON vkey_version(key_id, created DESC);
CREATE INDEX IF NOT EXISTS idx_cv_cert    ON vcert_version(cert_id, created DESC);
CREATE INDEX IF NOT EXISTS idx_s_deleted  ON secret(vault_id, deleted_at);
