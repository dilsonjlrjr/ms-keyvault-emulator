# API Endpoints

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

### AAD Identity (fake OIDC provider)

All served by the same process, no external identity provider needed.

| Method | Path | Description |
|---|---|---|
| `GET` | `/{tenant}/discovery/instance` | Instance discovery (MSAL4J) |
| `GET` | `/{tenant}/v2.0/.well-known/openid-configuration` | OIDC discovery v2 |
| `GET` | `/{tenant}/.well-known/openid-configuration` | OIDC discovery v1 |
| `GET` | `/{tenant}/discovery/v2.0/keys` | JWKS endpoint |
| `POST` | `/{tenant}/oauth2/v2.0/token` | Token endpoint v2 |
| `POST` | `/{tenant}/oauth2/token` | Token endpoint v1 |

### Secrets

| Method | Path | Description |
|---|---|---|
| `PUT` | `/secrets/{name}` | Set secret |
| `GET` | `/secrets/{name}` | Get latest version |
| `GET` | `/secrets/{name}/{version}` | Get specific version |
| `GET` | `/secrets` | List secrets |
| `GET` | `/secrets/{name}/versions` | List versions |
| `PATCH` | `/secrets/{name}/{version}` | Update attributes |
| `DELETE` | `/secrets/{name}` | Soft-delete |
| `GET` | `/deletedsecrets/{name}` | Get deleted |
| `GET` | `/deletedsecrets` | List deleted |
| `POST` | `/deletedsecrets/{name}/recover` | Recover |
| `DELETE` | `/deletedsecrets/{name}` | Purge |
| `POST` | `/secrets/{name}/backup` | Backup |
| `POST` | `/secrets/restore` | Restore |
| `GET` | `/healthz` | Health check (no auth) |

### Keys

| Method | Path | Description |
|---|---|---|
| `POST` | `/keys/{name}/create` | Create key |
| `PUT` | `/keys/{name}` | Import key |
| `GET` | `/keys/{name}` | Get latest version |
| `GET` | `/keys/{name}/{version}` | Get specific version |
| `GET` | `/keys` | List keys |
| `GET` | `/keys/{name}/versions` | List versions |
| `PATCH` | `/keys/{name}/{version}` | Update attributes |
| `DELETE` | `/keys/{name}` | Soft-delete |
| `POST` | `/keys/{name}/{version}/encrypt` | Encrypt |
| `POST` | `/keys/{name}/{version}/decrypt` | Decrypt |
| `POST` | `/keys/{name}/{version}/sign` | Sign |
| `POST` | `/keys/{name}/{version}/verify` | Verify |
| `POST` | `/keys/{name}/{version}/wrapkey` | Wrap key |
| `POST` | `/keys/{name}/{version}/unwrapkey` | Unwrap key |

### Certificates

| Method | Path | Description |
|---|---|---|
| `POST` | `/certificates/{name}/create` | Create self-signed |
| `PUT` | `/certificates/{name}/import` | Import PEM |
| `GET` | `/certificates/{name}` | Get latest |
| `GET` | `/certificates/{name}/{version}` | Get version |
| `GET` | `/certificates` | List |
| `GET` | `/certificates/{name}/versions` | List versions |
| `GET` | `/certificates/{name}/policy` | Get policy |
| `PATCH` | `/certificates/{name}/policy` | Update policy |
| `DELETE` | `/certificates/{name}` | Soft-delete |
| `GET` | `/certificates/contacts` | Get contacts |
| `PUT` | `/certificates/contacts` | Set contacts |
| `DELETE` | `/certificates/contacts` | Delete contacts |
| `GET` | `/certificates/issuers/{name}` | Get issuer |
| `PUT` | `/certificates/issuers/{name}` | Set issuer |
| `DELETE` | `/certificates/issuers/{name}` | Delete issuer |

### Vault Management (emulator-specific, no auth)

| Method | Path | Description |
|---|---|---|
| `GET` | `/vaults` | List vaults |
| `POST` | `/vaults` | Create vault |
| `GET` | `/vaults/{name}` | Get vault details |
| `DELETE` | `/vaults/{name}` | Delete vault (cascade) |
| `GET` | `/vaults/{name}/export` | Export vault as JSON |
| `POST` | `/vaults/import` | Import vault from JSON |

---

<a id="português"></a>
## 🇧🇷 Português

### Identidade AAD (provedor OIDC fake)

Todos servidos pelo mesmo processo, sem dependência externa de identidade.

| Método | Caminho | Descrição |
|---|---|---|
| `GET` | `/{tenant}/discovery/instance` | Descoberta de instância (MSAL4J) |
| `GET` | `/{tenant}/v2.0/.well-known/openid-configuration` | Descoberta OIDC v2 |
| `GET` | `/{tenant}/.well-known/openid-configuration` | Descoberta OIDC v1 |
| `GET` | `/{tenant}/discovery/v2.0/keys` | Endpoint JWKS |
| `POST` | `/{tenant}/oauth2/v2.0/token` | Endpoint de token v2 |
| `POST` | `/{tenant}/oauth2/token` | Endpoint de token v1 |

### Secrets

| Método | Caminho | Descrição |
|---|---|---|
| `PUT` | `/secrets/{name}` | Criar/atualizar secret |
| `GET` | `/secrets/{name}` | Obter versão mais recente |
| `GET` | `/secrets/{name}/{version}` | Obter versão específica |
| `GET` | `/secrets` | Listar secrets |
| `GET` | `/secrets/{name}/versions` | Listar versões |
| `PATCH` | `/secrets/{name}/{version}` | Atualizar atributos |
| `DELETE` | `/secrets/{name}` | Soft-delete |
| `GET` | `/deletedsecrets/{name}` | Obter deletado |
| `GET` | `/deletedsecrets` | Listar deletados |
| `POST` | `/deletedsecrets/{name}/recover` | Recuperar |
| `DELETE` | `/deletedsecrets/{name}` | Expurgar |
| `POST` | `/secrets/{name}/backup` | Backup |
| `POST` | `/secrets/restore` | Restaurar |
| `GET` | `/healthz` | Health check (sem auth) |

### Keys

| Método | Caminho | Descrição |
|---|---|---|
| `POST` | `/keys/{name}/create` | Criar chave |
| `PUT` | `/keys/{name}` | Importar chave |
| `GET` | `/keys/{name}` | Obter versão mais recente |
| `GET` | `/keys/{name}/{version}` | Obter versão específica |
| `GET` | `/keys` | Listar chaves |
| `GET` | `/keys/{name}/versions` | Listar versões |
| `PATCH` | `/keys/{name}/{version}` | Atualizar atributos |
| `DELETE` | `/keys/{name}` | Soft-delete |
| `POST` | `/keys/{name}/{version}/encrypt` | Cifrar |
| `POST` | `/keys/{name}/{version}/decrypt` | Decifrar |
| `POST` | `/keys/{name}/{version}/sign` | Assinar |
| `POST` | `/keys/{name}/{version}/verify` | Verificar assinatura |
| `POST` | `/keys/{name}/{version}/wrapkey` | Envelopar chave |
| `POST` | `/keys/{name}/{version}/unwrapkey` | Desenvelopar chave |

### Certificates

| Método | Caminho | Descrição |
|---|---|---|
| `POST` | `/certificates/{name}/create` | Criar auto-assinado |
| `PUT` | `/certificates/{name}/import` | Importar PEM |
| `GET` | `/certificates/{name}` | Obter mais recente |
| `GET` | `/certificates/{name}/{version}` | Obter versão |
| `GET` | `/certificates` | Listar |
| `GET` | `/certificates/{name}/versions` | Listar versões |
| `GET` | `/certificates/{name}/policy` | Obter política |
| `PATCH` | `/certificates/{name}/policy` | Atualizar política |
| `DELETE` | `/certificates/{name}` | Soft-delete |
| `GET` | `/certificates/contacts` | Obter contatos |
| `PUT` | `/certificates/contacts` | Definir contatos |
| `DELETE` | `/certificates/contacts` | Remover contatos |
| `GET` | `/certificates/issuers/{name}` | Obter emissor |
| `PUT` | `/certificates/issuers/{name}` | Definir emissor |
| `DELETE` | `/certificates/issuers/{name}` | Remover emissor |

### Gestão de Vaults (específico do emulador, sem auth)

| Método | Caminho | Descrição |
|---|---|---|
| `GET` | `/vaults` | Listar vaults |
| `POST` | `/vaults` | Criar vault |
| `GET` | `/vaults/{name}` | Detalhes do vault |
| `DELETE` | `/vaults/{name}` | Deletar vault (cascata) |
| `GET` | `/vaults/{name}/export` | Exportar vault como JSON |
| `POST` | `/vaults/import` | Importar vault de JSON |
