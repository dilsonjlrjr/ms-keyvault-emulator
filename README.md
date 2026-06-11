# kvemu — Azure Key Vault Emulator

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

### About

kvemu is a **zero-dependency Azure Key Vault emulator** that implements the data-plane API 7.4. Applications using the official Azure SDKs (Java, .NET, Python, Go, etc.) work without code changes — only endpoint + credentials configuration.

**Stack:** Go 1.24 · chi router · SQLite (pure-Go, no CGO) · golang-jwt/v5 · AES-256-GCM at-rest · Docker distroless (~8 MB image)

### Quick Start (Docker)

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Emulator starts on `https://localhost:13000` with auto-generated TLS and a fake AAD identity provider.

```bash
# Seed development data
docker compose -f deploy/docker-compose.yml exec kvemu /kvemu seed
```

#### Load pre-built image (air-gapped / offline)

```bash
# Export on build machine:
make dist                        # builds binary + image → dist/image-docker/kvemu-*.tar.gz
scp dist/image-docker/kvemu-*.tar.gz target-host:

# Load on target machine:
docker load -i kvemu-*.tar.gz    # imports ghcr.io/dilsonrabelo/kvemu
docker compose -f deploy/docker-compose.yml up
```

### Verified Compatibility

| Spring Boot | Spring Cloud Azure | Status |
|---|---|---|
| 2.7.9 | 4.5.0 | ✅ |
| 2.7.18 | 4.5.0 | ✅ |
| 3.4.5 | 5.21.0 | ✅ |

### Documentation

| Document | Description |
|---|---|
| [CLI & Configuration](docs/cli-and-config.md) | CLI commands, environment variables, Docker Compose |
| [API Endpoints](docs/api-endpoints.md) | AAD, Secrets, Keys, Certificates, Vault Management |
| [Deployment](docs/deployment.md) | TLS, certificates, air-gapped deployment |
| [Authentication Flow](docs/auth-flow.md) | Challenge flow, multi-vault hostname-based routing |
| [Integrations](docs/integrations.md) | Spring Boot (Java) + Go examples |
| [Development](docs/development.md) | Build, test, architecture, key features |

---

<a id="português"></a>
## 🇧🇷 Português

### Sobre

kvemu é um **emulador do Azure Key Vault sem dependências externas** que implementa a API data-plane 7.4. Aplicações que usam os SDKs oficiais do Azure (Java, .NET, Python, Go, etc.) funcionam sem alteração de código — apenas configuração de endpoint + credenciais.

**Stack:** Go 1.24 · chi router · SQLite (pure-Go, sem CGO) · golang-jwt/v5 · AES-256-GCM at-rest · Docker distroless (~8 MB imagem)

### Início Rápido (Docker)

```bash
docker compose -f deploy/docker-compose.yml up --build
```

O emulador sobe em `https://localhost:13000` com TLS auto-gerado e um provedor de identidade AAD fake.

```bash
# Popular dados de desenvolvimento
docker compose -f deploy/docker-compose.yml exec kvemu /kvemu seed
```

#### Carregar imagem pré-buildada (offline / air-gapped)

```bash
# Exportar na máquina de build:
make dist                        # compila binário + imagem → dist/image-docker/kvemu-*.tar.gz
scp dist/image-docker/kvemu-*.tar.gz maquina-alvo:

# Carregar na máquina alvo:
docker load -i kvemu-*.tar.gz    # importa ghcr.io/dilsonrabelo/kvemu
docker compose -f deploy/docker-compose.yml up
```

### Compatibilidade Verificada

| Spring Boot | Spring Cloud Azure | Status |
|---|---|---|
| 2.7.9 | 4.5.0 | ✅ |
| 2.7.18 | 4.5.0 | ✅ |
| 3.4.5 | 5.21.0 | ✅ |

### Documentação

| Documento | Descrição |
|---|---|
| [CLI & Configuração](docs/cli-and-config.md) | Comandos CLI, variáveis de ambiente, Docker Compose |
| [API Endpoints](docs/api-endpoints.md) | AAD, Secrets, Keys, Certificates, Gestão de Vaults |
| [Deployment](docs/deployment.md) | TLS, certificados, implantação offline |
| [Fluxo de Autenticação](docs/auth-flow.md) | Challenge flow, roteamento multi-vault por hostname |
| [Integrações](docs/integrations.md) | Spring Boot (Java) + Go exemplos |
| [Desenvolvimento](docs/development.md) | Build, testes, arquitetura, funcionalidades |
