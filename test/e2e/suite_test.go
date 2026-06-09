//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	kvCrypto "github.com/dilsonrabelo/kvemu/internal/adapters/crypto"
	kvHTTP "github.com/dilsonrabelo/kvemu/internal/adapters/http"
	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/app"
)

const (
	testTenantID  = "e2e-tenant"
	testMasterKey = "e2e-master-key-32-bytes-for-tests"
	apiVersion    = "7.4"
)

var tc *kvClient // cliente compartilhado por todos os testes

// ─── TestMain ────────────────────────────────────────────────────────────────

func TestMain(m *testing.M) {
	baseURL, cleanup := startServer()
	defer cleanup()

	insecure := insecureHTTP()
	tok := acquireToken(insecure, baseURL, testTenantID)

	tc = &kvClient{
		base:   baseURL,
		token:  tok,
		tenant: testTenantID,
		http:   insecure,
	}

	os.Exit(m.Run())
}

// ─── servidor in-process ──────────────────────────────────────────────────────

func startServer() (string, func()) {
	tmpDir, err := os.MkdirTemp("", "kvemu-e2e-*")
	must(err)

	certDir := filepath.Join(tmpDir, "certs")
	dataPath := filepath.Join(tmpDir, "kv.db")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	must(err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	vaultHost := fmt.Sprintf("127.0.0.1:%d", port)

	db, err := sqlite.Open(dataPath)
	must(err)
	runMigrations(db)
	db.Exec(`INSERT OR IGNORE INTO vault(id,dns_name,tenant_id,created) VALUES (?,?,?,?)`,
		vaultHost, "https://"+vaultHost, testTenantID, time.Now().Unix())

	secretRepo := sqlite.NewSecretRepo(db, testMasterKey)
	secretSvc := app.NewSecretService(secretRepo, vaultHost)
	keyRepo := sqlite.NewKeyRepo(db, testMasterKey)
	keySvc := app.NewKeyService(keyRepo, vaultHost)
	certRepo := sqlite.NewCertRepo(db)
	certSvc := app.NewCertService(certRepo, secretSvc, keySvc, vaultHost)
	auditRepo := sqlite.NewAuditRepo(db)

	aadKey, err := kvCrypto.NewAADKey()
	must(err)
	tlsBundle, err := kvCrypto.GenerateOrLoad(certDir, vaultHost, "")
	must(err)

	router := kvHTTP.NewRouter(kvHTTP.RouterConfig{
		VaultHost:  vaultHost,
		TenantID:   testTenantID,
		AADKey:     aadKey,
		AuthStrict: false,
		AuditFn: func(actor, op, resource string, status int, reqID string) {
			auditRepo.Log(context.Background(), actor, op, resource, status, reqID)
		},
		Secrets: kvHTTP.NewSecretHandlers(secretSvc, vaultHost),
		Keys:    kvHTTP.NewKeyHandlers(keySvc, vaultHost),
		Certs:   kvHTTP.NewCertHandlers(certSvc, vaultHost),
	})

	srv := &http.Server{
		Handler: router,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsBundle.Cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	ln2, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	must(err)
	go srv.ServeTLS(ln2, "", "")

	baseURL := "https://" + vaultHost
	waitReady(baseURL)

	return baseURL, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		db.Close()
		os.RemoveAll(tmpDir)
	}
}

// ─── kvClient ─────────────────────────────────────────────────────────────────

type kvClient struct {
	base   string
	token  string
	tenant string
	http   *http.Client
}

func (c *kvClient) do(method, path string, body any) *http.Response {
	var bodyR io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyR = bytes.NewReader(b)
	}
	u := c.base + path
	if body != nil {
		// só adiciona api-version em chamadas de dados
	}
	req, _ := http.NewRequest(method, u, bodyR)
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		panic(fmt.Sprintf("http do %s %s: %v", method, path, err))
	}
	return resp
}

func (c *kvClient) dataURL(path string) string {
	return path + "?api-version=" + apiVersion
}

func (c *kvClient) decodeJSON(resp *http.Response) map[string]any {
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	return m
}

func (c *kvClient) assertOK(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("want %d, got %d: %s", want, resp.StatusCode, body)
	}
}

// ─── helpers de autenticação ──────────────────────────────────────────────────

func acquireToken(client *http.Client, baseURL, tenantID string) string {
	u := fmt.Sprintf("%s/%s/oauth2/v2.0/token", baseURL, tenantID)
	resp, err := client.PostForm(u, url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {"e2e-client"},
		"client_secret": {"e2e-secret"},
		"scope":         {"https://vault.azure.net/.default"},
	})
	must(err)
	defer resp.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.AccessToken == "" {
		panic("acquireToken: empty access_token")
	}
	return result.AccessToken
}

// ─── helpers gerais ───────────────────────────────────────────────────────────

func insecureHTTP() *http.Client {
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   10 * time.Second,
	}
}

func waitReady(baseURL string) {
	client := insecureHTTP()
	for i := 0; i < 40; i++ {
		resp, err := client.Get(baseURL + "/healthz")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	panic("server not ready after 2s")
}

func runMigrations(db *sql.DB) {
	// working dir quando go test ./test/e2e/... é test/e2e/
	entries, err := os.ReadDir("../../migrations")
	if err != nil {
		panic(fmt.Sprintf("migrations dir: %v (run from repo root with: go test ./test/e2e/...)", err))
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		sqlBytes, _ := os.ReadFile(filepath.Join("../../migrations", e.Name()))
		db.Exec(string(sqlBytes))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
