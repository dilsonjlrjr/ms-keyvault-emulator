package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
)

// parentDomain extrai o domínio pai de um host (com ou sem porta).
// Exemplos:
//
//	"vault.kvemu.local:13000" → "kvemu.local"
//	"myvault.vault.azure.net" → "vault.azure.net"
//	"localhost"               → "localhost"  (sem pai, retorna igual)
//	"127.0.0.1:58995"         → "127.0.0.1"  (IP, retorna sem porta)
func parentDomain(host string) string {
	// Remove porta se presente
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	// Se for IP, retorna como está
	if net.ParseIP(host) != nil {
		return host
	}
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[1:], ".")
}

// challengeResource monta o host do resource= do challenge: domínio pai
// preservando a porta do vault host (se houver).
//
//	"gdr.kvemu.local:13000" → "kvemu.local:13000"
//	"myvault.vault.azure.net" → "vault.azure.net"
//
// A porta é obrigatória para o SDK Python: ele compara netloc completo
// (host:porta) da URL do vault com sufixo "."+resource — sem porta no
// resource, "gdr.kvemu.local:13000".endswith(".kvemu.local") falha e o
// cliente aborta com "challenge resource does not match the requested
// domain". O SDK Java compara só getHost() (sem porta), então a porta no
// resource não o afeta.
func challengeResource(vaultHost string) string {
	host := parentDomain(vaultHost)
	if _, port, ok := strings.Cut(vaultHost, ":"); ok {
		return host + ":" + port
	}
	return host
}

// BuildChallenge monta o valor do header WWW-Authenticate no formato canônico
// compatível com todos os SDKs Azure (incluindo o parser frágil do azure 4.x / Boot 2.7).
//
// Regras invioláveis (doc 05-autenticacao-challenge):
//  - Um único valor, sem espaços soltos ou tokens sem '='
//  - authorization= aponta para https://{vaultHost}/{tenantID} sem query string
//  - resource= usa o domínio pai do vault host (com a porta do vault) para
//    satisfazer a validação hierárquica dos SDKs: Java compara
//    host.endsWith("." + scopeHost); Python compara netloc completo
//    (com porta) via endswith
//  - Aspas duplas em todos os valores
//  - Separador ', ' (vírgula+espaço)
func BuildChallenge(vaultHost, tenantID string) string {
	authority := fmt.Sprintf("https://%s/%s", vaultHost, tenantID)
	resource := fmt.Sprintf("https://%s", challengeResource(vaultHost))
	return fmt.Sprintf(`Bearer authorization="%s", resource="%s"`, authority, resource)
}

// Challenge é um middleware chi que emite 401 + WWW-Authenticate quando
// não há header Authorization na requisição.
// Chamado ANTES do middleware de validação de token.
// O vault é obtido do context (injetado pelo VaultResolver).
func Challenge() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				vault := VaultFromContext(r.Context())
				if vault == nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				header := BuildChallenge(vault.Host, vault.TenantID)
				w.Header().Set("WWW-Authenticate", header)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
