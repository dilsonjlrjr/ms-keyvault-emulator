package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/app"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

type CertHandlers struct {
	svc       *app.CertService
	vaultHost string
}

func NewCertHandlers(svc *app.CertService, vaultHost string) *CertHandlers {
	return &CertHandlers{svc: svc, vaultHost: vaultHost}
}

func (h *CertHandlers) host(r *http.Request) string {
	return vaultHostFromContext(r, h.vaultHost)
}

// ─── POST /certificates/{name}/create ────────────────────────────────────────

func (h *CertHandlers) Create(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Policy map[string]any `json:"policy"`
		Attrs  struct {
			Enabled *bool `json:"enabled"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Policy == nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "policy is required"})
		return
	}

	attrs := domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable}
	if body.Attrs.Enabled != nil {
		attrs.Enabled = *body.Attrs.Enabled
	}

	cv, err := h.svc.Create(r.Context(), name, body.Policy, attrs)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	// Azure retorna CertificateOperation com status "completed" para issuer Self
	op := map[string]any{
		"id":            domain.ID(h.host(r), "certificates", name, "") + "/pending",
		"issuer":        map[string]string{"name": "Self"},
		"csr":           base64.StdEncoding.EncodeToString(cv.CerDER),
		"status":        "completed",
		"target":        domain.ID(h.host(r), "certificates", name, cv.Version),
	}
	writeJSON(w, http.StatusAccepted, op)
}

// ─── PUT /certificates/{name}/import ─────────────────────────────────────────

func (h *CertHandlers) Import(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Value  string         `json:"value"` // PEM ou base64 do PFX
		Policy map[string]any `json:"policy"`
		Attrs  struct {
			Enabled *bool `json:"enabled"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Value == "" {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "value is required"})
		return
	}

	attrs := domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable}
	if body.Attrs.Enabled != nil {
		attrs.Enabled = *body.Attrs.Enabled
	}
	if body.Policy == nil {
		body.Policy = map[string]any{}
	}

	cv, err := h.svc.Import(r.Context(), name, body.Value, body.Policy, attrs)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app.CertToJSON(cv, body.Policy, h.host(r)))
}

// ─── GET /certificates/{name}[/{version}] ────────────────────────────────────

func (h *CertHandlers) Get(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	cv, policy, err := h.svc.Get(r.Context(), name, version)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app.CertToJSON(cv, policy, h.host(r)))
}

// ─── GET /certificates/{name}/policy ─────────────────────────────────────────

func (h *CertHandlers) GetPolicy(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	policy, err := h.svc.GetPolicy(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app.PolicyJSON(policy, h.host(r), name))
}

// ─── PATCH /certificates/{name}/policy ───────────────────────────────────────

func (h *CertHandlers) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var policy map[string]any
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid policy body"})
		return
	}
	if err := h.svc.UpdatePolicy(r.Context(), name, policy); err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app.PolicyJSON(policy, h.host(r), name))
}

// ─── GET /certificates ────────────────────────────────────────────────────────

func (h *CertHandlers) List(w http.ResponseWriter, r *http.Request) {
	list, next, err := h.svc.List(r.Context(), maxResults(r), r.URL.Query().Get("$skiptoken"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]any, len(list))
	for i, cv := range list {
		items[i] = map[string]any{
			"id":         domain.ID(h.host(r), "certificates", cv.Name, ""),
			"x5t":        cv.X5T,
			"attributes": cv.Attributes,
		}
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── GET /certificates/{name}/versions ───────────────────────────────────────

func (h *CertHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	list, next, err := h.svc.ListVersions(r.Context(), name, maxResults(r), r.URL.Query().Get("$skiptoken"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]any, len(list))
	for i, cv := range list {
		items[i] = map[string]any{
			"id":         domain.ID(h.host(r), "certificates", cv.Name, cv.Version),
			"x5t":        cv.X5T,
			"attributes": cv.Attributes,
		}
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── DELETE / recover / purge ─────────────────────────────────────────────────

func (h *CertHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	dc, err := h.svc.Delete(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                  domain.ID(h.host(r), "certificates", dc.Name, dc.Version),
		"recoveryId":          dc.RecoveryID,
		"deletedDate":         dc.DeletedDate,
		"scheduledPurgeDate":  dc.ScheduledPurge,
		"attributes":          dc.Attributes,
	})
}

func (h *CertHandlers) GetDeleted(w http.ResponseWriter, r *http.Request) {
	dc, err := h.svc.GetDeleted(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                 domain.ID(h.host(r), "certificates", dc.Name, dc.Version),
		"recoveryId":         dc.RecoveryID,
		"deletedDate":        dc.DeletedDate,
		"scheduledPurgeDate": dc.ScheduledPurge,
		"cer":                base64.StdEncoding.EncodeToString(dc.CerDER),
		"attributes":         dc.Attributes,
	})
}

func (h *CertHandlers) ListDeleted(w http.ResponseWriter, r *http.Request) {
	list, next, err := h.svc.ListDeleted(r.Context(), maxResults(r), r.URL.Query().Get("$skiptoken"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]any, len(list))
	for i, dc := range list {
		items[i] = map[string]any{
			"id": domain.ID(h.host(r), "certificates", dc.Name, dc.Version),
			"recoveryId": dc.RecoveryID, "deletedDate": dc.DeletedDate,
		}
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

func (h *CertHandlers) Recover(w http.ResponseWriter, r *http.Request) {
	cv, err := h.svc.Recover(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": domain.ID(h.host(r), "certificates", cv.Name, cv.Version),
		"attributes": cv.Attributes,
	})
}

func (h *CertHandlers) Purge(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Purge(r.Context(), chi.URLParam(r, "name")); err != nil {
		middleware.DomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Contacts / Issuers (persistent) ──────────────────────────────────────────

func (h *CertHandlers) ContactsGet(w http.ResponseWriter, r *http.Request) {
	contacts, err := h.svc.GetContacts(r.Context())
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          "https://" + h.host(r) + "/certificates/contacts",
		"contactList": contacts,
	})
}

func (h *CertHandlers) ContactsSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ContactList []map[string]any `json:"contactList"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid body"})
		return
	}
	if err := h.svc.SetContacts(r.Context(), body.ContactList); err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          "https://" + h.host(r) + "/certificates/contacts",
		"contactList": body.ContactList,
	})
}

func (h *CertHandlers) ContactsDelete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteContacts(r.Context()); err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          "https://" + h.host(r) + "/certificates/contacts",
		"contactList": []any{},
	})
}

func (h *CertHandlers) IssuerGet(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	iss, err := h.svc.GetIssuer(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issuerToJSON(iss, h.host(r)))
}

func (h *CertHandlers) IssuerSet(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var body struct {
		Provider    string         `json:"provider"`
		Credentials map[string]any `json:"credentials"`
		OrgDetails  map[string]any `json:"org_details"`
		Attributes  map[string]any `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid body"})
		return
	}
	iss, err := h.svc.SetIssuer(r.Context(), name, body.Provider, body.Credentials, body.OrgDetails, body.Attributes)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issuerToJSON(iss, h.host(r)))
}

func (h *CertHandlers) IssuerDelete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	iss, err := h.svc.DeleteIssuer(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issuerToJSON(iss, h.host(r)))
}

func issuerToJSON(iss *sqlite.IssuerData, vaultHost string) map[string]any {
	out := map[string]any{
		"id":       domain.ID(vaultHost, "certificates/issuers", iss.Name, ""),
		"provider": iss.Provider,
	}
	if iss.Credentials != nil {
		out["credentials"] = iss.Credentials
	}
	if iss.OrgDetails != nil {
		out["org_details"] = iss.OrgDetails
	}
	if iss.Attributes != nil {
		out["attributes"] = iss.Attributes
	}
	return out
}
