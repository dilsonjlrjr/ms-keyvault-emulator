package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	kvHTTP "github.com/dilsonrabelo/kvemu/internal/adapters/http"
	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/app"
	"github.com/dilsonrabelo/kvemu/internal/config"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if len(os.Args) < 2 {
		runServer()
		return
	}

	switch os.Args[1] {
	case "healthcheck":
		addr := "https://localhost:13000"
		for i, a := range os.Args {
			if a == "--addr" && i+1 < len(os.Args) {
				addr = os.Args[i+1]
			}
		}
		runHealthcheck(addr)
	case "ca":
		if len(os.Args) < 3 || os.Args[2] != "export" {
			fmt.Fprintln(os.Stderr, "usage: kvemu ca export [--out <path>]")
			os.Exit(1)
		}
		runCAExport()
	case "seed":
		runSeed()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\nusage: kvemu [healthcheck|ca export|seed]\n", os.Args[1])
		os.Exit(1)
	}
}

// ─── stack compartilhado ───────────────────────────────────────────────────────

type stack struct {
	db         *sql.DB
	secretSvc  *app.SecretService
	keySvc     *app.KeyService
	certSvc    *app.CertService
	auditRepo  *sqlite.AuditRepo
	vaultRepo  *sqlite.VaultRepo
	vaultSvc   *app.VaultService
	secretRepo *sqlite.SecretRepo
	keyRepo    *sqlite.KeyRepo
	certRepo   *sqlite.CertRepo
	cfg        config.Config
}

func buildStack() *stack {
	cfg := config.Load()

	db, err := sqlite.Open(cfg.DataPath)
	mustOK(err, "sqlite open")
	mustOK(migrate(db, migrationsFS), "migrate")

	vaultRepo := sqlite.NewVaultRepo(db)
	vaultSvc := app.NewVaultService(vaultRepo, cfg.BaseDomain, portFromAddr(cfg.Addr))
	mustOK(ensureDefaultVault(vaultSvc, cfg.DefaultVault, cfg.TenantID), "ensure default vault")

	vaultName := cfg.DefaultVault

	secretRepo := sqlite.NewSecretRepo(db, cfg.MasterKey)
	secretSvc := app.NewSecretService(secretRepo, vaultName)

	keyRepo := sqlite.NewKeyRepo(db, cfg.MasterKey)
	keySvc := app.NewKeyService(keyRepo, vaultName)

	certRepo := sqlite.NewCertRepo(db)
	certSvc := app.NewCertService(certRepo, secretSvc, keySvc, vaultName)

	auditRepo := sqlite.NewAuditRepo(db)

	return &stack{
		db:         db,
		secretSvc:  secretSvc,
		keySvc:     keySvc,
		certSvc:    certSvc,
		auditRepo:  auditRepo,
		vaultRepo:  vaultRepo,
		vaultSvc:   vaultSvc,
		secretRepo: secretRepo,
		keyRepo:    keyRepo,
		certRepo:   certRepo,
		cfg:        cfg,
	}
}

// ─── servidor principal ────────────────────────────────────────────────────────

func runServer() {
	cfg := config.Load()
	slog.Info("kvemu starting", "version", version, "addr", cfg.Addr, "vault", cfg.VaultHost)

	s := buildStack()
	defer s.db.Close()

	// purge scheduler — purga soft-deleted vencidos a cada hora
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.StartPurgeScheduler(ctx, s.db, time.Hour)

	aadKey, err := kvCrypto.NewAADKey()
	mustOK(err, "aad key gen")

	tlsBundle := loadTLS(cfg)

	auditFn := func(actor, op, resource string, status int, reqID string) {
		s.auditRepo.Log(context.Background(), actor, op, resource, status, reqID)
	}

	router := kvHTTP.NewRouter(kvHTTP.RouterConfig{
		AADKey:       aadKey,
		AuthStrict:   cfg.AuthStrict,
		AuditFn:      auditFn,
		VaultRepo:    s.vaultRepo,
		BaseDomain:   cfg.BaseDomain,
		DefaultVault: cfg.DefaultVault,
		Secrets:      kvHTTP.NewSecretHandlers(s.secretSvc, vaultHostFromCtx(s.cfg)),
		Keys:         kvHTTP.NewKeyHandlers(s.keySvc, vaultHostFromCtx(s.cfg)),
		Certs:        kvHTTP.NewCertHandlers(s.certSvc, vaultHostFromCtx(s.cfg)),
		Vaults:       kvHTTP.NewVaultHandlers(s.vaultSvc, s.secretRepo, s.keyRepo, s.certRepo),
	})

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: router,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsBundle.Cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	slog.Info("kvemu ready",
		"https", "https://"+cfg.VaultHost,
		"tenant", cfg.TenantID,
		"auth_strict", cfg.AuthStrict,
	)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

// ─── ca export ────────────────────────────────────────────────────────────────

func runCAExport() {
	cfg := config.Load()
	caPath := filepath.Join(cfg.CertDir, "ca.crt")
	data, err := os.ReadFile(caPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ca export: %v\n(tip: start the server once with KV_TLS_AUTO=true to generate the CA)\n", err)
		os.Exit(1)
	}

	out := ""
	for i, a := range os.Args {
		if a == "--out" && i+1 < len(os.Args) {
			out = os.Args[i+1]
		}
	}
	if out != "" {
		if err := os.WriteFile(out, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "ca export write: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("CA exported to", out)
	} else {
		os.Stdout.Write(data)
	}
}

// ─── seed ─────────────────────────────────────────────────────────────────────

func runSeed() {
	s := buildStack()
	defer s.db.Close()

	ctx := context.Background()
	attrs := domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable}

	seeds := []struct{ name, value, ct string }{
		{"db-password", "sup3rS3cr3t!", "text/plain"},
		{"api-key", "sk-dev-1234567890abcdef", "text/plain"},
		{"connection-string", "Server=localhost;Database=devdb;User=sa;Password=dev!", "text/plain"},
		{"COSMO_DB_URL", "https://cosmos.example.com", "text/plain"},
	}
	for _, seed := range seeds {
		sv, err := s.secretSvc.Set(ctx, seed.name, seed.value, seed.ct, nil, attrs)
		if err != nil {
			slog.Warn("seed secret skip", "name", seed.name, "err", err)
			continue
		}
		slog.Info("seed secret", "name", sv.Name, "version", sv.Version)
	}

	keySeeds := []struct {
		name, kty string
		size       int
	}{
		{"rsa-key-2048", "RSA", 2048},
		{"rsa-key-4096", "RSA", 4096},
	}
	for _, ks := range keySeeds {
		kv, err := s.keySvc.Create(ctx, ks.name, ks.kty, "", ks.size,
			[]string{"sign", "verify", "encrypt", "decrypt"}, attrs)
		if err != nil {
			slog.Warn("seed key skip", "name", ks.name, "err", err)
			continue
		}
		slog.Info("seed key", "name", kv.Name, "kty", kv.Kty, "version", kv.Version)
	}

	fmt.Println("seed complete")
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func loadTLS(cfg config.Config) *kvCrypto.TLSBundle {
	var (
		bundle *kvCrypto.TLSBundle
		err    error
	)
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		bundle, err = kvCrypto.LoadBYO(cfg.TLSCert, cfg.TLSKey, "")
		mustOK(err, "load BYO cert")
		slog.Info("tls: using provided cert", "cert", cfg.TLSCert)
		return bundle
	}
	if cfg.TLSAuto {
		bundle, err = kvCrypto.GenerateOrLoad(cfg.CertDir, cfg.VaultHost, cfg.TLSSANs, cfg.BaseDomain)
		mustOK(err, "tls generate/load")
		if cfg.CAOut != "" {
			dir := filepath.Dir(cfg.CAOut)
			os.MkdirAll(dir, 0700)
			os.WriteFile(cfg.CAOut, bundle.CAPEM, 0644)
			slog.Info("tls: CA exported", "path", cfg.CAOut)
		}
		slog.Info("tls: auto cert ready", "cert_dir", cfg.CertDir)
		return bundle
	}
	slog.Error("TLS required: set KV_TLS_AUTO=true or provide KV_TLS_CERT+KV_TLS_KEY")
	os.Exit(1)
	return nil
}

func migrate(db *sql.DB, fsys embed.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (filename TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create _migrations: %w", err)
	}

	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return fmt.Errorf("migrations dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		var already int
		if err := db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE filename = ?", e.Name()).Scan(&already); err != nil {
			return fmt.Errorf("check migration %s: %w", e.Name(), err)
		}
		if already > 0 {
			slog.Debug("migration already applied, skipping", "file", e.Name())
			continue
		}

		data, err := fsys.ReadFile("migrations/" + e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err := db.Exec(string(data)); err != nil {
			return fmt.Errorf("exec %s: %w", e.Name(), err)
		}
		if _, err := db.Exec("INSERT INTO _migrations (filename, applied_at) VALUES (?, ?)", e.Name(), time.Now().Unix()); err != nil {
			return fmt.Errorf("record migration %s: %w", e.Name(), err)
		}
		slog.Info("migration applied", "file", e.Name())
	}
	return nil
}

func runHealthcheck(addr string) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr, Timeout: 5 * time.Second}
	resp, err := client.Get(addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck failed")
		os.Exit(1)
	}
	fmt.Println("ok")
}

func ensureDefaultVault(svc *app.VaultService, name, tenantID string) error {
	_, err := svc.GetByName(context.Background(), name)
	if err == nil {
		return nil
	}
	_, err = svc.Create(context.Background(), name, "Default Vault", tenantID)
	return err
}

func vaultHostFromCtx(cfg config.Config) string {
	return domain.BuildVaultHost(cfg.DefaultVault, cfg.BaseDomain, portFromAddr(cfg.Addr))
}

func portFromAddr(addr string) string {
	if _, port, ok := strings.Cut(addr, ":"); ok {
		return port
	}
	return "13000"
}

func mustOK(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}
