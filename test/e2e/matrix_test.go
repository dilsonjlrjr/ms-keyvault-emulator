//go:build e2e

package e2e_test

// matrix_test.go — testes matriciais que cobrem comportamentos transversais
// (error envelopes, headers, status codes, paginação).

import (
	"fmt"
	"io"
	"net/http"
	"testing"
)

// ─── Matriz de error envelopes ────────────────────────────────────────────────

func TestErrorEnvelopesMatrix(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
		wantCode   string
	}{
		{
			name:       "secret not found",
			method:     http.MethodGet,
			path:       "/secrets/does-not-exist",
			wantStatus: 404,
			wantCode:   "SecretNotFound",
		},
		{
			name:       "key not found",
			method:     http.MethodGet,
			path:       "/keys/does-not-exist",
			wantStatus: 404,
			wantCode:   "KeyNotFound",
		},
		{
			name:       "cert not found",
			method:     http.MethodGet,
			path:       "/certificates/does-not-exist",
			wantStatus: 404,
			wantCode:   "CertificateNotFound",
		},
		{
			name:       "secret bad param — empty value",
			method:     http.MethodPut,
			path:       "/secrets/bad-secret",
			body:       map[string]any{"value": ""},
			wantStatus: 400,
			wantCode:   "BadParameter",
		},
		{
			name:       "cert bad param — missing policy",
			method:     http.MethodPost,
			path:       "/certificates/bad-cert/create",
			body:       map[string]any{},
			wantStatus: 400,
			wantCode:   "BadParameter",
		},
	}

	for _, tc2 := range cases {
		t.Run(tc2.name, func(t *testing.T) {
			resp := tc.do(tc2.method, tc.dataURL(tc2.path), tc2.body)
			defer resp.Body.Close()
			if resp.StatusCode != tc2.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("want %d, got %d: %s", tc2.wantStatus, resp.StatusCode, body)
			}
			body := tc.decodeJSON(resp)
			errObj, _ := body["error"].(map[string]any)
			if errObj == nil {
				t.Fatalf("missing error envelope: %v", body)
			}
			code, _ := errObj["code"].(string)
			if code != tc2.wantCode {
				t.Errorf("want error.code=%q, got %q", tc2.wantCode, code)
			}
		})
	}
}

// ─── Matriz de headers obrigatórios ──────────────────────────────────────────

func TestRequiredHeadersMatrix(t *testing.T) {
	// qualquer endpoint deve retornar os headers x-ms-*
	endpoints := []struct {
		method, path string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, tc.dataURL("/secrets/anything")},
		{http.MethodGet, tc.dataURL("/keys/anything")},
		{http.MethodGet, tc.dataURL("/certificates/anything")},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			resp := tc.do(ep.method, ep.path, nil)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.Header.Get("X-Ms-Request-Id") == "" {
				t.Error("x-ms-request-id header missing")
			}
			if ep.path != "/healthz" {
				if resp.Header.Get("X-Ms-Keyvault-Region") == "" {
					t.Error("x-ms-keyvault-region header missing")
				}
			}
		})
	}
}

// ─── Matriz de soft-delete / conflict ────────────────────────────────────────

func TestDeletedConflictMatrix(t *testing.T) {
	for _, kind := range []string{"secret", "key"} {
		t.Run(kind, func(t *testing.T) {
			name := fmt.Sprintf("conflict-test-%s", kind)

			// cria
			switch kind {
			case "secret":
				resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{"value": "x"})
				tc.assertOK(t, resp, http.StatusOK)
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			case "key":
				resp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/create"), map[string]any{"kty": "RSA", "keySize": 2048})
				tc.assertOK(t, resp, http.StatusOK)
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}

			// deleta
			switch kind {
			case "secret":
				resp := tc.do(http.MethodDelete, tc.dataURL("/secrets/"+name), nil)
				tc.assertOK(t, resp, http.StatusOK)
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			case "key":
				resp := tc.do(http.MethodDelete, tc.dataURL("/keys/"+name), nil)
				tc.assertOK(t, resp, http.StatusOK)
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}

			// tentar criar com mesmo nome deve retornar 409 Conflict
			switch kind {
			case "secret":
				resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{"value": "y"})
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusConflict {
					io.Copy(io.Discard, resp.Body)
					t.Errorf("%s: want 409 after delete, got %d", kind, resp.StatusCode)
					return
				}
				io.Copy(io.Discard, resp.Body)
			}
		})
	}
}

// ─── Matriz de paginação ─────────────────────────────────────────────────────

func TestPaginationMatrix(t *testing.T) {
	// cria 5 secrets distintos para este teste
	prefix := "pagination-test"
	for i := range 5 {
		resp := tc.do(http.MethodPut, tc.dataURL(fmt.Sprintf("/secrets/%s-%d", prefix, i)), map[string]any{
			"value": fmt.Sprintf("val-%d", i),
		})
		tc.assertOK(t, resp, http.StatusOK)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// page 1: maxresults=3
	resp := tc.do(http.MethodGet, tc.dataURL("/secrets")+"&maxresults=3", nil)
	tc.assertOK(t, resp, http.StatusOK)
	page1 := tc.decodeJSON(resp)
	items1, _ := page1["value"].([]any)
	if len(items1) > 3 {
		t.Errorf("page1: want <= 3 items, got %d", len(items1))
	}
	// se há nextLink, busca page 2
	if nl, ok := page1["nextLink"].(string); ok && nl != "" {
		resp2, _ := tc.http.Do(newBearerReq(t, http.MethodGet, nl, tc.token))
		tc.assertOK(t, resp2, http.StatusOK)
		page2 := tc.decodeJSON(resp2)
		items2, _ := page2["value"].([]any)
		t.Logf("pagination: page1=%d items, page2=%d items", len(items1), len(items2))
	} else {
		t.Logf("pagination: single page, %d items total", len(items1))
	}
}

// ─── Matriz OIDC / AAD ────────────────────────────────────────────────────────

func TestAADEndpointsMatrix(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"oidc v2 discovery", fmt.Sprintf("/%s/v2.0/.well-known/openid-configuration", testTenantID)},
		{"oidc v1 discovery", fmt.Sprintf("/%s/.well-known/openid-configuration", testTenantID)},
		{"jwks", fmt.Sprintf("/%s/discovery/v2.0/keys", testTenantID)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := tc.http.Get(tc.base + c.path)
			if err != nil {
				t.Fatalf("GET %s: %v", c.path, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("want 200, got %d", resp.StatusCode)
			}
			body := tc.decodeJSON(resp)
			if body == nil {
				t.Error("empty JSON response")
			}
		})
	}
}

// ─── helper ───────────────────────────────────────────────────────────────────

func newBearerReq(t *testing.T, method, url, token string) *http.Request {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
