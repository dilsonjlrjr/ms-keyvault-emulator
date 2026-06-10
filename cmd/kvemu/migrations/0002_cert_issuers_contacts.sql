-- ─── CERTIFICATE CONTACTS ─────────────────────────────────
CREATE TABLE IF NOT EXISTS vcert_contacts (
    vault_id    TEXT PRIMARY KEY REFERENCES vault(id),
    contacts_json TEXT NOT NULL DEFAULT '[]',
    created     INTEGER NOT NULL,
    updated     INTEGER NOT NULL
);

-- ─── CERTIFICATE ISSUERS ──────────────────────────────────
CREATE TABLE IF NOT EXISTS vcert_issuer (
    id          INTEGER PRIMARY KEY,
    vault_id    TEXT NOT NULL REFERENCES vault(id),
    name        TEXT NOT NULL,
    provider    TEXT NOT NULL DEFAULT '',
    credentials_json TEXT NOT NULL DEFAULT '{}',
    org_details_json TEXT NOT NULL DEFAULT '{}',
    attributes_json  TEXT NOT NULL DEFAULT '{}',
    created     INTEGER NOT NULL,
    updated     INTEGER NOT NULL,
    UNIQUE (vault_id, name)
);
