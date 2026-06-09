package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
)

type aadHandler struct {
	key      *kvCrypto.AADKey
	vaultHost string
}

func newAADHandler(key *kvCrypto.AADKey, vaultHost string) *aadHandler {
	return &aadHandler{key: key, vaultHost: vaultHost}
}

func (h *aadHandler) issuer(tenantID string) string {
	return fmt.Sprintf("https://%s/%s/v2.0", h.vaultHost, tenantID)
}

// OIDC discovery v2 — GET /{tenant}/v2.0/.well-known/openid-configuration
func (h *aadHandler) oidcV2(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")
	w.Header().Set("Content-Type", "application/json")
	w.Write(kvCrypto.OIDCConfig(h.issuer(tenant)))
}

// OIDC discovery v1 — GET /{tenant}/.well-known/openid-configuration
func (h *aadHandler) oidcV1(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")
	w.Header().Set("Content-Type", "application/json")
	w.Write(kvCrypto.OIDCConfig(h.issuer(tenant)))
}

// JWKS — GET /{tenant}/discovery/v2.0/keys
func (h *aadHandler) jwks(w http.ResponseWriter, r *http.Request) {
	b, err := h.key.JWKS()
	if err != nil {
		http.Error(w, "jwks error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// Token endpoint v2 — POST /{tenant}/oauth2/v2.0/token
func (h *aadHandler) tokenV2(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")
	h.handleToken(w, r, tenant)
}

// Token endpoint v1 — POST /{tenant}/oauth2/token
func (h *aadHandler) tokenV1(w http.ResponseWriter, r *http.Request) {
	tenant := chi.URLParam(r, "tenant")
	h.handleToken(w, r, tenant)
}

func (h *aadHandler) handleToken(w http.ResponseWriter, r *http.Request, tenantID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	grantType := r.FormValue("grant_type")
	if grantType != "client_credentials" {
		http.Error(w, `{"error":"unsupported_grant_type"}`, http.StatusBadRequest)
		return
	}

	clientID := r.FormValue("client_id")
	if clientID == "" {
		clientID = "unknown"
	}

	issuer := h.issuer(tenantID)
	token, exp, err := h.key.IssueToken(issuer, clientID, tenantID)
	if err != nil {
		http.Error(w, "token error", http.StatusInternalServerError)
		return
	}

	resp := map[string]any{
		"token_type":     "Bearer",
		"expires_in":     exp - 3600 + 3600, // sempre 3600s de validade restante
		"ext_expires_in": 3600,
		"access_token":   token,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
