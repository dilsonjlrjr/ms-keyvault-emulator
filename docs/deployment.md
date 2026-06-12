# Deployment

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

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

### Air-gapped / Offline Deployment

Build and export the Docker image on a connected machine, then load it on the target:

```bash
# Export on build machine:
make dist                        # builds binary + image → dist/image-docker/kvemu-*.tar.gz
scp dist/image-docker/kvemu-*.tar.gz target-host:

# Load on target machine:
docker load -i kvemu-*.tar.gz    # imports ghcr.io/dilsonrabelo/kvemu
docker compose -f deploy/docker-compose.yml up
```

---

<a id="português"></a>
## 🇧🇷 Português

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

### Implantação Offline (Air-gapped)

Compile e exporte a imagem Docker em uma máquina conectada, depois carregue na máquina alvo:

```bash
# Exportar na máquina de build:
make dist                        # compila binário + imagem → dist/image-docker/kvemu-*.tar.gz
scp dist/image-docker/kvemu-*.tar.gz maquina-alvo:

# Carregar na máquina alvo:
docker load -i kvemu-*.tar.gz    # importa ghcr.io/dilsonrabelo/kvemu
docker compose -f deploy/docker-compose.yml up
```
