//go:build e2e

package e2e_test

// Testes de compatibilidade com Spring Boot 2.7 / azure-sdk-for-java ≤4.6.7
// Reproduz o parser de challenge do KeyVaultCredentialPolicy.extractChallengeAttributes
// que causava ArrayIndexOutOfBoundsException no SDK legado.

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ─── TestChallengeHeaderFormat ────────────────────────────────────────────────
// Garante que o emulador emite exatamente UM header WWW-Authenticate
// no formato canônico que o SDK Spring consegue parsear.

func TestChallengeHeaderFormat(t *testing.T) {
	// chamada sem token — deve retornar 401 com o header certo
	req, _ := http.NewRequest(http.MethodGet, tc.base+"/secrets/anything?api-version=7.4", nil)
	resp, err := tc.http.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}

	// exatamente UM valor de WWW-Authenticate
	headers := resp.Header["Www-Authenticate"]
	if len(headers) != 1 {
		t.Fatalf("want exactly 1 WWW-Authenticate header, got %d: %v", len(headers), headers)
	}

	hdr := headers[0]

	// deve começar com "Bearer " (não bare "Bearer" sem params)
	if !strings.HasPrefix(hdr, "Bearer ") {
		t.Errorf("header must start with 'Bearer ', got: %q", hdr)
	}

	// deve conter authorization= e resource=
	if !strings.Contains(hdr, `authorization="`) {
		t.Errorf("header missing authorization param: %q", hdr)
	}
	if !strings.Contains(hdr, `resource="`) {
		t.Errorf("header missing resource param: %q", hdr)
	}

	// NÃO deve conter /.default (quebra o parser legado)
	if strings.Contains(hdr, "/.default") {
		t.Errorf("header must NOT contain /.default: %q", hdr)
	}

	t.Logf("WWW-Authenticate: %s", hdr)
}

// ─── TestLegacySDKParserDoesNotCrash ─────────────────────────────────────────
// Replica exata do parser do azure-sdk-for-java ≤4.6.7:
// KeyVaultCredentialPolicy.extractChallengeAttributes
// O parser faz split("=") em cada token — se receber "Bearer" isolado, kv[1] estoura.

func TestLegacySDKParserDoesNotCrash(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, tc.base+"/secrets/anything?api-version=7.4", nil)
	resp, _ := tc.http.Do(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	hdr := resp.Header.Get("Www-Authenticate")
	authority, resource, err := legacySDKParse(hdr)
	if err != nil {
		t.Fatalf("legacySDKParse crashed (replicates Spring Boot 2.7 ArrayIndexOutOfBoundsException): %v\nheader was: %q", err, hdr)
	}
	if authority == "" {
		t.Errorf("authority empty after parse")
	}
	if resource == "" {
		t.Errorf("resource empty after parse")
	}
	t.Logf("legacy parse OK — authority=%q resource=%q", authority, resource)
}

// ─── TestFullAuthFlow ─────────────────────────────────────────────────────────
// Reproduz o fluxo completo que o Spring Boot 2.7 executa:
//  1. Chamada sem token → 401 + challenge
//  2. Parse do challenge para obter authority
//  3. GET token no endpoint do AAD emulado
//  4. Chamada com Bearer → 200

func TestFullAuthFlow(t *testing.T) {
	// 1. unauthenticated
	req, _ := http.NewRequest(http.MethodGet, tc.base+"/secrets/anything?api-version=7.4", nil)
	resp, _ := tc.http.Do(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("step 1: want 401, got %d", resp.StatusCode)
	}

	// 2. parse challenge
	hdr := resp.Header.Get("Www-Authenticate")
	authority, _, err := legacySDKParse(hdr)
	if err != nil {
		t.Fatalf("step 2: parse challenge: %v", err)
	}

	// 3. token — authority tem formato https://host/tenant
	// convertemos para https://host/{tenant}/oauth2/v2.0/token
	tokenURL := authority + "/oauth2/v2.0/token"
	tok := acquireToken(tc.http, strings.TrimSuffix(tc.base, ""), testTenantID)
	if tok == "" {
		// fallback: usa o URL derivado do authority
		tok = acquireToken(tc.http, tokenURLToBase(tokenURL), testTenantID)
	}
	if tok == "" {
		t.Fatal("step 3: empty token")
	}

	// 4. chamada autenticada → deve retornar 404 (secret não existe) mas NOT 401/403
	req2, _ := http.NewRequest(http.MethodGet, tc.base+"/secrets/anything?api-version=7.4", nil)
	req2.Header.Set("Authorization", "Bearer "+tok)
	resp2, _ := tc.http.Do(req2)
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()

	if resp2.StatusCode == 401 || resp2.StatusCode == 403 {
		t.Fatalf("step 4: auth failed, got %d (expected 404 or 200)", resp2.StatusCode)
	}
	t.Logf("full auth flow OK — final status %d", resp2.StatusCode)
}

// ─── TestRequestIDHeader ──────────────────────────────────────────────────────

func TestRequestIDHeader(t *testing.T) {
	resp := tc.do(http.MethodGet, tc.dataURL("/secrets/anything"), nil)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.Header.Get("X-Ms-Request-Id") == "" {
		t.Error("x-ms-request-id header missing")
	}
}

// ─── legacySDKParse: réplica do parser do azure-sdk-for-java ≤4.6.7 ──────────

func legacySDKParse(wwwAuth string) (authority, resource string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("index out of bounds panic (replicates Java ArrayIndexOutOfBoundsException): %v", r)
		}
	}()

	if !strings.HasPrefix(strings.ToLower(wwwAuth), "bearer ") {
		return "", "", fmt.Errorf("not a Bearer challenge")
	}
	rest := wwwAuth[len("Bearer "):]

	// SDK faz split(", ") nos tokens do challenge
	tokens := strings.Split(rest, ", ")
	kvMap := make(map[string]string)
	for _, tok := range tokens {
		// split("=") — kv[1] causa OOB se tok não contém "="
		kv := strings.Split(tok, "=")
		key := strings.TrimSpace(kv[0])
		val := strings.Trim(strings.TrimSpace(kv[1]), `"`) // PANIC se len(kv) < 2
		kvMap[key] = val
	}

	return kvMap["authorization"], kvMap["resource"], nil
}

func tokenURLToBase(tokenURL string) string {
	// https://host/tenant/oauth2/v2.0/token → https://host
	parts := strings.SplitN(tokenURL, "/", 4)
	if len(parts) < 3 {
		return ""
	}
	return parts[0] + "//" + parts[2]
}
