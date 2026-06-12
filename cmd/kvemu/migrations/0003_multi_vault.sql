-- FK enforcement blocks DROP of tables referenced by other FKs.
-- Disable during schema transformation, re-enable at the end.
PRAGMA foreign_keys = OFF;

-- ── Step 1: Nova tabela vault com PK=(name), UNIQUE=(host) ─────────────────
CREATE TABLE IF NOT EXISTS vault_v2 (
    name         TEXT NOT NULL PRIMARY KEY,
    host         TEXT NOT NULL UNIQUE,
    dns_name     TEXT NOT NULL,
    tenant_id    TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    created      INTEGER NOT NULL,
    updated      INTEGER NOT NULL
);

-- ── Step 2: Migrar dados do vault antigo ──────────────────────────────────
INSERT OR IGNORE INTO vault_v2 (name, host, dns_name, tenant_id, display_name, created, updated)
SELECT
    CASE
        WHEN instr(id, '.') > 0 THEN substr(id, 1, instr(id, '.') - 1)
        ELSE id
    END,
    id,
    dns_name,
    tenant_id,
    'Default Vault',
    created,
    created
FROM vault;

-- ── Step 3: Recriar SECRET com FK → vault_v2(name) ────────────────────────
CREATE TABLE IF NOT EXISTS secret_new (
    id             INTEGER PRIMARY KEY,
    vault_id       TEXT NOT NULL REFERENCES vault_v2(name),
    name           TEXT NOT NULL,
    current_ver    TEXT,
    managed        INTEGER NOT NULL DEFAULT 0,
    deleted_at     INTEGER,
    scheduled_purge INTEGER,
    recovery_id    TEXT,
    recovery_level TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',
    UNIQUE (vault_id, name)
);

INSERT INTO secret_new
SELECT s.id,
    COALESCE((SELECT v2.name FROM vault_v2 v2 WHERE v2.host = s.vault_id), s.vault_id),
    s.name, s.current_ver, s.managed, s.deleted_at,
    s.scheduled_purge, s.recovery_id, s.recovery_level
FROM secret s;

DROP TABLE secret;
ALTER TABLE secret_new RENAME TO secret;

-- ── Step 4: Recriar VKEY com FK → vault_v2(name) ─────────────────────────
CREATE TABLE IF NOT EXISTS vkey_new (
    id              INTEGER PRIMARY KEY,
    vault_id        TEXT NOT NULL REFERENCES vault_v2(name),
    name            TEXT NOT NULL,
    current_ver     TEXT,
    deleted_at      INTEGER,
    scheduled_purge INTEGER,
    recovery_id     TEXT,
    recovery_level  TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',
    UNIQUE (vault_id, name)
);

INSERT INTO vkey_new
SELECT k.id,
    COALESCE((SELECT v2.name FROM vault_v2 v2 WHERE v2.host = k.vault_id), k.vault_id),
    k.name, k.current_ver, k.deleted_at,
    k.scheduled_purge, k.recovery_id, k.recovery_level
FROM vkey k;

DROP TABLE vkey;
ALTER TABLE vkey_new RENAME TO vkey;

-- ── Step 5: Recriar VCERT com FK → vault_v2(name) ────────────────────────
CREATE TABLE IF NOT EXISTS vcert_new (
    id              INTEGER PRIMARY KEY,
    vault_id        TEXT NOT NULL REFERENCES vault_v2(name),
    name            TEXT NOT NULL,
    current_ver     TEXT,
    policy_json     TEXT NOT NULL DEFAULT '{}',
    deleted_at      INTEGER,
    scheduled_purge INTEGER,
    recovery_id     TEXT,
    recovery_level  TEXT NOT NULL DEFAULT 'Recoverable+Purgeable',
    UNIQUE (vault_id, name)
);

INSERT INTO vcert_new
SELECT c.id,
    COALESCE((SELECT v2.name FROM vault_v2 v2 WHERE v2.host = c.vault_id), c.vault_id),
    c.name, c.current_ver, c.policy_json, c.deleted_at,
    c.scheduled_purge, c.recovery_id, c.recovery_level
FROM vcert c;

DROP TABLE vcert;
ALTER TABLE vcert_new RENAME TO vcert;

-- ── Step 6: Recriar VCERT_CONTACTS ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS vcert_contacts_new (
    vault_id      TEXT NOT NULL PRIMARY KEY REFERENCES vault_v2(name),
    contacts_json TEXT NOT NULL DEFAULT '{}',
    created       INTEGER NOT NULL,
    updated       INTEGER NOT NULL
);

INSERT INTO vcert_contacts_new
SELECT
    COALESCE((SELECT v2.name FROM vault_v2 v2 WHERE v2.host = vc.vault_id), vc.vault_id),
    vc.contacts_json, vc.created, vc.updated
FROM vcert_contacts vc;

DROP TABLE vcert_contacts;
ALTER TABLE vcert_contacts_new RENAME TO vcert_contacts;

-- ── Step 7: Recriar VCERT_ISSUER ──────────────────────────────────────────
CREATE TABLE IF NOT EXISTS vcert_issuer_new (
    id               INTEGER PRIMARY KEY,
    vault_id         TEXT NOT NULL REFERENCES vault_v2(name),
    name             TEXT NOT NULL,
    provider         TEXT NOT NULL DEFAULT '',
    credentials_json TEXT NOT NULL DEFAULT '{}',
    org_details_json TEXT NOT NULL DEFAULT '{}',
    attributes_json  TEXT NOT NULL DEFAULT '{}',
    created          INTEGER NOT NULL,
    updated          INTEGER NOT NULL,
    UNIQUE (vault_id, name)
);

INSERT INTO vcert_issuer_new
SELECT i.id,
    COALESCE((SELECT v2.name FROM vault_v2 v2 WHERE v2.host = i.vault_id), i.vault_id),
    i.name, i.provider, i.credentials_json, i.org_details_json,
    i.attributes_json, i.created, i.updated
FROM vcert_issuer i;

DROP TABLE vcert_issuer;
ALTER TABLE vcert_issuer_new RENAME TO vcert_issuer;

-- ── Step 8: Substituir vault antigo ──────────────────────────────────────
DROP TABLE vault;
ALTER TABLE vault_v2 RENAME TO vault;

-- ── Step 9: Recriar índices ──────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_vault_host ON vault(host);
CREATE INDEX IF NOT EXISTS idx_s_deleted ON secret(vault_id, deleted_at);

-- ── Re-enable FK enforcement ─────────────────────────────────────────────
PRAGMA foreign_keys = ON;
