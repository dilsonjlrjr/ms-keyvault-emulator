package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
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
		"id":            domain.ID(h.vaultHost, "certificates", name, "") + "/pending",
		"issuer":        map[string]string{"name": "Self"},
		"csr":           base64.StdEncoding.EncodeToString(cv.CerDER),
		"status":        "completed",
		"target":        domain.ID(h.vaultHost, "certificates", name, cv.Version),
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
	writeJSON(w, http.StatusOK, app.CertToJSON(cv, body.Policy, h.vaultHost))
}

// ─── GET /certificates/{name}[/{version}] ────────────────────────────────────

func (h *CertHandlers) Get(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	cv, policy, err := h.svc.Get(r.Context(), name, version)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app.CertToJSON(cv, policy, h.vaultHost))
}

// ─── GET /certificates/{name}/policy ─────────────────────────────────────────

func (h *CertHandlers) GetPolicy(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	policy, err := h.svc.GetPolicy(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app.PolicyJSON(policy, h.vaultHost, name))
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
	writeJSON(w, http.StatusOK, app.PolicyJSON(policy, h.vaultHost, name))
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
			"id":         domain.ID(h.vaultHost, "certificates", cv.Name, ""),
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
			"id":         domain.ID(h.vaultHost, "certificates", cv.Name, cv.Version),
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
		"id":                  domain.ID(h.vaultHost, "certificates", dc.Name, dc.Version),
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
		"id":                 domain.ID(h.vaultHost, "certificates", dc.Name, dc.Version),
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
			"id": domain.ID(h.vaultHost, "certificates", dc.Name, dc.Version),
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
		"id": domain.ID(h.vaultHost, "certificates", cv.Name, cv.Version),
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

// ─── Contacts / Issuers (stubs MVP) ──────────────────────────────────────────

func (h *CertHandlers) ContactsGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"id": "https://" + h.vaultHost + "/certificates/contacts", "contactList": []any{}})
}
func (h *CertHandlers) ContactsSet(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, http.StatusOK, body)
}
func (h *CertHandlers) ContactsDelete(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"contactList": []any{}})
}
func (h *CertHandlers) IssuerGet(w http.ResponseWriter, r *http.Request) {
	middleware.DomainError(w, domain.ErrNotFound{Kind: "CertificateIssuer", Name: chi.URLParam(r, "name")})
}
func (h *CertHandlers) IssuerSet(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, http.StatusOK, body)
}
func (h *CertHandlers) IssuerDelete(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{})
}
