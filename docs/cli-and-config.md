# CLI & Configuration

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

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
| `KV_BASE_DOMAIN` | `kvemu.local` | Base domain for vault hostname routing (v1.1) |
| `KV_DEFAULT_VAULT` | `vault` | Default vault name (v1.1) |

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

<a id="português"></a>
## 🇧🇷 Português

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
| `KV_BASE_DOMAIN` | `kvemu.local` | Domínio base para roteamento por hostname (v1.1) |
| `KV_DEFAULT_VAULT` | `vault` | Nome do vault padrão (v1.1) |

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
