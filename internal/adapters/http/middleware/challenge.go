package middleware

import (
	"fmt"
	"net/http"
)

const kvAudience = "https://vault.azure.net"

// BuildChallenge monta o valor do header WWW-Authenticate no formato canônico
// compatível com todos os SDKs Azure (incluindo o parser frágil do azure 4.x / Boot 2.7).
//
// Regras invioláveis (doc 05-autenticacao-challenge):
//  - Um único valor, sem espaços soltos ou tokens sem '='
//  - authorization= aponta para https://{vaultHost}/{tenantID} sem query string
//  - resource= é https://vault.azure.net sem '/.default' e sem barra final
//  - Aspas duplas em todos os valores
//  - Separador ', ' (vírgula+espaço)
func BuildChallenge(vaultHost, tenantID string) string {
	authority := fmt.Sprintf("https://%s/%s", vaultHost, tenantID)
	return fmt.Sprintf(`Bearer authorization="%s", resource="%s"`, authority, kvAudience)
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
