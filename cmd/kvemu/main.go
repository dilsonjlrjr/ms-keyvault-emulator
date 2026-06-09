package main

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	kvHTTP "github.com/dilsonrabelo/kvemu/internal/adapters/http"
	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/app"
	"github.com/dilsonrabelo/kvemu/internal/config"
)

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// subcomando healthcheck (usado pelo Docker HEALTHCHECK)
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		addr := "https://localhost:13000"
		for i, a := range os.Args {
			if a == "--addr" && i+1 < len(os.Args) {
				addr = os.Args[i+1]
			}
		}
		runHealthcheck(addr)
		return
	}

	cfg := config.Load()

	slog.Info("kvemu starting", "version", version, "addr", cfg.Addr, "vault", cfg.VaultHost)

	// ── banco ──────────────────────────────────────────────────────────────
	db, err := sqlite.Open(cfg.DataPath)
	mustOK(err, "sqlite open")
	defer db.Close()
	mustOK(migrate(db, "migrations"), "migrate")
	mustOK(ensureVault(db, cfg.VaultHost, cfg.TenantID), "ensure vault")

	// ── repositórios e serviços ────────────────────────────────────────────
	secretRepo := sqlite.NewSecretRepo(db, cfg.MasterKey)
	secretSvc := app.NewSecretService(secretRepo, cfg.VaultHost)

	keyRepo := sqlite.NewKeyRepo(db, cfg.MasterKey)
	keySvc := app.NewKeyService(keyRepo, cfg.VaultHost)

	certRepo := sqlite.NewCertRepo(db)
	certSvc := app.NewCertService(certRepo, secretSvc, keySvc, cfg.VaultHost)

	// ── cripto: chave AAD fake ─────────────────────────────────────────────
	aadKey, err := kvCrypto.NewAADKey()
	mustOK(err, "aad key gen")

	// ── TLS ───────────────────────────────────────────────────────────────
	var tlsBundle *kvCrypto.TLSBundle
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		tlsBundle, err = kvCrypto.LoadBYO(cfg.TLSCert, cfg.TLSKey, "")
		mustOK(err, "load BYO cert")
		slog.Info("tls: using provided cert", "cert", cfg.TLSCert)
	} else if cfg.TLSAuto {
		tlsBundle, err = kvCrypto.GenerateOrLoad(cfg.CertDir, cfg.VaultHost, cfg.TLSSANs)
		mustOK(err, "tls generate/load")
		if cfg.CAOut != "" {
			dir := filepath.Dir(cfg.CAOut)
			os.MkdirAll(dir, 0700)
			os.WriteFile(cfg.CAOut, tlsBundle.CAPEM, 0644)
			slog.Info("tls: CA exported", "path", cfg.CAOut)
		}
		slog.Info("tls: auto cert ready", "cert-dir", cfg.CertDir)
	} else {
		slog.Error("TLS required: set KV_TLS_AUTO=true or provide KV_TLS_CERT+KV_TLS_KEY")
		os.Exit(1)
	}

	// ── router ────────────────────────────────────────────────────────────
	router := kvHTTP.NewRouter(kvHTTP.RouterConfig{
		VaultHost:  cfg.VaultHost,
		TenantID:   cfg.TenantID,
		AADKey:     aadKey,
		AuthStrict: cfg.AuthStrict,
		Secrets:    kvHTTP.NewSecretHandlers(secretSvc, cfg.VaultHost),
		Keys:       kvHTTP.NewKeyHandlers(keySvc, cfg.VaultHost),
		Certs:      kvHTTP.NewCertHandlers(certSvc, cfg.VaultHost),
	})

	// ── servidor HTTPS ─────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: router,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsBundle.Cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	slog.Info("kvemu ready", "https", "https://"+cfg.VaultHost,
		"tenant", cfg.TenantID, "auth-strict", cfg.AuthStrict)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func migrate(db *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("migrations dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sql, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		if _, err := db.Exec(string(sql)); err != nil {
			return fmt.Errorf("exec %s: %w", e.Name(), err)
		}
		slog.Debug("migration applied", "file", e.Name())
	}
	return nil
}

func runHealthcheck(addr string) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(addr + "/healthz")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck failed")
		os.Exit(1)
	}
	fmt.Println("ok")
}

// ensureVault cria o registro do vault padrão se não existir.
func ensureVault(db *sql.DB, vaultHost, tenantID string) error {
	_, err := db.Exec(`
		INSERT OR IGNORE INTO vault(id, dns_name, tenant_id, created)
		VALUES (?, ?, ?, ?)`,
		vaultHost,
		"https://"+vaultHost,
		tenantID,
		time.Now().Unix(),
	)
	return err
}

func mustOK(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}
