# kvemu — Azure Key Vault Emulator

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

### About

kvemu is a **zero-dependency Azure Key Vault emulator** that implements the data-plane API 7.4. Applications using the official Azure SDKs (Java, .NET, Python, Go, etc.) work without code changes — only endpoint + credentials configuration.

**Stack:** Go 1.24 · chi router · SQLite (pure-Go, no CGO) · golang-jwt/v5 · AES-256-GCM at-rest · Docker distroless (~8 MB image)

---

### Quick Start (Docker)

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Emulator starts on `https://localhost:13000` with auto-generated TLS and a fake AAD identity provider.

---

### CLI Commands

```bash
kvemu                  # run server (default)
kvemu healthcheck      # check if server is healthy
kvemu ca export        # export CA certificate (PEM)
kvemu seed             # populate dev secrets/keys into running emulator
```

#### Healthcheck

```bash
kvemu healthcheck --addr https://localhost:13000
```

Uses loopback HTTPS with `InsecureSkipVerify`. Returns exit code 0 if `/healthz` responds 200.

#### CA Export

```bash
kvemu ca export --out ca.pem
```

Exports the emulator's CA certificate. Import this file into your application's truststore to avoid TLS verification errors.

#### Seed

```bash
docker compose -f deploy/docker-compose.yml exec kvemu /kvemu seed
```

Populates development secrets and keys:

| Secret | Value |
|---|---|
| `db-password` | `sup3rS3cr3t!` |
| `api-key` | `sk-dev-1234567890abcdef` |
| `connection-string` | `Server=localhost;Database=devdb;User=sa;Password=dev!` |
| `COSMO_DB_URL` | `https://cosmos.example.com` |

| Key | Type | Size |
|---|---|---|
| `rsa-key-2048` | RSA | 2048-bit |
| `rsa-key-4096` | RSA | 4096-bit |

---

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `KV_ADDR` | `0.0.0.0:13000` | Listen address |
| `KV_VAULT_HOST` | `localhost:13000` | Vault host:port (used in challenge, IDs, SANs) |
| `KV_TENANT_ID` | `a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f` | AAD tenant ID |
| `KV_DATA` | `./data/kv.db` | SQLite database path |
| `KV_TLS_AUTO` | `true` | Auto-generate CA + leaf certificate on boot |
| `KV_TLS_SAN` | _(empty)_ | Extra SANs (comma-separated) for TLS certificate |
| `KV_TLS_CERT` | _(empty)_ | Bring-your-own TLS certificate (PEM path) |
| `KV_TLS_KEY` | _(empty)_ | Bring-your-own TLS private key (PEM path) |
| `KV_CA_OUT` | `./certs/ca.pem` | Path to export CA PEM |
| `KV_CERT_DIR` | `./certs` | Certificate storage directory |
| `KV_AUTH_STRICT` | `false` | Validate JWT signature (RS256) instead of lenient |
| `KV_MASTER_KEY` | _(dev key)_ | Master key for AES-256-GCM at-rest encryption |

---

### Docker Compose

#### Production-style deployment

```yaml
# deploy/docker-compose.yml
services:
  kvemu:
    build:
      context: ..
      dockerfile: deploy/Dockerfile
    image: ghcr.io/dilsonrabelo/kvemu:latest
    container_name: kvemu
    restart: unless-stopped
    ports:
      - "${KV_PORT:-13000}:13000"
    environment:
      KV_ADDR:         "0.0.0.0:13000"
      KV_VAULT_HOST:   "${KV_VAULT_HOST:-localhost:13000}"
      KV_TENANT_ID:    "${KV_TENANT_ID:-a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f}"
      KV_TLS_AUTO:     "true"
      KV_AUTH_STRICT:  "${KV_AUTH_STRICT:-false}"
      KV_MASTER_KEY:   "${KV_MASTER_KEY:-}"
      KV_DATA:         "/data/kv.db"
      KV_CERT_DIR:     "/certs"
      KV_CA_OUT:       "/certs/ca.pem"
    volumes:
      - kvemu-data:/data
      - kvemu-certs:/certs
    healthcheck:
      test: ["CMD", "/kvemu", "healthcheck", "--addr", "https://localhost:13000"]

volumes:
  kvemu-data:
  kvemu-certs:
```

#### App + emulator side-by-side

```yaml
# deploy/docker-compose.app.yml
services:
  kvemu:
    build:
      context: .
      dockerfile: deploy/Dockerfile
    ports:
      - "13000:13000"
    environment:
      KV_ADDR:         "0.0.0.0:13000"
      KV_VAULT_HOST:   "vault.kvemu.local:13000"
      KV_TENANT_ID:    "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
      KV_TLS_AUTO:     "true"
      KV_TLS_SAN:      "login.microsoftonline.com"
      KV_AUTH_STRICT:  "false"
    volumes:
      - data:/data
      - certs:/certs

  kvemu-seed:
    build:
      context: .
      dockerfile: deploy/Dockerfile
    command: ["seed"]
    environment:
      KV_DATA:       "/data/kv.db"
      KV_VAULT_HOST: "vault.kvemu.local:13000"
      KV_TENANT_ID:  "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
    volumes:
      - data:/data
    depends_on:
      kvemu:
        condition: service_healthy

  myapp:
    image: myapp:latest
    environment:
      AZURE_KEYVAULT_ENDPOINT: "https://vault.kvemu.local:13000"
      AZURE_CLIENT_ID:         "dev-client"
      AZURE_CLIENT_SECRET:     "dev-secret"
      AZURE_TENANT_ID:         "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
    volumes:
      - certs:/certs:ro

volumes:
  data:
  certs:
```

---

### TLS & Certificates

#### Auto-generated TLS (default)

Set `KV_TLS_AUTO=true` (default). On first boot, kvemu generates:

- **CA certificate**: `{KV_CERT_DIR}/ca.pem`
- **Leaf certificate**: `{KV_CERT_DIR}/leaf.pem`

The leaf certificate SANs include `KV_VAULT_HOST`, `localhost`, `kvemu`, and any hosts from `KV_TLS_SAN`. On subsequent boots, existing certificates are reused.

#### Bring-your-own certificate

```bash
export KV_TLS_CERT=/path/to/server.pem
export KV_TLS_KEY=/path/to/server-key.pem
export KV_TLS_AUTO=false
```

#### Importing CA into client truststore

**Java (JRE):**
```bash
keytool -importcert -noprompt \
  -alias kvemu-ca \
  -file ca.pem \
  -keystore $JAVA_HOME/lib/security/cacerts \
  -storepass changeit
```

**Go:**
```go
caPEM, _ := os.ReadFile("ca.pem")
pool := x509.NewCertPool()
pool.AppendCertsFromPEM(caPEM)
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{RootCAs: pool},
    },
}
```

**curl:**
```bash
curl --cacert ca.pem https://localhost:13000/healthz
```

---

### API Endpoints

#### AAD Identity (fake OIDC provider)

All served by the same process, no external identity provider needed.

| Method | Path | Description |
|---|---|---|
| `GET` | `/{tenant}/discovery/instance` | Instance discovery (MSAL4J) |
| `GET` | `/{tenant}/v2.0/.well-known/openid-configuration` | OIDC discovery v2 |
| `GET` | `/{tenant}/.well-known/openid-configuration` | OIDC discovery v1 |
| `GET` | `/{tenant}/discovery/v2.0/keys` | JWKS endpoint |
| `POST` | `/{tenant}/oauth2/v2.0/token` | Token endpoint v2 |
| `POST` | `/{tenant}/oauth2/token` | Token endpoint v1 |

#### Secrets

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

#### Keys

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

#### Certificates

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

---

### Authentication Flow

```
App                  kvemu
 |                     |
 |-- GET /secrets ---->|  (no Authorization header)
 |<---- 401 -----------|  WWW-Authenticate: Bearer authorization="https://host/tenant", resource="https://parent.domain"
 |                     |
 |-- GET /tenant/discovery/instance -->|  (MSAL4J instance discovery)
 |<---- 200 (JSON) ----|
 |                     |
 |-- GET /tenant/v2.0/.well-known/openid-configuration -->|
 |<---- 200 (OIDC) ----|
 |                     |
 |-- POST /tenant/oauth2/v2.0/token -->|  (client_credentials)
 |<---- 200 (JWT) -----|
 |                     |
 |-- GET /secrets ----->|  (Authorization: Bearer <jwt>)
 |<---- 200 (JSON) ----|
```

1. App sends a request without auth → **401** with `WWW-Authenticate` header
2. SDK extracts `authorization` URL and performs OIDC discovery
3. SDK obtains JWT token from the fake AAD token endpoint
4. App retries with the `Authorization: Bearer <token>` header

**Lenient mode** (default): only validates JWT structure and expiration.

**Strict mode** (`KV_AUTH_STRICT=true`): validates RS256 signature against the AAD public key.

---

### Integration with Java (Spring Boot)

#### Configuration

```properties
# application.properties
spring.cloud.azure.keyvault.secret.property-sources[0].name=vault-secrets
spring.cloud.azure.keyvault.secret.property-sources[0].endpoint=${AZURE_KEYVAULT_ENDPOINT}
spring.cloud.azure.keyvault.secret.property-sources[0].refresh-interval=30m

spring.cloud.azure.credential.client-id=${AZURE_CLIENT_ID:dev-client}
spring.cloud.azure.credential.client-secret=${AZURE_CLIENT_SECRET:dev-secret}
spring.cloud.azure.profile.tenant-id=${AZURE_TENANT_ID}

spring.cloud.azure.profile.environment.active-directory-endpoint=${AZURE_AUTHORITY_HOST:https://login.microsoftonline.com/}

# Property resolution: maps Key Vault secrets to Spring properties
app.cosmos.url=${COSMO_DB_URL}
```

#### Using secrets in code

```java
@RestController
public class MyController {

    @Value("${db-password}")
    private String dbPassword;

    @Value("${api-key}")
    private String apiKey;

    @Value("${connection-string}")
    private String connectionString;

    @Value("${app.cosmos.url}")
    private String cosmosUrl;   // resolved from COSMO_DB_URL secret
}
```

#### Property resolution via CommandLineRunner

```java
@SpringBootApplication
public class Application implements CommandLineRunner {

    private static final Logger log = LoggerFactory.getLogger(Application.class);

    @Value("${app.cosmos.url:NOT_FOUND}")
    private String cosmosUrl;

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @Override
    public void run(String... args) {
        log.info("app.cosmos.url = {}", cosmosUrl);
        // app.cosmos.url resolves from Key Vault secret COSMO_DB_URL
    }
}
```

#### Importing CA in entrypoint

```bash
#!/bin/sh
keytool -importcert -noprompt \
  -alias kvemu-ca \
  -file /certs/ca.pem \
  -keystore $JAVA_HOME/lib/security/cacerts \
  -storepass changeit

exec java -jar /app/app.jar
```

#### Docker Compose with Spring Boot

```yaml
services:
  myapp:
    build: .
    environment:
      AZURE_KEYVAULT_ENDPOINT: "https://vault.kvemu.local:13000"
      AZURE_CLIENT_ID:         "dev-client"
      AZURE_CLIENT_SECRET:     "dev-secret"
      AZURE_TENANT_ID:         "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
      AZURE_AUTHORITY_HOST:    "https://vault.kvemu.local:13000/"
    volumes:
      - certs:/certs:ro
```

#### Verified compatibility

| Spring Boot | Spring Cloud Azure | Status |
|---|---|---|
| 2.7.9 | 4.5.0 | ✅ |
| 2.7.18 | 4.5.0 | ✅ |
| 3.4.5 | 5.21.0 | ✅ |

---

### Integration with Go

#### HTTP client with CA trust

```go
package main

import (
    "bytes"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
)

func main() {
    baseURL := "https://localhost:13000"
    vaultHost := "localhost:13000"
    tenantID := "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
    caPath := "./certs/ca.pem"

    // ── 1. Load CA and create HTTP client ────────────────────────────
    caPEM, _ := os.ReadFile(caPath)
    pool := x509.NewCertPool()
    pool.AppendCertsFromPEM(caPEM)

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{RootCAs: pool},
        },
    }

    // ── 2. Obtain token from AAD endpoint ────────────────────────────
    scope := fmt.Sprintf("https://%s/.default", vaultHost)
    tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", baseURL, tenantID)
    resp, _ := client.PostForm(tokenURL, url.Values{
        "grant_type":    {"client_credentials"},
        "client_id":     {"dev-client"},
        "client_secret": {"dev-secret"},
        "scope":         {scope},
    })
    var result struct {
        AccessToken string `json:"access_token"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    resp.Body.Close()

    // ── 3. Make authenticated request ────────────────────────────────
    req, _ := http.NewRequest("GET",
        baseURL+"/secrets/db-password?api-version=7.4", nil)
    req.Header.Set("Authorization", "Bearer "+result.AccessToken)

    resp, _ = client.Do(req)
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()
    fmt.Println(string(body))
}
```

#### Set a secret

```go
body := map[string]any{
    "value": "my-secret-value",
}
b, _ := json.Marshal(body)

req, _ := http.NewRequest("PUT",
    baseURL+"/secrets/my-secret?api-version=7.4",
    bytes.NewReader(b))
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

#### Encrypt / Decrypt with a key

```go
// Encrypt
encBody := map[string]any{
    "alg":   "RSA-OAEP-256",
    "value": "SGVsbG8gV29ybGQ=", // base64("Hello World")
}
b, _ := json.Marshal(encBody)
req, _ := http.NewRequest("POST",
    fmt.Sprintf("%s/keys/rsa-key-2048/%s/encrypt?api-version=7.4", baseURL, version),
    bytes.NewReader(b))
// ... same auth headers ...
resp, _ := client.Do(req)

// Decrypt
decBody := map[string]any{
    "alg":   "RSA-OAEP-256",
    "value": encryptedValue, // from encrypt response
}
// POST /keys/{name}/{version}/decrypt
```

#### List secrets with pagination

```go
req, _ := http.NewRequest("GET",
    baseURL+"/secrets?api-version=7.4&maxresults=25", nil)
req.Header.Set("Authorization", "Bearer "+token)

resp, _ := client.Do(req)
var page struct {
    Value    []map[string]any `json:"value"`
    NextLink *string          `json:"nextLink"`
}
json.NewDecoder(resp.Body).Decode(&page)
resp.Body.Close()

// next page via $skiptoken query parameter
```

---

### Build & Development

```bash
# Build
go build ./cmd/kvemu

# Run in development mode
KV_TLS_AUTO=true go run ./cmd/kvemu

# Unit tests
go test ./... -count=1 -race

# E2E tests (in-process server, no Docker)
go test -tags=e2e ./test/e2e/... -v -timeout=5m

# Full gate (unit + E2E)
make gate

# Docker build
make docker/build

# Compatibility tests (require Docker)
make compat/spring27     # Spring Boot 2.7.18
make compat/spring279    # Spring Boot 2.7.9
make compat/spring3      # Spring Boot 3.4.5

# Lint
golangci-lint run ./...
```

---

### Architecture

Hexagonal (Ports & Adapters):

```
cmd/kvemu/main.go                             # CLI entrypoint
internal/config/                              # Environment-based configuration
internal/domain/                              # Domain types (Secret, Key, Cert, Attributes)
internal/ports/                               # Repository interfaces
internal/app/                                 # Service layer (CRUD, crypto, purge scheduler)
internal/adapters/http/                       # HTTP handlers + chi router
internal/adapters/http/middleware/            # Challenge, Auth, Audit, Logger
internal/adapters/crypto/                     # JWT, key generation, TLS, at-rest encryption
internal/adapters/persistence/sqlite/         # SQLite repositories (WAL mode)
deploy/                                       # Dockerfile, docker-compose
migrations/                                   # SQLite schema migrations
```

---

### Key Features

- **TLS auto-generation** — CA + leaf certificate created on first boot
- **Fake AAD** — built-in OIDC identity provider (no external dependencies)
- **Instance Discovery** — MSAL4J-compatible endpoint
- **Soft-delete / recover / purge** — full lifecycle for secrets, keys, and certificates
- **At-rest encryption** — AES-256-GCM encryption of secret values and private key material
- **Purge scheduler** — automatic cleanup of expired soft-deleted items
- **Audit logging** — request audit trail stored in SQLite
- **Pagination** — maxresults + skipToken for all list endpoints
- **Distroless Docker image** — ~8 MB, zero shell, zero libc dependencies

---

<a id="português"></a>
## 🇧🇷 Português

### Sobre

kvemu é um **emulador do Azure Key Vault sem dependências externas** que implementa a API data-plane 7.4. Aplicações que usam os SDKs oficiais do Azure (Java, .NET, Python, Go, etc.) funcionam sem alteração de código — apenas configuração de endpoint + credenciais.

**Stack:** Go 1.24 · chi router · SQLite (pure-Go, sem CGO) · golang-jwt/v5 · AES-256-GCM at-rest · Docker distroless (~8 MB imagem)

---

### Início Rápido (Docker)

```bash
docker compose -f deploy/docker-compose.yml up --build
```

O emulador sobe em `https://localhost:13000` com TLS auto-gerado e um provedor de identidade AAD fake.

---

### Comandos CLI

```bash
kvemu                  # executar servidor (padrão)
kvemu healthcheck      # verificar se o servidor está saudável
kvemu ca export        # exportar certificado CA (PEM)
kvemu seed             # popular secrets/keys de desenvolvimento
```

#### Healthcheck

```bash
kvemu healthcheck --addr https://localhost:13000
```

Usa HTTPS em loopback com `InsecureSkipVerify`. Retorna código 0 se `/healthz` responder 200.

#### Exportar CA

```bash
kvemu ca export --out ca.pem
```

Exporta o certificado CA do emulador. Importe este arquivo no truststore da sua aplicação para evitar erros de verificação TLS.

#### Seed

```bash
docker compose -f deploy/docker-compose.yml exec kvemu /kvemu seed
```

Popula secrets e chaves de desenvolvimento:

| Secret | Valor |
|---|---|
| `db-password` | `sup3rS3cr3t!` |
| `api-key` | `sk-dev-1234567890abcdef` |
| `connection-string` | `Server=localhost;Database=devdb;User=sa;Password=dev!` |
| `COSMO_DB_URL` | `https://cosmos.example.com` |

| Chave | Tipo | Tamanho |
|---|---|---|
| `rsa-key-2048` | RSA | 2048-bit |
| `rsa-key-4096` | RSA | 4096-bit |

---

### Variáveis de Ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `KV_ADDR` | `0.0.0.0:13000` | Endereço de escuta |
| `KV_VAULT_HOST` | `localhost:13000` | Host:porta do vault (usado no challenge, IDs, SANs) |
| `KV_TENANT_ID` | `a0c2a3f5-...` | ID do tenant AAD |
| `KV_DATA` | `./data/kv.db` | Caminho do banco SQLite |
| `KV_TLS_AUTO` | `true` | Auto-gerar CA + certificado leaf na inicialização |
| `KV_TLS_SAN` | _(vazio)_ | SANs extras (separados por vírgula) para o certificado TLS |
| `KV_TLS_CERT` | _(vazio)_ | Certificado TLS próprio (caminho PEM) |
| `KV_TLS_KEY` | _(vazio)_ | Chave privada TLS própria (caminho PEM) |
| `KV_CA_OUT` | `./certs/ca.pem` | Caminho para exportar CA PEM |
| `KV_CERT_DIR` | `./certs` | Diretório de armazenamento dos certificados |
| `KV_AUTH_STRICT` | `false` | Validar assinatura JWT (RS256) em vez de leniente |
| `KV_MASTER_KEY` | _(chave dev)_ | Chave mestra para cifra AES-256-GCM at-rest |

---

### Docker Compose

#### Implantação estilo produção

```yaml
# deploy/docker-compose.yml
services:
  kvemu:
    build:
      context: ..
      dockerfile: deploy/Dockerfile
    image: ghcr.io/dilsonrabelo/kvemu:latest
    container_name: kvemu
    restart: unless-stopped
    ports:
      - "${KV_PORT:-13000}:13000"
    environment:
      KV_ADDR:         "0.0.0.0:13000"
      KV_VAULT_HOST:   "${KV_VAULT_HOST:-localhost:13000}"
      KV_TENANT_ID:    "${KV_TENANT_ID:-a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f}"
      KV_TLS_AUTO:     "true"
      KV_AUTH_STRICT:  "${KV_AUTH_STRICT:-false}"
      KV_MASTER_KEY:   "${KV_MASTER_KEY:-}"
      KV_DATA:         "/data/kv.db"
      KV_CERT_DIR:     "/certs"
      KV_CA_OUT:       "/certs/ca.pem"
    volumes:
      - kvemu-data:/data
      - kvemu-certs:/certs
    healthcheck:
      test: ["CMD", "/kvemu", "healthcheck", "--addr", "https://localhost:13000"]

volumes:
  kvemu-data:
  kvemu-certs:
```

#### App + emulador lado a lado

```yaml
# deploy/docker-compose.app.yml
services:
  kvemu:
    build:
      context: .
      dockerfile: deploy/Dockerfile
    ports:
      - "13000:13000"
    environment:
      KV_ADDR:         "0.0.0.0:13000"
      KV_VAULT_HOST:   "vault.kvemu.local:13000"
      KV_TENANT_ID:    "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
      KV_TLS_AUTO:     "true"
      KV_TLS_SAN:      "login.microsoftonline.com"
      KV_AUTH_STRICT:  "false"
    volumes:
      - data:/data
      - certs:/certs

  kvemu-seed:
    build:
      context: .
      dockerfile: deploy/Dockerfile
    command: ["seed"]
    environment:
      KV_DATA:       "/data/kv.db"
      KV_VAULT_HOST: "vault.kvemu.local:13000"
      KV_TENANT_ID:  "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
    volumes:
      - data:/data
    depends_on:
      kvemu:
        condition: service_healthy

  myapp:
    image: myapp:latest
    environment:
      AZURE_KEYVAULT_ENDPOINT: "https://vault.kvemu.local:13000"
      AZURE_CLIENT_ID:         "dev-client"
      AZURE_CLIENT_SECRET:     "dev-secret"
      AZURE_TENANT_ID:         "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
    volumes:
      - certs:/certs:ro

volumes:
  data:
  certs:
```

---

### TLS & Certificados

#### TLS auto-gerado (padrão)

Defina `KV_TLS_AUTO=true` (padrão). Na primeira inicialização, o kvemu gera:

- **Certificado CA**: `{KV_CERT_DIR}/ca.pem`
- **Certificado leaf**: `{KV_CERT_DIR}/leaf.pem`

Os SANs do certificado leaf incluem `KV_VAULT_HOST`, `localhost`, `kvemu` e quaisquer hosts de `KV_TLS_SAN`. Nas inicializações seguintes, os certificados existentes são reutilizados.

#### Certificado próprio (BYO)

```bash
export KV_TLS_CERT=/caminho/para/server.pem
export KV_TLS_KEY=/caminho/para/server-key.pem
export KV_TLS_AUTO=false
```

#### Importando CA no truststore do cliente

**Java (JRE):**
```bash
keytool -importcert -noprompt \
  -alias kvemu-ca \
  -file ca.pem \
  -keystore $JAVA_HOME/lib/security/cacerts \
  -storepass changeit
```

**Go:**
```go
caPEM, _ := os.ReadFile("ca.pem")
pool := x509.NewCertPool()
pool.AppendCertsFromPEM(caPEM)
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{RootCAs: pool},
    },
}
```

**curl:**
```bash
curl --cacert ca.pem https://localhost:13000/healthz
```

---

### Endpoints da API

#### Identidade AAD (provedor OIDC fake)

Todos servidos pelo mesmo processo, sem dependência externa de identidade.

| Método | Caminho | Descrição |
|---|---|---|
| `GET` | `/{tenant}/discovery/instance` | Descoberta de instância (MSAL4J) |
| `GET` | `/{tenant}/v2.0/.well-known/openid-configuration` | Descoberta OIDC v2 |
| `GET` | `/{tenant}/.well-known/openid-configuration` | Descoberta OIDC v1 |
| `GET` | `/{tenant}/discovery/v2.0/keys` | Endpoint JWKS |
| `POST` | `/{tenant}/oauth2/v2.0/token` | Endpoint de token v2 |
| `POST` | `/{tenant}/oauth2/token` | Endpoint de token v1 |

#### Secrets

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

#### Keys

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

#### Certificates

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

---

### Fluxo de Autenticação

```
App                  kvemu
 |                     |
 |-- GET /secrets ---->|  (sem header Authorization)
 |<---- 401 -----------|  WWW-Authenticate: Bearer authorization="https://host/tenant", resource="https://parent.domain"
 |                     |
 |-- GET /tenant/discovery/instance -->|  (descoberta de instância MSAL4J)
 |<---- 200 (JSON) ----|
 |                     |
 |-- GET /tenant/v2.0/.well-known/openid-configuration -->|
 |<---- 200 (OIDC) ----|
 |                     |
 |-- POST /tenant/oauth2/v2.0/token -->|  (client_credentials)
 |<---- 200 (JWT) -----|
 |                     |
 |-- GET /secrets ----->|  (Authorization: Bearer <jwt>)
 |<---- 200 (JSON) ----|
```

1. App envia requisição sem autenticação → **401** com header `WWW-Authenticate`
2. SDK extrai a URL `authorization` e faz descoberta OIDC
3. SDK obtém token JWT do endpoint de token do AAD fake
4. App reenvia com o header `Authorization: Bearer <token>`

**Modo leniente** (padrão): apenas valida estrutura e expiração do JWT.

**Modo estrito** (`KV_AUTH_STRICT=true`): valida assinatura RS256 contra a chave pública do AAD.

---

### Integração com Java (Spring Boot)

#### Configuração

```properties
# application.properties
spring.cloud.azure.keyvault.secret.property-sources[0].name=vault-secrets
spring.cloud.azure.keyvault.secret.property-sources[0].endpoint=${AZURE_KEYVAULT_ENDPOINT}
spring.cloud.azure.keyvault.secret.property-sources[0].refresh-interval=30m

spring.cloud.azure.credential.client-id=${AZURE_CLIENT_ID:dev-client}
spring.cloud.azure.credential.client-secret=${AZURE_CLIENT_SECRET:dev-secret}
spring.cloud.azure.profile.tenant-id=${AZURE_TENANT_ID}

spring.cloud.azure.profile.environment.active-directory-endpoint=${AZURE_AUTHORITY_HOST:https://login.microsoftonline.com/}

# Resolução de propriedade: mapeia secrets do Key Vault para propriedades Spring
app.cosmos.url=${COSMO_DB_URL}
```

#### Usando secrets no código

```java
@RestController
public class MyController {

    @Value("${db-password}")
    private String dbPassword;

    @Value("${api-key}")
    private String apiKey;

    @Value("${connection-string}")
    private String connectionString;

    @Value("${app.cosmos.url}")
    private String cosmosUrl;   // resolvido do secret COSMO_DB_URL
}
```

#### Resolução de propriedade via CommandLineRunner

```java
@SpringBootApplication
public class Application implements CommandLineRunner {

    private static final Logger log = LoggerFactory.getLogger(Application.class);

    @Value("${app.cosmos.url:NOT_FOUND}")
    private String cosmosUrl;

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @Override
    public void run(String... args) {
        log.info("app.cosmos.url = {}", cosmosUrl);
        // app.cosmos.url é resolvido do secret COSMO_DB_URL do Key Vault
    }
}
```

#### Importando CA no entrypoint

```bash
#!/bin/sh
keytool -importcert -noprompt \
  -alias kvemu-ca \
  -file /certs/ca.pem \
  -keystore $JAVA_HOME/lib/security/cacerts \
  -storepass changeit

exec java -jar /app/app.jar
```

#### Docker Compose com Spring Boot

```yaml
services:
  myapp:
    build: .
    environment:
      AZURE_KEYVAULT_ENDPOINT: "https://vault.kvemu.local:13000"
      AZURE_CLIENT_ID:         "dev-client"
      AZURE_CLIENT_SECRET:     "dev-secret"
      AZURE_TENANT_ID:         "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
      AZURE_AUTHORITY_HOST:    "https://vault.kvemu.local:13000/"
    volumes:
      - certs:/certs:ro
```

#### Compatibilidade verificada

| Spring Boot | Spring Cloud Azure | Status |
|---|---|---|
| 2.7.9 | 4.5.0 | ✅ |
| 2.7.18 | 4.5.0 | ✅ |
| 3.4.5 | 5.21.0 | ✅ |

---

### Integração com Go

#### Cliente HTTP com confiança CA

```go
package main

import (
    "bytes"
    "crypto/tls"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
)

func main() {
    baseURL := "https://localhost:13000"
    vaultHost := "localhost:13000"
    tenantID := "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
    caPath := "./certs/ca.pem"

    // ── 1. Carregar CA e criar cliente HTTP ────────────────────────────
    caPEM, _ := os.ReadFile(caPath)
    pool := x509.NewCertPool()
    pool.AppendCertsFromPEM(caPEM)

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{RootCAs: pool},
        },
    }

    // ── 2. Obter token do endpoint AAD ──────────────────────────────────
    scope := fmt.Sprintf("https://%s/.default", vaultHost)
    tokenURL := fmt.Sprintf("%s/%s/oauth2/v2.0/token", baseURL, tenantID)
    resp, _ := client.PostForm(tokenURL, url.Values{
        "grant_type":    {"client_credentials"},
        "client_id":     {"dev-client"},
        "client_secret": {"dev-secret"},
        "scope":         {scope},
    })
    var result struct {
        AccessToken string `json:"access_token"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    resp.Body.Close()

    // ── 3. Fazer requisição autenticada ──────────────────────────────────
    req, _ := http.NewRequest("GET",
        baseURL+"/secrets/db-password?api-version=7.4", nil)
    req.Header.Set("Authorization", "Bearer "+result.AccessToken)

    resp, _ = client.Do(req)
    body, _ := io.ReadAll(resp.Body)
    resp.Body.Close()
    fmt.Println(string(body))
}
```

#### Criar/atualizar um secret

```go
body := map[string]any{
    "value": "meu-valor-secreto",
}
b, _ := json.Marshal(body)

req, _ := http.NewRequest("PUT",
    baseURL+"/secrets/meu-secret?api-version=7.4",
    bytes.NewReader(b))
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")

resp, _ := client.Do(req)
```

#### Cifrar / Decifrar com chave

```go
// Cifrar
encBody := map[string]any{
    "alg":   "RSA-OAEP-256",
    "value": "SGVsbG8gV29ybGQ=", // base64("Olá Mundo")
}
b, _ := json.Marshal(encBody)
req, _ := http.NewRequest("POST",
    fmt.Sprintf("%s/keys/rsa-key-2048/%s/encrypt?api-version=7.4", baseURL, version),
    bytes.NewReader(b))
// ... mesmos headers de auth ...
resp, _ := client.Do(req)

// Decifrar
decBody := map[string]any{
    "alg":   "RSA-OAEP-256",
    "value": valorCifrado, // da resposta do encrypt
}
// POST /keys/{name}/{version}/decrypt
```

#### Listar secrets com paginação

```go
req, _ := http.NewRequest("GET",
    baseURL+"/secrets?api-version=7.4&maxresults=25", nil)
req.Header.Set("Authorization", "Bearer "+token)

resp, _ := client.Do(req)
var page struct {
    Value    []map[string]any `json:"value"`
    NextLink *string          `json:"nextLink"`
}
json.NewDecoder(resp.Body).Decode(&page)
resp.Body.Close()

// próxima página via parâmetro $skiptoken
```

---

### Build e Desenvolvimento

```bash
# Compilar
go build ./cmd/kvemu

# Executar em modo desenvolvimento
KV_TLS_AUTO=true go run ./cmd/kvemu

# Testes unitários
go test ./... -count=1 -race

# Testes E2E (servidor em processo, sem Docker)
go test -tags=e2e ./test/e2e/... -v -timeout=5m

# Gate completo (unitário + E2E)
make gate

# Build Docker
make docker/build

# Testes de compatibilidade (requer Docker)
make compat/spring27     # Spring Boot 2.7.18
make compat/spring279    # Spring Boot 2.7.9
make compat/spring3      # Spring Boot 3.4.5

# Lint
golangci-lint run ./...
```

---

### Arquitetura

Hexagonal (Ports & Adapters):

```
cmd/kvemu/main.go                             # Ponto de entrada CLI
internal/config/                              # Configuração via variáveis de ambiente
internal/domain/                              # Tipos de domínio (Secret, Key, Cert, Attributes)
internal/ports/                               # Interfaces de repositório
internal/app/                                 # Camada de serviço (CRUD, crypto, purge scheduler)
internal/adapters/http/                       # Handlers HTTP + chi router
internal/adapters/http/middleware/            # Challenge, Auth, Audit, Logger
internal/adapters/crypto/                     # JWT, geração de chaves, TLS, cifra at-rest
internal/adapters/persistence/sqlite/         # Repositórios SQLite (modo WAL)
deploy/                                       # Dockerfile, docker-compose
migrations/                                   # Migrações do schema SQLite
```

---

### Funcionalidades Principais

- **TLS auto-gerado** — certificado CA + leaf criados na primeira inicialização
- **AAD fake** — provedor de identidade OIDC embutido (sem dependências externas)
- **Instance Discovery** — endpoint compatível com MSAL4J
- **Soft-delete / recover / purge** — ciclo de vida completo para secrets, keys e certificates
- **Cifra at-rest** — cifra AES-256-GCM de valores de secrets e material de chave privada
- **Purge scheduler** — limpeza automática de itens soft-deleted expirados
- **Audit logging** — trilha de auditoria de requisições armazenada no SQLite
- **Paginação** — maxresults + skipToken em todos os endpoints de lista
- **Imagem Docker distroless** — ~8 MB, sem shell, sem dependências de libc
