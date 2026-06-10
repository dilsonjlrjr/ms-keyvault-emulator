package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dilsonrabelo/kvemu/internal/adapters/http/middleware"
	"github.com/dilsonrabelo/kvemu/internal/domain"
)

const (
	testVaultHost = "lab-dilson:13000"
	testTenantID  = "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"
)

func withVault(r *http.Request) *http.Request {
	v := &domain.Vault{
		Name:     "lab-dilson",
		Host:     testVaultHost,
		TenantID: testTenantID,
	}
	ctx := context.WithValue(r.Context(), middleware.CtxKeyVault, v)
	return r.WithContext(ctx)
}

// legacySdkParse replica o parser FRÁGIL do azure-security-keyvault-secrets <=4.6
// (Boot 2.7). Qualquer token sem '=' causa ArrayIndexOutOfBoundsException no Java.
func legacySdkParse(header string) (map[string]string, error) {
	h := strings.ReplaceAll(header, "Bearer ", "")
	h = strings.ReplaceAll(h, "bearer ", "")
	out := map[string]string{}
	for _, pair := range strings.Split(h, " ") {
		pair = strings.TrimRight(strings.TrimSpace(pair), ",")
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) < 2 {
			return nil, fmt.Errorf("token sem '=': %q  (=> ArrayIndexOutOfBoundsException no SDK legado)", pair)
		}
		out[strings.Trim(kv[0], `"`)] = strings.Trim(kv[1], `",`)
	}
	return out, nil
}

// TestBuildChallenge_GoldenString verifica o valor byte-a-byte esperado.
func TestBuildChallenge_GoldenString(t *testing.T) {
	got := middleware.BuildChallenge(testVaultHost, testTenantID)
	want := `Bearer authorization="https://lab-dilson:13000/a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f", resource="https://lab-dilson"`
	if got != want {
		t.Fatalf("header divergiu do canônico:\n got:  %s\n want: %s", got, want)
	}
}

// TestChallengeMiddleware_EmitsExactlyOneHeader garante que o middleware emite
// UM único header WWW-Authenticate (nunca duplicado via Add).
func TestChallengeMiddleware_EmitsExactlyOneHeader(t *testing.T) {
	mw := middleware.Challenge()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := withVault(httptest.NewRequest(http.MethodGet, "/secrets/x", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	vals := rec.Header().Values("WWW-Authenticate")
	if len(vals) != 1 {
		t.Fatalf("esperado 1 header WWW-Authenticate, veio %d: %v", len(vals), vals)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, veio %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("esperado body vazio no 401, veio %q", rec.Body.String())
	}
}

// TestChallengeMiddleware_SurviveLegacySdkParser — a prova central.
// Simula o que o Java faz: concatena múltiplos headers com vírgula, depois
// passa pelo parser split(" ") -> split("="). Não pode estourar.
func TestChallengeMiddleware_SurviveLegacySdkParser(t *testing.T) {
	mw := middleware.Challenge()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := withVault(httptest.NewRequest(http.MethodGet, "/secrets/x", nil))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// simula concatenação por vírgula que o HttpClient Java faz com múltiplos headers
	joined := strings.Join(rec.Header().Values("WWW-Authenticate"), ", ")

	attrs, err := legacySdkParse(joined)
	if err != nil {
		t.Fatalf("parser legado (Boot 2.7) quebraria com ArrayIndexOutOfBoundsException: %v", err)
	}

	if _, ok := attrs["authorization"]; !ok {
		t.Fatal("campo 'authorization' ausente — SDK legado não consegue extrair tenant/authority")
	}
	if attrs["resource"] != "https://lab-dilson" {
		t.Fatalf("campo 'resource' errado: %q", attrs["resource"])
	}
}

// TestChallengeMiddleware_AuthorizationIsCleanURI garante que authorization= é
// uma URI sem query string (sinal '=' na URL quebra split("=") do SDK legado).
func TestChallengeMiddleware_AuthorizationIsCleanURI(t *testing.T) {
	got := middleware.BuildChallenge(testVaultHost, testTenantID)

	// extrai valor de authorization= (entre aspas)
	const prefix = `authorization="`
	idx := strings.Index(got, prefix)
	if idx < 0 {
		t.Fatal("authorization= não encontrado no header")
	}
	rest := got[idx+len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatal("aspas de fechamento de authorization não encontradas")
	}
	authURI := rest[:end]

	u, err := url.ParseRequestURI(authURI)
	if err != nil {
		t.Fatalf("authorization não é URI válida: %v", err)
	}
	if u.RawQuery != "" {
		t.Fatalf("authorization tem query string (%q) — quebra split('=') do SDK legado", u.RawQuery)
	}
}

// TestChallengeMiddleware_PassesWithToken — com Bearer, 401 não deve ser emitido.
func TestChallengeMiddleware_PassesWithToken(t *testing.T) {
	mw := middleware.Challenge()
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := withVault(httptest.NewRequest(http.MethodGet, "/secrets/x", nil))
	req.Header.Set("Authorization", "Bearer fake.token.here")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("com token, esperado 200, veio %d", rec.Code)
	}
	if v := rec.Header().Get("WWW-Authenticate"); v != "" {
		t.Fatalf("não deve emitir WWW-Authenticate em resposta 200, veio: %q", v)
	}
}

// TestBuildChallenge_NoBareBearer — nunca pode haver um 'Bearer' solto (sem atributos).
func TestBuildChallenge_NoBareBearer(t *testing.T) {
	header := middleware.BuildChallenge(testVaultHost, testTenantID)
	parts := strings.Split(header, " ")
	for i, p := range parts {
		clean := strings.TrimRight(p, ",")
		if strings.EqualFold(clean, "bearer") && i > 0 {
			t.Fatalf("encontrado 'Bearer' solto na posição %d do header: %q", i, header)
		}
	}
}

// TestBuildChallenge_ResourceNoDefaultSuffix — resource NÃO deve ter '/.default'.
func TestBuildChallenge_ResourceNoDefaultSuffix(t *testing.T) {
	header := middleware.BuildChallenge(testVaultHost, testTenantID)
	if strings.Contains(header, "/.default") {
		t.Fatalf("resource não deve conter '/.default': %q", header)
	}
}
