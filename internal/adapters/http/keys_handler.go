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

type KeyHandlers struct {
	svc       *app.KeyService
	vaultHost string
}

func NewKeyHandlers(svc *app.KeyService, vaultHost string) *KeyHandlers {
	return &KeyHandlers{svc: svc, vaultHost: vaultHost}
}

// ─── response shapes ──────────────────────────────────────────────────────────

type keyBundleResp struct {
	Key        map[string]any    `json:"key"`
	Attributes domain.Attributes `json:"attributes"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type deletedKeyResp struct {
	keyBundleResp
	RecoveryID     string `json:"recoveryId,omitempty"`
	DeletedDate    int64  `json:"deletedDate,omitempty"`
	ScheduledPurge int64  `json:"scheduledPurgeDate,omitempty"`
}

func (h *KeyHandlers) host(r *http.Request) string {
	return vaultHostFromContext(r, h.vaultHost)
}

func (h *KeyHandlers) toBundle(kv *domain.KeyVersion, host string) keyBundleResp {
	pub := make(map[string]any)
	for k, v := range kv.PubJWK {
		pub[k] = v
	}
	pub["kid"] = domain.ID(host, "keys", kv.Name, kv.Version)
	pub["key_ops"] = kv.KeyOps
	return keyBundleResp{Key: pub, Attributes: kv.Attributes}
}

func (h *KeyHandlers) toDeleted(dk *domain.DeletedKey, host string) deletedKeyResp {
	return deletedKeyResp{
		keyBundleResp:  h.toBundle(&dk.KeyVersion, host),
		RecoveryID:     dk.RecoveryID,
		DeletedDate:    dk.DeletedDate,
		ScheduledPurge: dk.ScheduledPurge,
	}
}

// ─── POST /keys/{name}/create ─────────────────────────────────────────────────

func (h *KeyHandlers) Create(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Kty     string            `json:"kty"`
		KeySize int               `json:"key_size"`
		Crv     string            `json:"crv"`
		KeyOps  []string          `json:"key_ops"`
		Tags    map[string]string `json:"tags"`
		Attrs   struct {
			Enabled   *bool  `json:"enabled"`
			NotBefore *int64 `json:"nbf"`
			Expires   *int64 `json:"exp"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid JSON body"})
		return
	}
	if body.Kty == "" {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "kty is required"})
		return
	}

	attrs := domain.Attributes{Enabled: true, NotBefore: body.Attrs.NotBefore, Expires: body.Attrs.Expires}
	if body.Attrs.Enabled != nil {
		attrs.Enabled = *body.Attrs.Enabled
	}

	kv, err := h.svc.Create(r.Context(), name, body.Kty, body.Crv, body.KeySize, body.KeyOps, attrs)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(kv, h.host(r)))
}

// ─── PUT /keys/{name} ─────────────────────────────────────────────────────────

func (h *KeyHandlers) Import(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Key    map[string]any    `json:"key"`
		KeyOps []string          `json:"key_ops"`
		Tags   map[string]string `json:"tags"`
		Attrs  struct {
			Enabled   *bool  `json:"enabled"`
			NotBefore *int64 `json:"nbf"`
			Expires   *int64 `json:"exp"`
		} `json:"attributes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid JSON body — key required"})
		return
	}

	attrs := domain.Attributes{Enabled: true, NotBefore: body.Attrs.NotBefore, Expires: body.Attrs.Expires}
	if body.Attrs.Enabled != nil {
		attrs.Enabled = *body.Attrs.Enabled
	}

	kv, err := h.svc.Import(r.Context(), name, body.Key, body.KeyOps, attrs)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(kv, h.host(r)))
}

// ─── GET /keys/{name}[/{version}] ─────────────────────────────────────────────

func (h *KeyHandlers) Get(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	version := chi.URLParam(r, "version")
	kv, err := h.svc.Get(r.Context(), name, version)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(kv, h.host(r)))
}

// ─── GET /keys ────────────────────────────────────────────────────────────────

func (h *KeyHandlers) List(w http.ResponseWriter, r *http.Request) {
	list, next, err := h.svc.List(r.Context(), maxResults(r), r.URL.Query().Get("$skiptoken"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]any, len(list))
	host := h.host(r)
	for i, kv := range list {
		b := h.toBundle(kv, host)
		b.Key["kid"] = domain.ID(host, "keys", kv.Name, "")
		items[i] = b
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── GET /keys/{name}/versions ───────────────────────────────────────────────

func (h *KeyHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	list, next, err := h.svc.ListVersions(r.Context(), name, maxResults(r), r.URL.Query().Get("$skiptoken"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]any, len(list))
	for i, kv := range list {
		items[i] = h.toBundle(kv, h.host(r))
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

// ─── PATCH /keys/{name}/{version} ────────────────────────────────────────────

func (h *KeyHandlers) Update(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		KeyOps []string          `json:"key_ops"`
		Tags   map[string]string `json:"tags"`
		Attrs  struct {
			Enabled   *bool  `json:"enabled"`
			NotBefore *int64 `json:"nbf"`
			Expires   *int64 `json:"exp"`
		} `json:"attributes"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	attrs := domain.Attributes{Enabled: true, NotBefore: body.Attrs.NotBefore, Expires: body.Attrs.Expires}
	if body.Attrs.Enabled != nil {
		attrs.Enabled = *body.Attrs.Enabled
	}

	kv, err := h.svc.Update(r.Context(), name, version, attrs, body.KeyOps, body.Tags)
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(kv, h.host(r)))
}

// ─── DELETE /keys/{name} ─────────────────────────────────────────────────────

func (h *KeyHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	dk, err := h.svc.Delete(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toDeleted(dk, h.host(r)))
}

func (h *KeyHandlers) GetDeleted(w http.ResponseWriter, r *http.Request) {
	dk, err := h.svc.GetDeleted(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toDeleted(dk, h.host(r)))
}

func (h *KeyHandlers) ListDeleted(w http.ResponseWriter, r *http.Request) {
	list, next, err := h.svc.ListDeleted(r.Context(), maxResults(r), r.URL.Query().Get("$skiptoken"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	items := make([]any, len(list))
	for i, dk := range list {
		items[i] = h.toDeleted(dk, h.host(r))
	}
	writeJSON(w, http.StatusOK, listResp{Value: items, NextLink: nextLink(r, next)})
}

func (h *KeyHandlers) Recover(w http.ResponseWriter, r *http.Request) {
	kv, err := h.svc.Recover(r.Context(), chi.URLParam(r, "name"))
	if err != nil {
		middleware.DomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.toBundle(kv, h.host(r)))
}

func (h *KeyHandlers) Purge(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Purge(r.Context(), chi.URLParam(r, "name")); err != nil {
		middleware.DomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── operações criptográficas ─────────────────────────────────────────────────

func (h *KeyHandlers) Encrypt(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		Alg   string `json:"alg"`
		Value string `json:"value"` // base64url
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid body"}); return
	}
	plain, _ := base64.RawURLEncoding.DecodeString(body.Value)
	ct, ver, err := h.svc.Encrypt(r.Context(), name, version, body.Alg, plain)
	if err != nil {
		middleware.DomainError(w, err); return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"kid":   	domain.ID(h.host(r), "keys", name, ver),
		"value": base64.RawURLEncoding.EncodeToString(ct),
	})
}

func (h *KeyHandlers) Decrypt(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		Alg   string `json:"alg"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		middleware.DomainError(w, domain.ErrBadParam{Msg: "invalid body"}); return
	}
	ct, _ := base64.RawURLEncoding.DecodeString(body.Value)
	plain, ver, err := h.svc.Decrypt(r.Context(), name, version, body.Alg, ct)
	if err != nil {
		middleware.DomainError(w, err); return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"kid":   	domain.ID(h.host(r), "keys", name, ver),
		"value": base64.RawURLEncoding.EncodeToString(plain),
	})
}

func (h *KeyHandlers) Sign(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		Alg   string `json:"alg"`
		Value string `json:"value"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	data, _ := base64.RawURLEncoding.DecodeString(body.Value)
	sig, ver, err := h.svc.Sign(r.Context(), name, version, body.Alg, data)
	if err != nil {
		middleware.DomainError(w, err); return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"kid":   	domain.ID(h.host(r), "keys", name, ver),
		"value": base64.RawURLEncoding.EncodeToString(sig),
	})
}

func (h *KeyHandlers) Verify(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		Alg       string `json:"alg"`
		Digest    string `json:"digest"` // SHA-256 do dado original, em base64url
		Signature string `json:"value"`  // assinatura a verificar, em base64url
	}
	json.NewDecoder(r.Body).Decode(&body)
	data, _ := base64.RawURLEncoding.DecodeString(body.Digest)
	sig, _ := base64.RawURLEncoding.DecodeString(body.Signature)
	ok, _, err := h.svc.Verify(r.Context(), name, version, body.Alg, data, sig)
	if err != nil {
		middleware.DomainError(w, err); return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"value": ok})
}

func (h *KeyHandlers) WrapKey(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		Alg   string `json:"alg"`
		Value string `json:"value"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	km, _ := base64.RawURLEncoding.DecodeString(body.Value)
	ct, ver, err := h.svc.WrapKey(r.Context(), name, version, body.Alg, km)
	if err != nil {
		middleware.DomainError(w, err); return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"kid":   	domain.ID(h.host(r), "keys", name, ver),
		"value": base64.RawURLEncoding.EncodeToString(ct),
	})
}

func (h *KeyHandlers) UnwrapKey(w http.ResponseWriter, r *http.Request) {
	name, version := chi.URLParam(r, "name"), chi.URLParam(r, "version")
	var body struct {
		Alg   string `json:"alg"`
		Value string `json:"value"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	wrapped, _ := base64.RawURLEncoding.DecodeString(body.Value)
	plain, ver, err := h.svc.UnwrapKey(r.Context(), name, version, body.Alg, wrapped)
	if err != nil {
		middleware.DomainError(w, err); return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"kid":   	domain.ID(h.host(r), "keys", name, ver),
		"value": base64.RawURLEncoding.EncodeToString(plain),
	})
}
