package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
)

// RouterConfig agrupa tudo que o router precisa.
type RouterConfig struct {
	VaultHost  string
	TenantID   string
	AADKey     *kvCrypto.AADKey
	AuthStrict bool
	Secrets    *SecretHandlers
	Keys       *KeyHandlers
	Certs      *CertHandlers
	// AuditFn é chamada após cada requisição do data-plane. Pode ser nil.
	AuditFn func(actor, op, resource string, status int, reqID string)
}

func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(kvmsHeaders)
	r.Use(middleware.Logger)

	// health — sem auth
	r.Get("/healthz", healthz)

	// AAD fake — sem auth (são os endpoints de token/discovery)
	aad := newAADHandler(cfg.AADKey, cfg.VaultHost)
	r.Get("/{tenant}/discovery/instance", aad.instanceDiscovery)
	r.Get("/{tenant}/v2.0/.well-known/openid-configuration", aad.oidcV2)
	r.Get("/{tenant}/.well-known/openid-configuration", aad.oidcV1)
	r.Get("/{tenant}/discovery/v2.0/keys", aad.jwks)
	r.Post("/{tenant}/oauth2/v2.0/token", aad.tokenV2)
	r.Post("/{tenant}/oauth2/token", aad.tokenV1)

	// data-plane — challenge + auth + audit obrigatórios
	r.Group(func(r chi.Router) {
		r.Use(middleware.Challenge(cfg.VaultHost, cfg.TenantID))
		r.Use(middleware.Auth(cfg.AADKey, cfg.AuthStrict, cfg.VaultHost))
		if cfg.AuditFn != nil {
			r.Use(middleware.Audit(cfg.AuditFn))
		}

		// Secrets
		if cfg.Secrets != nil {
			sh := cfg.Secrets
			r.Route("/secrets", func(r chi.Router) {
				r.Get("/", sh.List)
				r.Put("/{name}", sh.Set)
				r.Post("/restore", sh.Restore)
				r.Post("/{name}/backup", sh.Backup)
				r.Get("/{name}/versions", sh.ListVersions)
				r.Get("/{name}/{version}", sh.Get)
				r.Get("/{name}", sh.Get)
				r.Patch("/{name}/{version}", sh.Update)
				r.Delete("/{name}", sh.Delete)
			})
			r.Route("/deletedsecrets", func(r chi.Router) {
				r.Get("/", sh.ListDeleted)
				r.Get("/{name}", sh.GetDeleted)
				r.Post("/{name}/recover", sh.Recover)
				r.Delete("/{name}", sh.Purge)
			})
		} else {
			r.Route("/secrets", func(r chi.Router) {
				r.Get("/", secretsListHandler)
				r.Put("/{name}", secretSetHandler)
				r.Get("/{name}", secretGetHandler)
				r.Get("/{name}/{version}", secretGetHandler)
				r.Get("/{name}/versions", secretListVersionsHandler)
				r.Patch("/{name}/{version}", secretUpdateHandler)
				r.Delete("/{name}", secretDeleteHandler)
				r.Post("/{name}/backup", secretBackupHandler)
				r.Post("/restore", secretRestoreHandler)
			})
			r.Route("/deletedsecrets", func(r chi.Router) {
				r.Get("/", deletedSecretsListHandler)
				r.Get("/{name}", deletedSecretGetHandler)
				r.Post("/{name}/recover", secretRecoverHandler)
				r.Delete("/{name}", secretPurgeHandler)
			})
		}

		// Keys
		if cfg.Keys != nil {
			kh := cfg.Keys
			r.Route("/keys", func(r chi.Router) {
				r.Get("/", kh.List)
				r.Post("/{name}/create", kh.Create)
				r.Put("/{name}", kh.Import)
				r.Get("/{name}/versions", kh.ListVersions)
				r.Get("/{name}/{version}", kh.Get)
				r.Get("/{name}", kh.Get)
				r.Patch("/{name}/{version}", kh.Update)
				r.Delete("/{name}", kh.Delete)
				r.Post("/{name}/{version}/encrypt", kh.Encrypt)
				r.Post("/{name}/{version}/decrypt", kh.Decrypt)
				r.Post("/{name}/{version}/sign", kh.Sign)
				r.Post("/{name}/{version}/verify", kh.Verify)
				r.Post("/{name}/{version}/wrapkey", kh.WrapKey)
				r.Post("/{name}/{version}/unwrapkey", kh.UnwrapKey)
			})
			r.Route("/deletedkeys", func(r chi.Router) {
				r.Get("/", kh.ListDeleted)
				r.Get("/{name}", kh.GetDeleted)
				r.Post("/{name}/recover", kh.Recover)
				r.Delete("/{name}", kh.Purge)
			})
		} else {
			r.Route("/keys", func(r chi.Router) {
				r.Get("/", keysListHandler)
				r.Post("/{name}/create", keyCreateHandler)
				r.Put("/{name}", keyImportHandler)
				r.Get("/{name}", keyGetHandler)
				r.Get("/{name}/{version}", keyGetHandler)
				r.Get("/{name}/versions", keyListVersionsHandler)
				r.Patch("/{name}/{version}", keyUpdateHandler)
				r.Delete("/{name}", keyDeleteHandler)
				r.Post("/{name}/{version}/encrypt", keyEncryptHandler)
				r.Post("/{name}/{version}/decrypt", keyDecryptHandler)
				r.Post("/{name}/{version}/sign", keySignHandler)
				r.Post("/{name}/{version}/verify", keyVerifyHandler)
				r.Post("/{name}/{version}/wrapkey", keyWrapHandler)
				r.Post("/{name}/{version}/unwrapkey", keyUnwrapHandler)
			})
			r.Route("/deletedkeys", func(r chi.Router) {
				r.Get("/", deletedKeysListHandler)
				r.Get("/{name}", deletedKeyGetHandler)
				r.Post("/{name}/recover", keyRecoverHandler)
				r.Delete("/{name}", keyPurgeHandler)
			})
		}

		// Certificates
		if cfg.Certs != nil {
			ch := cfg.Certs
			r.Route("/certificates", func(r chi.Router) {
				r.Get("/", ch.List)
				r.Get("/contacts", ch.ContactsGet)
				r.Put("/contacts", ch.ContactsSet)
				r.Delete("/contacts", ch.ContactsDelete)
				r.Get("/issuers/{name}", ch.IssuerGet)
				r.Put("/issuers/{name}", ch.IssuerSet)
				r.Delete("/issuers/{name}", ch.IssuerDelete)
				r.Post("/{name}/create", ch.Create)
				r.Put("/{name}/import", ch.Import)
				r.Get("/{name}/versions", ch.ListVersions)
				r.Get("/{name}/policy", ch.GetPolicy)
				r.Patch("/{name}/policy", ch.UpdatePolicy)
				r.Get("/{name}/{version}", ch.Get)
				r.Get("/{name}", ch.Get)
				r.Delete("/{name}", ch.Delete)
			})
			r.Route("/deletedcertificates", func(r chi.Router) {
				r.Get("/", ch.ListDeleted)
				r.Get("/{name}", ch.GetDeleted)
				r.Post("/{name}/recover", ch.Recover)
				r.Delete("/{name}", ch.Purge)
			})
		} else {
			r.Route("/certificates", func(r chi.Router) {
				r.Get("/", certsListHandler)
				r.Post("/{name}/create", certCreateHandler)
				r.Put("/{name}/import", certImportHandler)
				r.Get("/{name}", certGetHandler)
				r.Get("/{name}/{version}", certGetHandler)
				r.Get("/{name}/versions", certListVersionsHandler)
				r.Get("/{name}/policy", certGetPolicyHandler)
				r.Patch("/{name}/policy", certUpdatePolicyHandler)
				r.Delete("/{name}", certDeleteHandler)
				r.Get("/contacts", certContactsGetHandler)
				r.Put("/contacts", certContactsSetHandler)
				r.Delete("/contacts", certContactsDeleteHandler)
				r.Get("/issuers/{name}", certIssuerGetHandler)
				r.Put("/issuers/{name}", certIssuerSetHandler)
				r.Delete("/issuers/{name}", certIssuerDeleteHandler)
			})
			r.Route("/deletedcertificates", func(r chi.Router) {
				r.Get("/", deletedCertsListHandler)
				r.Get("/{name}", deletedCertGetHandler)
				r.Post("/{name}/recover", certRecoverHandler)
				r.Delete("/{name}", certPurgeHandler)
			})
		}
	})

	return r
}

func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func kvmsHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := chiMiddleware.GetReqID(r.Context())
		w.Header().Set("x-ms-request-id", reqID)
		w.Header().Set("x-ms-keyvault-region", "emulator")
		w.Header().Set("x-ms-keyvault-service-version", "1.0")
		next.ServeHTTP(w, r)
	})
}

// ─── stubs (implementados nas fases seguintes) ───────────────────────────────

func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": "NotImplemented", "message": "not implemented yet"},
	})
}

var (
	secretSetHandler          = notImplemented
	secretGetHandler          = notImplemented
	secretsListHandler        = notImplemented
	secretListVersionsHandler = notImplemented
	secretUpdateHandler       = notImplemented
	secretDeleteHandler       = notImplemented
	secretBackupHandler       = notImplemented
	secretRestoreHandler      = notImplemented
	deletedSecretsListHandler = notImplemented
	deletedSecretGetHandler   = notImplemented
	secretRecoverHandler      = notImplemented
	secretPurgeHandler        = notImplemented

	keysListHandler        = notImplemented
	keyCreateHandler       = notImplemented
	keyImportHandler       = notImplemented
	keyGetHandler          = notImplemented
	keyListVersionsHandler = notImplemented
	keyUpdateHandler       = notImplemented
	keyDeleteHandler       = notImplemented
	keyEncryptHandler      = notImplemented
	keyDecryptHandler      = notImplemented
	keySignHandler         = notImplemented
	keyVerifyHandler       = notImplemented
	keyWrapHandler         = notImplemented
	keyUnwrapHandler       = notImplemented
	deletedKeysListHandler = notImplemented
	deletedKeyGetHandler   = notImplemented
	keyRecoverHandler      = notImplemented
	keyPurgeHandler        = notImplemented

	certsListHandler        = notImplemented
	certCreateHandler       = notImplemented
	certImportHandler       = notImplemented
	certGetHandler          = notImplemented
	certListVersionsHandler = notImplemented
	certGetPolicyHandler    = notImplemented
	certUpdatePolicyHandler = notImplemented
	certDeleteHandler       = notImplemented
	certContactsGetHandler  = notImplemented
	certContactsSetHandler  = notImplemented
	certContactsDeleteHandler = notImplemented
	certIssuerGetHandler    = notImplemented
	certIssuerSetHandler    = notImplemented
	certIssuerDeleteHandler = notImplemented
	deletedCertsListHandler = notImplemented
	deletedCertGetHandler   = notImplemented
	certRecoverHandler      = notImplemented
	certPurgeHandler        = notImplemented
)
