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

### Local Machine Setup (for client applications)

#### 1. Download CA Certificate

```bash
# From the container:
docker cp kvemu:/certs/ca.pem ./ca.pem

# Or from the web panel at http://<host>:3000/ca

# Linux / macOS / Windows — import into system trust store:
# Debian/Ubuntu:
sudo cp ca.pem /usr/local/share/ca-certificates/kvemu.crt && sudo update-ca-certificates
# RHEL/Fedora:
sudo cp ca.pem /etc/pki/ca-trust/source/anchors/kvemu.crt && sudo update-ca-trust
# macOS:
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.pem
# Windows (PowerShell — run as Administrator):
Import-Certificate -FilePath .\ca.pem -CertStoreLocation Cert:\LocalMachine\Root
```

#### 2. Configure /etc/hosts

Add these entries so your application resolves AAD endpoints and vault hostnames to the emulator:

```
# /etc/hosts — Key Vault Emulator
192.168.100.112 login.microsoftonline.com
192.168.100.112 lab-dilson
192.168.100.112 vault.kvemu.local
```

Replace `192.168.100.112` with the actual IP of the machine running kvemu.

#### 3. Custom Hosts File for Java (alternative to /etc/hosts)

If you cannot edit `/etc/hosts` (e.g., CI, managed environments), use a custom hosts file with the JVM:

```bash
# Create custom-hosts file:
cat > custom-hosts << 'EOF'
192.168.100.112 login.microsoftonline.com
192.168.100.112 lab-dilson
192.168.100.112 vault.kvemu.local
EOF

# Set JVM property:
JAVA_TOOL_OPTIONS=-Djdk.net.hosts.file=/path/to/custom-hosts
```

This works from Java 8+ and overrides `/etc/hosts` for DNS resolution within the JVM only.

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

### Configuração da Máquina Local (para aplicações cliente)

#### 1. Baixar Certificado CA

```bash
# Do container:
docker cp kvemu:/certs/ca.pem ./ca.pem

# Ou pelo painel web em http://<host>:3000/ca

# Linux / macOS / Windows — importar no trust store do sistema:
# Debian/Ubuntu:
sudo cp ca.pem /usr/local/share/ca-certificates/kvemu.crt && sudo update-ca-certificates
# RHEL/Fedora:
sudo cp ca.pem /etc/pki/ca-trust/source/anchors/kvemu.crt && sudo update-ca-trust
# macOS:
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.pem
# Windows (PowerShell — executar como Administrador):
Import-Certificate -FilePath .\ca.pem -CertStoreLocation Cert:\LocalMachine\Root
```

#### 2. Configurar /etc/hosts

Adicione estas entradas para que sua aplicação resolva os endpoints AAD e hostnames do vault para o emulador:

```
# /etc/hosts — Key Vault Emulator
192.168.100.112 login.microsoftonline.com
192.168.100.112 lab-dilson
192.168.100.112 vault.kvemu.local
```

Substitua `192.168.100.112` pelo IP real da máquina onde o kvemu está rodando.

#### 3. Arquivo de Hosts Customizado para Java (alternativa ao /etc/hosts)

Se você não pode editar o `/etc/hosts` (ex: CI, ambientes gerenciados), use um arquivo de hosts customizado com a JVM:

```bash
# Criar arquivo custom-hosts:
cat > custom-hosts << 'EOF'
192.168.100.112 login.microsoftonline.com
192.168.100.112 lab-dilson
192.168.100.112 vault.kvemu.local
EOF

# Configurar propriedade da JVM:
JAVA_TOOL_OPTIONS=-Djdk.net.hosts.file=/caminho/para/custom-hosts
```

Isso funciona a partir do Java 8+ e sobrescreve o `/etc/hosts` para resolução DNS apenas dentro da JVM.

### Documentação

| Documento | Descrição |
|---|---|
| [CLI & Configuração](docs/cli-and-config.md) | Comandos CLI, variáveis de ambiente, Docker Compose |
| [API Endpoints](docs/api-endpoints.md) | AAD, Secrets, Keys, Certificates, Gestão de Vaults |
| [Deployment](docs/deployment.md) | TLS, certificados, implantação offline |
| [Fluxo de Autenticação](docs/auth-flow.md) | Challenge flow, roteamento multi-vault por hostname |
| [Integrações](docs/integrations.md) | Spring Boot (Java) + Go exemplos |
| [Desenvolvimento](docs/development.md) | Build, testes, arquitetura, funcionalidades |
