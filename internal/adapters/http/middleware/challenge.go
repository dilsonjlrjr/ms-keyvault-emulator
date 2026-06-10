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

// BuildChallenge monta o valor do header WWW-Authenticate no formato canônico
// compatível com todos os SDKs Azure (incluindo o parser frágil do azure 4.x / Boot 2.7).
//
// Regras invioláveis (doc 05-autenticacao-challenge):
//  - Um único valor, sem espaços soltos ou tokens sem '='
//  - authorization= aponta para https://{vaultHost}/{tenantID} sem query string
//  - resource= usa o domínio pai do vault host para satisfazer a validação
//    hierárquica do SDK (host.endsWith("." + scopeHost))
//  - Aspas duplas em todos os valores
//  - Separador ', ' (vírgula+espaço)
func BuildChallenge(vaultHost, tenantID string) string {
	authority := fmt.Sprintf("https://%s/%s", vaultHost, tenantID)
	resource := fmt.Sprintf("https://%s", parentDomain(vaultHost))
	return fmt.Sprintf(`Bearer authorization="%s", resource="%s"`, authority, resource)
}

// Challenge é um middleware chi que emite 401 + WWW-Authenticate quando
// não há header Authorization na requisição.
// Chamado ANTES do middleware de validação de token.
func Challenge(vaultHost, tenantID string) func(http.Handler) http.Handler {
	header := BuildChallenge(vaultHost, tenantID)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				// Set (nunca Add) — evita duplicar o header
				w.Header().Set("WWW-Authenticate", header)
				w.WriteHeader(http.StatusUnauthorized)
				return // body vazio
			}
			next.ServeHTTP(w, r)
		})
	}
}
