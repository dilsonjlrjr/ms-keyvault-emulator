package crypto

import (
	"encoding/json"
	"testing"
)

// TestOIDCConfig_HasAuthorizationEndpoint — o MSAL (azure-identity Python)
// exige authorization_endpoint no discovery mesmo em client_credentials;
// sem a chave o cliente aborta com KeyError 'authorization_endpoint'.
func TestOIDCConfig_HasAuthorizationEndpoint(t *testing.T) {
	b := OIDCConfig("gdr.kvemu.local:13000", "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f")

	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("discovery não é JSON válido: %v", err)
	}

	want := map[string]string{
		"issuer":                 "https://gdr.kvemu.local:13000/a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f/v2.0",
		"authorization_endpoint": "https://gdr.kvemu.local:13000/a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f/oauth2/v2.0/authorize",
		"token_endpoint":         "https://gdr.kvemu.local:13000/a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f/oauth2/v2.0/token",
		"jwks_uri":               "https://gdr.kvemu.local:13000/a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f/discovery/v2.0/keys",
	}
	for k, v := range want {
		got, ok := doc[k]
		if !ok {
			t.Errorf("chave %q ausente no discovery", k)
			continue
		}
		if got != v {
			t.Errorf("%s: got %q, want %q", k, got, v)
		}
	}
}
