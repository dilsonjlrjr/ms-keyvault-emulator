# kvemu — Contexto Completo do Projeto

## Visão Geral

**kvemu** é um emulador do Azure Key Vault (data-plane API 7.4) em Go, projetado para que aplicações Spring Boot usando os SDKs oficiais funcionem sem mudanças de código — apenas configuração + confiança no CA.

**Stack:** Go 1.24, chi router, modernc.org/sqlite (pure-Go, sem CGO), golang-jwt/v5, slog, AES-256-GCM at-rest, Docker distroless.

---

## Arquitetura

Hexagonal (Ports & Adapters):

```
backend/
├── cmd/kvemu/main.go                          # CLI: serve, healthcheck, ca export, seed
├── internal/config/config.go                   # config via env vars
├── internal/domain/                            # secret.go, key.go, certificate.go, attributes.go, identifier.go, errors.go
├── internal/ports/                             # repositories.go (SecretRepo, KeyRepo interfaces)
├── internal/app/                               # secret_service.go, key_service.go, cert_service.go, purge_scheduler.go
├── internal/adapters/http/                     # router.go, secrets_handler.go, keys_handler.go, certs_handler.go, aad_handler.go
│   └── middleware/                             # challenge.go, auth.go, errors.go, audit.go, logger.go
├── internal/adapters/crypto/                   # jwt.go, keygen.go, keyops.go, atrest.go, tls.go
├── internal/adapters/persistence/sqlite/       # db.go, secret_repo.go, key_repo.go, cert_repo.go, audit_repo.go
├── deploy/                                     # Dockerfile, docker-compose.yml, docker-compose.app.yml
├── test/e2e/                                   # suite_test.go, secrets_test.go, keys_test.go, certs_test.go, compat_test.go, matrix_test.go
├── test/compat/                                # spring27_compat_test.go, docker-compose.spring27.yml
├── Makefile                                    # build, test, verify, gate, run, docker/*
└── migrations/0001_init.sql
```

---

## Tipos de Domínio Principais

- `SecretVersion` (Name, Version, Value, ContentType, Tags, Managed, Attributes)
- `KeyVersion` (Name, Version, Kty, Crv, KeySize, KeyOps, PubJWK, Attributes)
- `CertVersion` (Name, Version, CerDER, X5T, X5TS256, BackingSecretVer, BackingKeyVer, Policy, Attributes)
- `Attributes` (Enabled, NotBefore, Expires, Created, Updated, RecoveryLevel, RecoverableDays)
- `DeletedSecret/Key/Cert` embedem a version + DeletedDate, ScheduledPurge, RecoveryID
- Errors: `ErrNotFound`, `ErrConflict`, `ErrBadParam`, `ErrForbidden`, `ErrDeleted`

---

## Ports (Interfaces)

- `SecretRepository`: Upsert, Get, GetCurrent, List, ListVersions, UpdateAttributes, SoftDelete, GetDeleted, ListDeleted, Recover, Purge, IsDeleted
- `KeyRepository`: mesmo padrão + GetPriv para material JWK privado
- CertRepo usa `*sqlite.CertRepo` concreto (não interface) em CertService

---

## Fluxo de Autenticação Challenge (doc 05)

- Exatamente UM header `WWW-Authenticate` no 401: `Bearer authorization="https://{host}/{tenant}", resource="https://{parentDomain}"`
- Usa `w.Header().Set()` nunca `Add()` para prevenir duplicação
- `BuildChallenge()` em `middleware/challenge.go` gera formato canônico
- Auth middleware: leniente (default) verifica só estrutura/exp; strict valida assinatura RS256 + aud + exp

### Validação de Domínio do SDK

O SDK Azure (`KeyVaultCredentialPolicy`) valida que o host do vault termina com `.{scopeHost}`:
```java
String host = request.getUrl().getHost();
String toEndWith = "." + scopeUri.getHost();
return host.regionMatches(true, host.length() - toEndWith.length(), toEndWith, 0, toEndWith.length());
```

**Solução aplicada:** `parentDomain()` em `challenge.go` extrai o domínio pai:
- `vault.kvemu.local:13000` → recurso `https://kvemu.local`
- `myvault.vault.azure.net` → recurso `https://vault.azure.net`
- IPs retornam como estão

---

## Endpoints AAD Fake

- `GET /{tenant}/discovery/instance` — Instance discovery (MSAL4J)
- `GET /{tenant}/v2.0/.well-known/openid-configuration` — OIDC discovery v2
- `GET /{tenant}/.well-known/openid-configuration` — OIDC discovery v1
- `GET /{tenant}/discovery/v2.0/keys` — JWKS
- `POST /{tenant}/oauth2/v2.0/token` — Token endpoint v2
- `POST /{tenant}/oauth2/token` — Token endpoint v1

Todos servidos pelo mesmo processo.

### Configuração JWT

- RS256, claims: aud, iss, appid, tid, oid, sub, ver=1.0
- Função `OIDCConfig(vaultHost, tenantID)` gera discovery JSON com URLs corretas
- **BUG CORRIGIDO:** `token_endpoint` e `jwks_uri` tinham paths errados (`/v2.0/oauth2/...` em vez de `/oauth2/v2.0/...`)

---

## Variáveis de Ambiente

| Var | Descrição |
|-----|-----------|
| `KV_ADDR` | Endereço de escuta (ex: `0.0.0.0:13000`) |
| `KV_VAULT_HOST` | Hostname do vault (ex: `vault.kvemu.local:13000`) |
| `KV_TENANT_ID` | ID do tenant AAD |
| `KV_DATA` | Caminho do banco SQLite |
| `KV_TLS_AUTO` | `true` para auto-gerar TLS |
| `KV_TLS_SAN` | SANs adicionais para o certificado TLS |
| `KV_TLS_CERT` / `KV_TLS_KEY` | Cert/key TLS explícitos |
| `KV_CA_OUT` | Caminho para exportar o CA PEM |
| `KV_CERT_DIR` | Diretório para certs gerados |
| `KV_AUTH_STRICT` | `true` para validar assinatura JWT |
| `KV_MASTER_KEY` | Chave mestra para at-rest encryption |

---

## Status do Roadmap

| Fase | Status | Descrição |
|------|--------|-----------|
| 0 | ✅ | Foundation: config, SQLite WAL, migrations, TLS auto-gen, healthz, HTTPS server |
| 1 | ✅ | Auth: canonical challenge, AAD fake, auth lenient/strict |
| 2 | ✅ | Secrets: full CRUD, soft-delete, recover, purge, backup, pagination |
| 3 | ✅ | Keys: create/import/get/list, encrypt/decrypt, sign/verify, wrapKey/unwrapKey |
| 4 | ✅ | Certificates: create self-signed, import PEM, get/list, policy, delete/recover/purge, contacts/issuers |
| 5 | Parcial | Purge scheduler, audit log, seed, healthcheck subcommand |
| 5.5 | ✅ | Docker: distroless Dockerfile, docker-compose, docker-compose.app |
| 6 | ✅ | Teste real com Spring Boot (3 versões: 2.7.9, 2.7.18, 3.4.5) |

---

## Contacts/Issuers — Implementação Persistente

Endpoints de contacts e issuers que antes retornavam 404 agora são persistidos em SQLite:

**Migração:** `migrations/0002_cert_issuers_contacts.sql`
- `vcert_contacts` (vault_id PK, contacts_json, created, updated)
- `vcert_issuer` (id, vault_id, name, provider, credentials_json, org_details_json, attributes_json, created, updated)

**Métodos adicionados:**
- `cert_repo.go`: `GetContacts`, `SetContacts`, `DeleteContacts`, `GetIssuer`, `SetIssuer`, `DeleteIssuer`, `ListIssuers`
- `cert_service.go`: mesmos métodos
- `certs_handler.go`: stubs substituídos por implementações persistentes

---

## Teste de Compatibilidade Spring Boot 2.7

### Setup

- **App Spring:** `test/compat/spring27/` — Spring Boot 2.7.18 + Spring Cloud Azure 4.5.0
- **POM:** `spring-cloud-azure-starter-keyvault-secrets` 4.5.0
- **ProbeController:** usa `@Value` para injetar `db-password` e `api-key`, expõe `/probe`
- **Dockerfile:** multi-stage Maven build, importa CA do kvemu no truststore JRE
- **entrypoint.sh:** `keytool -importcert` + `-Dazure.identity.azure-authority-host` JVM flag
- **docker-compose:** kvemu + kvemu-seed + spring27 + probe containers

### Fluxo do Teste

1. `docker compose up --build -d`
2. Aguarda spring27-compat ficar healthy (healthcheck: `curl /probe`)
3. Executa `curl /probe` e verifica `kv_connected:true`
4. Passa se e somente se secrets carregados no Spring context

### Problemas Encontrados e Correções

#### 1. ArrayIndexOutOfBoundsException no parser de challenge (RESOLVIDO)

O SDK Spring Cloud Azure 4.5.0 (azure-security-keyvault-secrets ≤4.6.7) tem parser frágil do header `WWW-Authenticate`. Teste `legacySDKParse` reproduz o crash.

**Status:** Resolvido — formato canônico do challenge evita o bug.

#### 2. Validação de domínio hierárquico do SDK (RESOLVIDO)

SDK exige que host do vault termine com `.{scopeHost}`. Hostnames planos (`kvemu-compat`) sempre falham.

**Solução:** `parentDomain()` em `challenge.go` + hostname hierárquico `vault.kvemu.local` no docker-compose.

#### 3. MSAL4J contatando Azure AD real (RESOLVIDO)

**Problema:** MSAL4J ignora challenge `authorization` e usa sua própria autoridade configurada. Por default vai para `login.microsoftonline.com`.

**Tentativas:**

| Abordagem | Resultado |
|-----------|-----------|
| `spring.cloud.azure.profile.environment.active-directory-endpoint` no application.properties | App Spring trava silenciosamente |
| `AZURE_AUTHORITY_HOST` env var com trailing slash | App Spring trava silenciosamente |
| `-Dazure.identity.azure-authority-host` JVM system property | Testado — mesmo resultado (trava) |

**Solução aplicada:** DNS redirect via Docker network aliases + TLS SAN.

1. **Docker network alias:** `login.microsoftonline.com` adicionado como alias do container kvemu
2. **TLS SAN:** `KV_TLS_SAN: "login.microsoftonline.com"` para o certificado TLS aceitar conexões para esse hostname
3. **CA importado:** entrypoint.sh importa CA do kvemu no truststore JRE

Fluxo resultante:
1. MSAL4J tenta conectar em `https://login.microsoftonline.com/{tenant}/...`
2. DNS resolve para o container kvemu (network alias)
3. TLS handshake succeeds (SAN inclui `login.microsoftonline.com`)
4. kvemu responde endpoints OIDC/token normalmente
5. OIDC discovery retorna URLs apontando para `vault.kvemu.local:13000`
6. MSAL4J usa `vault.kvemu.local:13000` para token (também resolve para kvemu)

#### 4. Metadata OIDC com paths errados (RESOLVIDO)

**Problema:** `OIDCConfig()` gerava `token_endpoint` e `jwks_uri` concatenando ao issuer (que inclui `/v2.0`), resultando em paths como `/v2.0/oauth2/v2.0/token` em vez de `/oauth2/v2.0/token`.

**Solução:** Refatorado `OIDCConfig(vaultHost, tenantID)` para construir URLs corretas:
```go
issuer := fmt.Sprintf("https://%s/%s/v2.0", vaultHost, tenantID)
tokenEP := fmt.Sprintf("https://%s/%s/oauth2/v2.0/token", vaultHost, tenantID)
jwksEP := fmt.Sprintf("https://%s/%s/discovery/v2.0/keys", vaultHost, tenantID)
```

### Último Resultado do Teste

**Solução aplicada:** DNS redirect via Docker network aliases + TLS SAN.

Configuração docker-compose:
- `kvemu` service com network alias `login.microsoftonline.com`
- `KV_TLS_SAN: "login.microsoftonline.com"` para certificado TLS
- CA importado no truststore JRE via entrypoint.sh

Fluxo testado:
1. Spring Boot inicia e envia GET /secrets → 401 (challenge)
2. MSAL4J resolve `login.microsoftonline.com` via DNS → container kvemu
3. TLS handshake succeeds (SAN inclui hostname)
4. OIDC discovery + token endpoint funcionam normalmente
5. Spring Boot carrega secrets com sucesso

---

## Teste de Compatibilidade Spring Boot 3.4

### Setup

- **App Spring:** `test/compat/spring3/` — Spring Boot 3.4.5 + Spring Cloud Azure 5.21.0
- **POM:** `spring-cloud-azure-starter-keyvault-secrets` 5.21.0
- **Java:** Eclipse Temurin 21 (Jakarta namespace)
- **ProbeController:** mesmo código do spring27 (compatível com Spring Boot 3.x)
- **Dockerfile:** multi-stage Maven build, Java 21 JRE, importa CA do kvemu
- **entrypoint.sh:** idêntico ao spring27

### Fluxo do Teste

1. `docker compose up --build -d`
2. Aguarda spring3-compat ficar healthy (healthcheck: `curl /probe`)
3. Executa `curl /probe` e verifica `kv_connected:true`
4. Passa se 3/3 secrets carregados no Spring context

### Resultado

✅ **COMPAT OK** — Spring Boot 2.7.18 + Spring Cloud Azure 4.5.0 (22s)
- Challenge parseado sem ArrayIndexOutOfBoundsException
- Instance Discovery → OIDC → Token obtidos do AAD emulado
- 4/4 secrets (db-password, api-key, connection-string, COSMO_DB_URL) carregados via property-sources
- Property resolution `app.cosmos.url=${COSMO_DB_URL}` validada

---

## Teste de Compatibilidade Spring Boot 2.7.9

### Setup

- **App Spring:** `test/compat/spring279/` — Spring Boot 2.7.9 + Spring Cloud Azure 4.5.0
- **POM:** `spring-cloud-azure-starter-keyvault-secrets` 4.5.0
- **Java:** Eclipse Temurin 17
- **ProbeController:** mesmo código do spring27

### Resultado

✅ **COMPAT OK** — Spring Boot 2.7.9 + Spring Cloud Azure 4.5.0 (24s)
- Mesma configuração do spring27, parser legado funciona
- 4/4 secrets carregados via property-sources

---

## Teste de Compatibilidade Spring Boot 3.4

### Setup

- **App Spring:** `test/compat/spring3/` — Spring Boot 3.4.5 + Spring Cloud Azure 5.21.0
- **POM:** `spring-cloud-azure-starter-keyvault-secrets` 5.21.0
- **Java:** Eclipse Temurin 21 (Jakarta namespace)
- **ProbeController:** mesmo código do spring27 (compatível com Spring Boot 3.x)
- **Dockerfile:** multi-stage Maven build, Java 21 JRE, importa CA do kvemu
- **entrypoint.sh:** idêntico ao spring27

### Fluxo do Teste

1. `docker compose up --build -d`
2. Aguarda spring3-compat ficar healthy (healthcheck: `curl /probe`)
3. Executa `curl /probe` e verifica `kv_connected:true`
4. Passa se 4/4 secrets carregados no Spring context

### Resultado

✅ **COMPAT OK** — Spring Boot 3.4 + Spring Cloud Azure 5.x (21s)
- Challenge parseado sem erros
- Instance Discovery → OIDC → Token obtidos do AAD emulado
- 4/4 secrets (db-password, api-key, connection-string, COSMO_DB_URL) carregados via property-sources
- **Sem problemas encontrados** — SDK Azure 5.x funciona nativamente com o emulador

---

## Property Resolution: `app.cosmos.url=${COSMO_DB_URL}`

O secret `COSMO_DB_URL` (`https://cosmos.example.com`) é injetado via seed. No `application.properties` dos testes:

```properties
app.cosmos.url=${COSMO_DB_URL}
```

O Spring Cloud Azure resolve `${COSMO_DB_URL}` do Key Vault emulado → `app.cosmos.url` = `https://cosmos.example.com`.

Validado via `CommandLineRunner` (Application.java imprime o valor no startup) + `ProbeController` (expõe `cosmos_db_url` no `/probe`). Testes Go verificam `"cosmos_db_url":"https://cosmos.example.com"` na resposta.

---

## Análise de Compatibilidade

| Componente | Status | Risco |
|-----------|--------|-------|
| Auth challenge | ✅ Testado | Zero |
| AAD fake (OIDC + Instance Discovery) | ✅ Testado | Zero |
| Secrets CRUD | ✅ Testado | Zero |
| Property resolution (`${SECRET_NAME}`) | ✅ Testado | Zero |
| Keys CRUD + crypto | ✅ Implementado | Baixo |
| Certificates CRUD | ✅ Implementado | Baixo |
| Cert backing PEM vs PFX | Só PEM | Alto |
| @AzureKeyVaultSecretValue polling | Não implementado | Médio |
| ManagedIdentityCredential (IMDS) | Não implementado | N/A |
| Spring Boot 2.7.9 + Azure 4.5.0 | ✅ Testado | Zero |
| Spring Boot 2.7.18 + Azure 4.5.0 | ✅ Testado | Zero |
| Spring Boot 3.4 + Azure 5.x | ✅ Testado | Zero |
| MSAL4J authority host | ✅ Resolvido (DNS redirect) | Zero |

**Verdito atual:** 100% para caso de uso primário Spring Boot com @Value + secrets.

---

## Arquivos Modificados (sessão atual)

| Arquivo | Mudança |
|---------|---------|
| `internal/adapters/http/middleware/challenge.go` | Adicionado `parentDomain()`, resource dinâmico baseado em domínio pai |
| `internal/adapters/http/middleware/challenge_test.go` | Golden strings atualizados |
| `internal/adapters/crypto/jwt.go` | `IssueToken` e `ValidateToken` com param `audience`; `OIDCConfig(vaultHost, tenantID)` |
| `internal/adapters/http/aad_handler.go` | Passa `vaultHost, tenant` para `OIDCConfig` |
| `internal/adapters/http/middleware/auth.go` | `Auth(aadKey, strict, vaultHost)` com audience dinâmico |
| `internal/adapters/http/router.go` | Passa `cfg.VaultHost` para `middleware.Auth()` |
| `test/e2e/suite_test.go` | `acquireToken` usa scope dinâmico baseado em vault host |
| `test/compat/docker-compose.spring27.yml` | `vault.kvemu.local` alias, `AZURE_AUTHORITY_HOST`, TLS SAN |
| `test/compat/spring27/entrypoint.sh` | `-Dazure.identity.azure-authority-host` JVM flag |
| `test/compat/spring27/src/main/resources/application.properties` | `active-directory-endpoint`, DEBUG logging |
| `migrations/0002_cert_issuers_contacts.sql` | Nova migração para contacts/issuers |
| `internal/adapters/persistence/sqlite/cert_repo.go` | Métodos contacts/issuers |
| `internal/app/cert_service.go` | Métodos contacts/issuers |
| `internal/adapters/http/certs_handler.go` | Implementações contacts/issuers |

---

## Comandos Úteis

```bash
# Build
go build ./...

# Unit tests
go test ./... -count=1 -race

# E2E tests
go test -tags=e2e ./test/e2e/... -v -timeout=5m

# Compat test Spring Boot 2.7
make compat/spring27

# Compat test Spring Boot 2.7.9
make compat/spring279

# Compat test Spring Boot 3.4
make compat/spring3

# Run emulator
make run

# Docker build
make docker/build
```
