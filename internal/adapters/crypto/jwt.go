package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// AADKey é o par de chaves usado pelo AAD fake para assinar JWTs.
type AADKey struct {
	Private *rsa.PrivateKey
	KID     string
}

func NewAADKey() (*AADKey, error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	kidBytes := make([]byte, 16)
	rand.Read(kidBytes)
	return &AADKey{
		Private: k,
		KID:     fmt.Sprintf("%x", kidBytes),
	}, nil
}

// IssueToken emite um JWT RS256 compatível com o que o azure-identity espera.
func (k *AADKey) IssueToken(issuer, clientID, tenantID string) (string, int64, error) {
	exp := time.Now().Add(time.Hour).Unix()
	claims := gojwt.MapClaims{
		"aud":   "https://vault.azure.net",
		"iss":   issuer,
		"appid": clientID,
		"tid":   tenantID,
		"oid":   deterministicUUID(clientID),
		"sub":   deterministicUUID(clientID),
		"iat":   time.Now().Unix(),
		"nbf":   time.Now().Unix(),
		"exp":   exp,
		"ver":   "1.0",
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	token.Header["kid"] = k.KID
	signed, err := token.SignedString(k.Private)
	if err != nil {
		return "", 0, err
	}
	return signed, exp, nil
}

// JWKS retorna o JSON do JWKS com a chave pública.
func (k *AADKey) JWKS() ([]byte, error) {
	pub := &k.Private.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	doc := map[string]any{
		"keys": []map[string]any{
			{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": k.KID, "n": n, "e": e},
		},
	}
	return json.Marshal(doc)
}

// OIDCConfig retorna o JSON de discovery OIDC mínimo.
func OIDCConfig(issuer string) []byte {
	doc := map[string]any{
		"issuer":                                issuer,
		"token_endpoint":                        issuer + "/oauth2/v2.0/token",
		"jwks_uri":                              issuer + "/discovery/v2.0/keys",
		"response_modes_supported":              []string{"query", "fragment", "form_post"},
		"subject_types_supported":               []string{"pairwise"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post", "client_secret_basic"},
		"grant_types_supported":                 []string{"client_credentials"},
	}
	b, _ := json.Marshal(doc)
	return b
}

// ValidateToken valida um JWT (strict) ou só confere estrutura (leniente).
func (k *AADKey) ValidateToken(tokenStr string, strict bool) error {
	if !strict {
		// leniente: só verifica estrutura (3 partes) e exp
		parser := gojwt.NewParser(gojwt.WithoutClaimsValidation())
		claims := gojwt.MapClaims{}
		_, _, err := parser.ParseUnverified(tokenStr, claims)
		if err != nil {
			return fmt.Errorf("malformed token: %w", err)
		}
		return nil
	}
	_, err := gojwt.Parse(tokenStr, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &k.Private.PublicKey, nil
	}, gojwt.WithAudience("https://vault.azure.net"))
	return err
}

// SubjectFromToken extrai o claim "sub" sem validar assinatura.
// Útil para logging/auditoria.
func SubjectFromToken(tokenStr string) string {
	p := gojwt.NewParser(gojwt.WithoutClaimsValidation())
	claims := gojwt.MapClaims{}
	if _, _, err := p.ParseUnverified(tokenStr, claims); err != nil {
		return "unknown"
	}
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		return sub
	}
	if appid, ok := claims["appid"].(string); ok && appid != "" {
		return appid
	}
	return "unknown"
}

func deterministicUUID(seed string) string {
	// UUID v5 manual (sem importar crypto/sha1 só pra isso)
	h := make([]byte, 16)
	copy(h, []byte(seed+"kvemu-oid"))
	h[6] = (h[6] & 0x0f) | 0x50
	h[8] = (h[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", h[0:4], h[4:6], h[6:8], h[8:10], h[10:])
}
