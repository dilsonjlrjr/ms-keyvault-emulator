package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
	"github.com/dilsonrabelo/kvemu/internal/app"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

// SecretHandlers expõe todos os handlers de secrets como métodos
// para permitir wiring com a dependência do SecretService.
type SecretHandlers struct {
	svc       *app.SecretService
	vaultHost string
}

func NewSecretHandlers(svc *app.SecretService, vaultHost string) *SecretHandlers {
	return &SecretHandlers{svc: svc, vaultHost: vaultHost}
}

// ─── response shapes ──────────────────────────────────────────────────────────

type secretBundleResp struct {
	ID          string            `json:"id"`
	Value       string            `json:"value,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Managed     bool              `json:"managed,omitempty"`
	Attributes  domain.Attributes `json:"attributes"`
}

type deletedSecretResp struct {
	secretBundleResp
	RecoveryID     string `json:"recoveryId,omitempty"`
	DeletedDate    int64  `json:"deletedDate,omitempty"`
	ScheduledPurge int64  `json:"scheduledPurgeDate,omitempty"`
}

type listResp struct {
	Value    []any   `json:"value"`
	NextLink *string `json:"nextLink,omitempty"`
}

func (h *SecretHandlers) host(r *http.Request) string {
	return vaultHostFromContext(r, h.vaultHost)
}

func (h *SecretHandlers) toBundle(sv *domain.SecretVersion, host string) secretBundleResp {
	return secretBundleResp{
		ID:          domain.ID(host, "secrets", sv.Name, sv.Version),
		Value:       sv.Value,
		ContentType: sv.ContentType,
		Tags:        sv.Tags,
		Managed:     sv.Managed,
		Attributes:  sv.Attributes,
	}
}

func (h *SecretHandlers) toBundleNoValue(sv *domain.SecretVersion, host string) secretBundleResp {
	b := h.toBundle(sv, host)
	b.Value = ""
	b.ID = domain.ID(host, "secrets", sv.Name, sv.Version)
	return b
}

func (h *SecretHandlers) toDeleted(ds *domain.DeletedSecret, host string) deletedSecretResp {
	return deletedSecretResp{
		secretBundleResp: h.toBundle(&ds.SecretVersion, host),
		RecoveryID:       ds.RecoveryID,
		DeletedDate:      ds.DeletedDate,
		ScheduledPurge:   ds.ScheduledPurge,
	}
}

func nextLink(r *http.Request, token string) *string {
	if token == "" {
		return nil
	}
	q := r.URL.Query()
	q.Set("$skiptoken", token)
	u := *r.URL
	u.RawQuery = q.Encode()
	s := "https://" + r.Host + u.String()
	return &s
}

func maxResults(r *http.Request) int {
	v := r.URL.Query().Get("maxresults")
	if v == "" {
		return 25
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 25
	}
	return n
}

// ─── PUT /secrets/{name} ──────────────────────────────────────────────────────

func (h *SecretHandlers) Set(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Value       string            `json:"value"`
		ContentType string            `json:"contentType"`
		Tags        map[string]string `json:"tags"`
		Attributes  struct {
			Enabled   *bool  `json:"enabled"`
			NotBefore *int64 `json:"nbf"`
			Expires   *int64 `json:"exp"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid JSON body"})
		return
	}
	if body.Value == "" {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "value is required"})
		return
	}

	attrs := domain.Attributes{
		Enabled:       true,
		NotBefore:     body.Attributes.NotBefore,
		Expires:       body.Attributes.Expires,
		RecoveryLevel: domain.RecoveryLevelPurgeable,
	}
	if body.Attributes.Enabled != nil {
		attrs.Enabled = *body.Attributes.Enabled
	}

	sv, err := h.svc.Set(r.Context(), name, body.Value, body.ContentType, body.Tags, attrs)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(sv, h.host(r)))
}

// ─── GET /secrets/{name}[/{version}] ─────────────────────────────────────────

func (h *SecretHandlers) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	sv, err := h.svc.Get(r.Context(), name, version)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(sv, h.host(r)))
}

// ─── GET /secrets ─────────────────────────────────────────────────────────────

func (h *SecretHandlers) List(w http.ResponseWriter, r *http.Request) {
	max := maxResults(r)
	tok := r.URL.Query().Get("$skiptoken")

	list, next, err := h.svc.List(r.Context(), max, tok)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}

	items := make([]any, len(list))
	for i, sv := range list {
		items[i] = h.toBundleNoValue(sv, h.host(r))
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── GET /secrets/{name}/versions ────────────────────────────────────────────

func (h *SecretHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	max := maxResults(r)
	tok := r.URL.Query().Get("$skiptoken")

	list, next, err := h.svc.ListVersions(r.Context(), name, max, tok)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}

	items := make([]any, len(list))
	for i, sv := range list {
		b := h.toBundle(sv, h.host(r))
		b.ID = domain.ID(h.host(r), "secrets", sv.Name, sv.Version) // versões incluem o version no id
		items[i] = b
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── PATCH /secrets/{name}/{version} ─────────────────────────────────────────

func (h *SecretHandlers) Update(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")

	var body struct {
		ContentType *string           `json:"contentType"`
		Tags        map[string]string `json:"tags"`
		Attributes  struct {
			Enabled   *bool  `json:"enabled"`
			NotBefore *int64 `json:"nbf"`
			Expires   *int64 `json:"exp"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid JSON body"})
		return
	}

	attrs := domain.Attributes{Enabled: true}
	if body.Attributes.Enabled != nil {
		attrs.Enabled = *body.Attributes.Enabled
	}
	attrs.NotBefore = body.Attributes.NotBefore
	attrs.Expires = body.Attributes.Expires

	sv, err := h.svc.Update(r.Context(), name, version, attrs, body.ContentType, body.Tags)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(sv, h.host(r)))
}

// ─── DELETE /secrets/{name} ───────────────────────────────────────────────────

func (h *SecretHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ds, err := h.svc.Delete(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toDeleted(ds, h.host(r)))
}

// ─── GET /deletedsecrets/{name} ───────────────────────────────────────────────

func (h *SecretHandlers) GetDeleted(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ds, err := h.svc.GetDeleted(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toDeleted(ds, h.host(r)))
}

// ─── GET /deletedsecrets ──────────────────────────────────────────────────────

func (h *SecretHandlers) ListDeleted(w http.ResponseWriter, r *http.Request) {
	max := maxResults(r)
	tok := r.URL.Query().Get("$skiptoken")

	list, next, err := h.svc.ListDeleted(r.Context(), max, tok)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}

	items := make([]any, len(list))
	for i, ds := range list {
		items[i] = h.toDeleted(ds, h.host(r))
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── POST /deletedsecrets/{name}/recover ──────────────────────────────────────

func (h *SecretHandlers) Recover(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	sv, err := h.svc.Recover(r.Context(), name)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(sv, h.host(r)))
}

// ─── DELETE /deletedsecrets/{name} ────────────────────────────────────────────

func (h *SecretHandlers) Purge(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.svc.Purge(r.Context(), name); err != nil {
		middleware.DomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── backup/restore (MVP: blob opaco simples) ─────────────────────────────────

func (h *SecretHandlers) Backup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	sv, err := h.svc.Get(r.Context(), name, "")
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	// blob opaco: JSON em base64 (MVP — sem assinatura real)
	b, _ := json.Marshal(sv)
	writeJSON(w, http.StatusOK, map[string]string{
		"value": fmt.Sprintf("%x", b),
	})
}

func (h *SecretHandlers) Restore(w http.ResponseWriter, r *http.Request) {
	// MVP: não implementado (retorna 501 com envelope Azure)
	middleware.DomainError(w, domain.ErrBadParam{Msg: "restore not yet implemented"})
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
