# Development

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

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
internal/domain/                              # Domain types (Secret, Key, Cert, Attributes, Vault)
internal/ports/                               # Repository interfaces
internal/app/                                 # Service layer (CRUD, crypto, purge scheduler)
internal/adapters/http/                       # HTTP handlers + chi router
internal/adapters/http/middleware/            # Challenge, Auth, Audit, VaultResolver, Logger
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
- **Multi-vault** — hostname-based routing (e.g. `prod.kvemu.local`, `staging.kvemu.local`), vault CRUD, export/import

---

<a id="português"></a>
## 🇧🇷 Português

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
internal/domain/                              # Tipos de domínio (Secret, Key, Cert, Attributes, Vault)
internal/ports/                               # Interfaces de repositório
internal/app/                                 # Camada de serviço (CRUD, crypto, purge scheduler)
internal/adapters/http/                       # Handlers HTTP + chi router
internal/adapters/http/middleware/            # Challenge, Auth, Audit, VaultResolver, Logger
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
- **Multi-vault** — roteamento por hostname (ex: `prod.kvemu.local`, `staging.kvemu.local`), CRUD de vaults, export/import
