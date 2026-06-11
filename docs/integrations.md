# Integrations

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

### Spring Boot (Java)

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

#### Local Development (without Docker)

When running the app directly on your machine against a remote kvemu instance,
the Azure SDK's MSAL4J contacts `login.microsoftonline.com` by default. Using
`AZURE_AUTHORITY_HOST` causes silent crashes in SDK 4.x (Spring Boot 2.7).

Instead, redirect DNS **only for the JVM process** using `jdk.net.hosts.file`
(requires Java 18+ — Zulu 21 is fine):

**1. Configure the kvemu server to accept `login.microsoftonline.com` traffic:**

```yaml
# docker-compose.yml on the server
services:
  kvemu:
    ports:
      - "13001:13000"
      - "443:13000"                   # ← required for MSAL4J (default HTTPS port)
    environment:
      KV_TLS_SAN: "login.microsoftonline.com"  # ← required for TLS hostname validation
```

**2. Import the CA certificate into the JVM truststore:**

```bash
keytool -importcert -noprompt \
  -alias kvemu-ca \
  -file ca.pem \
  -keystore $JAVA_HOME/lib/security/cacerts \
  -storepass changeit
```

**3. Create a custom hosts file in the project:**

```
# custom-hosts (in project root)
192.168.100.112 login.microsoftonline.com
192.168.100.112 lab-dilson
```

**4. Create a `.env` file for the run configuration:**

```bash
# gmsuite-configs-dev.env
JAVA_TOOL_OPTIONS=-Djdk.net.hosts.file=/absolute/path/to/project/custom-hosts

AzureKeyVault__Enabled=true
AzureKeyVault__Endpoint=https://lab-dilson:13001/
AzureKeyVault__Common__Endpoint=https://lab-dilson:13002/

AZURE_TENANT_ID=a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f
AZURE_CLIENT_ID=kv-interface
AZURE_CLIENT_SECRET=kv-interface-secret
```

**How it works:**

- The JVM resolves `login.microsoftonline.com` via the custom hosts file — system `/etc/hosts` is untouched
- MSAL4J contacts `https://login.microsoftonline.com:443/{tenant}/oauth2/v2.0/token`
- Port 443 on the server forwards to the kvemu instance
- TLS handshake succeeds because the cert includes `login.microsoftonline.com` as SAN
- Token is obtained from kvemu's fake AAD, then used for data-plane calls
- `portal.azure.com` and other Azure services continue to work normally

This approach is **verified compatible with all supported SDK versions** (4.x and 5.x).

#### Verified compatibility

| Spring Boot | Spring Cloud Azure | Status |
|---|---|---|
| 2.7.9 | 4.5.0 | ✅ |
| 2.7.18 | 4.5.0 | ✅ |
| 3.4.5 | 5.21.0 | ✅ |

---

### Go

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

    caPEM, _ := os.ReadFile(caPath)
    pool := x509.NewCertPool()
    pool.AppendCertsFromPEM(caPEM)

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{RootCAs: pool},
        },
    }

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
resp, _ := client.Do(req)

// Decrypt
decBody := map[string]any{
    "alg":   "RSA-OAEP-256",
    "value": encryptedValue, // from encrypt response
}
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
```

---

<a id="português"></a>
## 🇧🇷 Português

### Spring Boot (Java)

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

#### Desenvolvimento Local (sem Docker)

Ao rodar a aplicação diretamente na sua máquina contra uma instância remota do
kvemu, o MSAL4J do SDK Azure contata `login.microsoftonline.com` por padrão.
Usar `AZURE_AUTHORITY_HOST` causa crash silencioso no SDK 4.x (Spring Boot 2.7).

A solução é redirecionar o DNS **apenas para o processo JVM** usando
`jdk.net.hosts.file` (requer Java 18+ — Zulu 21 funciona):

**1. Configure o servidor kvemu para aceitar tráfego de `login.microsoftonline.com`:**

```yaml
# docker-compose.yml no servidor
services:
  kvemu:
    ports:
      - "13001:13000"
      - "443:13000"                   # ← obrigatório para MSAL4J (porta HTTPS padrão)
    environment:
      KV_TLS_SAN: "login.microsoftonline.com"  # ← obrigatório para validação TLS do hostname
```

**2. Importe o certificado CA no truststore da JVM:**

```bash
keytool -importcert -noprompt \
  -alias kvemu-ca \
  -file ca.pem \
  -keystore $JAVA_HOME/lib/security/cacerts \
  -storepass changeit
```

**3. Crie um arquivo de hosts customizado no projeto:**

```
# custom-hosts (na raiz do projeto)
192.168.100.112 login.microsoftonline.com
192.168.100.112 lab-dilson
```

**4. Crie um arquivo `.env` para a configuração de execução:**

```bash
# gmsuite-configs-dev.env
JAVA_TOOL_OPTIONS=-Djdk.net.hosts.file=/caminho/absoluto/para/projeto/custom-hosts

AzureKeyVault__Enabled=true
AzureKeyVault__Endpoint=https://lab-dilson:13001/
AzureKeyVault__Common__Endpoint=https://lab-dilson:13002/

AZURE_TENANT_ID=a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f
AZURE_CLIENT_ID=kv-interface
AZURE_CLIENT_SECRET=kv-interface-secret
```

**Como funciona:**

- A JVM resolve `login.microsoftonline.com` via o arquivo de hosts customizado — o `/etc/hosts` do sistema permanece intacto
- MSAL4J contata `https://login.microsoftonline.com:443/{tenant}/oauth2/v2.0/token`
- A porta 443 no servidor encaminha para a instância kvemu
- TLS handshake funciona porque o certificado inclui `login.microsoftonline.com` como SAN
- Token é obtido do AAD fake do kvemu e usado nas chamadas data-plane
- `portal.azure.com` e outros serviços Azure continuam funcionando normalmente

Esta abordagem é **compatível com todas as versões de SDK suportadas** (4.x e 5.x).

#### Compatibilidade verificada

| Spring Boot | Spring Cloud Azure | Status |
|---|---|---|
| 2.7.9 | 4.5.0 | ✅ |
| 2.7.18 | 4.5.0 | ✅ |
| 3.4.5 | 5.21.0 | ✅ |

---

### Go

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

    caPEM, _ := os.ReadFile(caPath)
    pool := x509.NewCertPool()
    pool.AppendCertsFromPEM(caPEM)

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{RootCAs: pool},
        },
    }

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
resp, _ := client.Do(req)

// Decifrar
decBody := map[string]any{
    "alg":   "RSA-OAEP-256",
    "value": valorCifrado, // da resposta do encrypt
}
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
```
