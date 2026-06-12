package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/app"
	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type VaultHandlers struct {
	svc        *app.VaultService
	secretRepo ports.SecretRepository
	keyRepo    ports.KeyRepository
	certRepo   *sqlite.CertRepo
}

func NewVaultHandlers(svc *app.VaultService, secretRepo ports.SecretRepository, keyRepo ports.KeyRepository, certRepo *sqlite.CertRepo) *VaultHandlers {
	return &VaultHandlers{svc: svc, secretRepo: secretRepo, keyRepo: keyRepo, certRepo: certRepo}
}

type vaultResp struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	DisplayName string `json:"displayName"`
	TenantID    string `json:"tenantId"`
	Created     int64  `json:"created"`
	Updated     int64  `json:"updated"`
}

func toVaultResp(v *domain.Vault) vaultResp {
	return vaultResp{
		Name:        v.Name,
		Host:        v.Host,
		DisplayName: v.DisplayName,
		TenantID:    v.TenantID,
		Created:     v.Created,
		Updated:     v.Updated,
	}
}

func (h *VaultHandlers) List(w http.ResponseWriter, r *http.Request) {
	vaults, err := h.svc.List(r.Context())
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]vaultResp, len(vaults))
	for i, v := range vaults {
		items[i] = toVaultResp(v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"value": items})
}

func (h *VaultHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		TenantID    string `json:"tenantId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid JSON"})
		return
	}
	if body.Name == "" {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "name is required"})
		return
	}
	if body.TenantID == "" {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "tenantId is required"})
		return
	}

	v, err := h.svc.Create(r.Context(), body.Name, body.DisplayName, body.TenantID)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toVaultResp(v))
}

func (h *VaultHandlers) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	v, err := h.svc.GetByName(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toVaultResp(v))
}

func (h *VaultHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.svc.Delete(r.Context(), name); err != nil {
		middleware.DomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *VaultHandlers) Export(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	vault, err := h.svc.GetByName(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}

	export := &domain.VaultExport{
		Version: "1.0",
		Vault: domain.VaultExportMeta{
			Name:        vault.Name,
			DisplayName: vault.DisplayName,
			TenantID:    vault.TenantID,
		},
	}

	if h.secretRepo != nil {
		secrets, _, err := h.secretRepo.List(r.Context(), name, 1000, "")
		if err == nil {
			for _, s := range secrets {
				full, err := h.secretRepo.Get(r.Context(), name, s.Name, "")
				if err != nil {
					continue
				}
				export.Secrets = append(export.Secrets, domain.SecretExportItem{
					Name:        full.Name,
					Value:       full.Value,
					ContentType: full.ContentType,
					Tags:        full.Tags,
					Attributes:  attrsToExport(&full.Attributes),
				})
			}
		}
	}

	if h.keyRepo != nil {
		keys, _, err := h.keyRepo.List(r.Context(), name, 1000, "")
		if err == nil {
			for _, k := range keys {
				export.Keys = append(export.Keys, domain.KeyExportItem{
					Name:    k.Name,
					Kty:     k.Kty,
					KeySize: k.KeySize,
					Crv:     k.Crv,
					KeyOps:  k.KeyOps,
				})
			}
		}
	}

	if h.certRepo != nil {
		certs, _, err := h.certRepo.List(r.Context(), name, 1000, "")
		if err == nil {
			for _, c := range certs {
				item := domain.CertExportItem{Name: c.Name}
				if c.CerDER != nil {
					item.CerBase64 = base64.StdEncoding.EncodeToString(c.CerDER)
				}
				export.Certificates = append(export.Certificates, item)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, name))
	json.NewEncoder(w).Encode(export)
}

func (h *VaultHandlers) Import(w http.ResponseWriter, r *http.Request) {
	var export domain.VaultExport
	if err := json.NewDecoder(r.Body).Decode(&export); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid JSON body"})
		return
	}
	if export.Vault.Name == "" {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "vault.name is required"})
		return
	}

	result := domain.VaultImportResult{Vault: export.Vault.Name}

	_, err := h.svc.Create(r.Context(), export.Vault.Name, export.Vault.DisplayName, export.Vault.TenantID)
	if err != nil {
		_, err = h.svc.GetByName(r.Context(), export.Vault.Name)
		if err != nil {
			middleware.DomainError(w, err)
			return
		}
	}

	attrs := domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable}

		if h.secretRepo != nil {
		for _, s := range export.Secrets {
			err := h.secretRepo.Upsert(r.Context(), export.Vault.Name, &domain.SecretVersion{
				Name:        s.Name,
				Value:       s.Value,
				ContentType: s.ContentType,
				Tags:        s.Tags,
				Attributes:  exportAttrsToAttrs(s.Attributes, attrs),
			})
			if err != nil {
				result.SecretsSkipped++
			} else {
				result.SecretsCreated++
			}
		}
	}

	if h.keyRepo != nil {
		keySvc := app.NewKeyService(h.keyRepo, export.Vault.Name)
		for _, k := range export.Keys {
			_, err := keySvc.Create(r.Context(), k.Name, k.Kty, k.Crv, k.KeySize, k.KeyOps, attrs)
			if err != nil {
				result.KeysSkipped++
			} else {
				result.KeysCreated++
			}
		}
	}

	if h.certRepo != nil && h.secretRepo != nil && h.keyRepo != nil {
		secretSvc := app.NewSecretService(h.secretRepo, export.Vault.Name)
		keySvc := app.NewKeyService(h.keyRepo, export.Vault.Name)
		certSvc := app.NewCertService(h.certRepo, secretSvc, keySvc, export.Vault.Name)

		for _, c := range export.Certificates {
			var pemBlock string
			if c.CerBase64 != "" {
				der, err := base64.StdEncoding.DecodeString(c.CerBase64)
				if err == nil {
					b64 := base64.StdEncoding.EncodeToString(der)
					pemBlock = "-----BEGIN CERTIFICATE-----\n" + wrapLines(b64, 64) + "\n-----END CERTIFICATE-----"
				}
			}
			if pemBlock == "" {
				result.CertsSkipped++
				continue
			}
			_, err := certSvc.Import(r.Context(), c.Name, pemBlock, c.PolicyJSON, attrs)
			if err != nil {
				result.CertsSkipped++
			} else {
				result.CertsCreated++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

func wrapLines(s string, n int) string {
	var result string
	for len(s) > n {
		result += s[:n] + "\n"
		s = s[n:]
	}
	if s != "" {
		result += s
	}
	return result
}

func attrsToExport(a *domain.Attributes) domain.ExportAttributes {
	if a == nil {
		return domain.ExportAttributes{}
	}
	ea := domain.ExportAttributes{
		Enabled: &a.Enabled,
	}
	if a.NotBefore != nil {
		ea.Nbf = a.NotBefore
	}
	if a.Expires != nil {
		ea.Exp = a.Expires
	}
	return ea
}

func exportAttrsToAttrs(ea domain.ExportAttributes, def domain.Attributes) domain.Attributes {
	a := def
	if ea.Enabled != nil {
		a.Enabled = *ea.Enabled
	}
	if ea.Nbf != nil {
		a.NotBefore = ea.Nbf
	}
	if ea.Exp != nil {
		a.Expires = ea.Exp
	}
	return a
}
