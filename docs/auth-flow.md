# Authentication Flow

**[🇺🇸 English](#english) · [🇧🇷 Português](#português)**

---

<a id="english"></a>
## 🇺🇸 English

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

### Multi-Vault Hostname-Based Routing (v1.1)

Each vault is identified by the request's `Host` header. The `VaultResolver` middleware extracts the vault name from the subdomain:

- `https://prod.kvemu.local:13000/secrets/...` → vault "prod"
- `https://staging.kvemu.local:13000/secrets/...` → vault "staging"
- `https://vault.kvemu.local:13000/secrets/...` → vault "vault" (default)

The TLS certificate auto-generates with wildcard SAN `*.{KV_BASE_DOMAIN}`, ensuring valid TLS for any subdomain.

---

<a id="português"></a>
## 🇧🇷 Português

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

### Roteamento Multi-Vault por Hostname (v1.1)

Cada vault é identificado pelo header `Host` da request HTTP. O middleware `VaultResolver` extrai o nome do vault do subdomínio:

- `https://prod.kvemu.local:13000/secrets/...` → vault "prod"
- `https://staging.kvemu.local:13000/secrets/...` → vault "staging"
- `https://vault.kvemu.local:13000/secrets/...` → vault "vault" (default)

O certificado TLS é auto-gerado com wildcard SAN `*.{KV_BASE_DOMAIN}`, garantindo TLS válido para qualquer subdomínio.
