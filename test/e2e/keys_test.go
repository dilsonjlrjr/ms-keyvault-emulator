//go:build e2e

package e2e_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestKeyCreateAndGet(t *testing.T) {
	name := "e2e-rsa-key"

	resp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/create"), map[string]any{
		"kty":     "RSA",
		"keySize": 2048,
		"key_ops": []string{"sign", "verify", "encrypt", "decrypt"},
		"attributes": map[string]any{"enabled": true},
	})
	tc.assertOK(t, resp, http.StatusOK)
	body := tc.decodeJSON(resp)
	key, _ := body["key"].(map[string]any)
	if key == nil {
		t.Fatal("create: missing 'key' field")
	}
	if key["kty"] != "RSA" {
		t.Errorf("create: kty mismatch: %v", key["kty"])
	}
	if key["n"] == nil {
		t.Error("create: missing modulus 'n'")
	}
	kid, _ := key["kid"].(string)
	if kid == "" {
		t.Error("create: missing kid")
	}
	t.Logf("created key kid=%s", kid)

	// get by name (current version)
	resp2 := tc.do(http.MethodGet, tc.dataURL("/keys/"+name), nil)
	tc.assertOK(t, resp2, http.StatusOK)
	body2 := tc.decodeJSON(resp2)
	key2, _ := body2["key"].(map[string]any)
	if key2["kty"] != "RSA" {
		t.Errorf("get: kty mismatch: %v", key2["kty"])
	}
}

func TestKeyList(t *testing.T) {
	for i := range 2 {
		resp := tc.do(http.MethodPost, tc.dataURL(fmt.Sprintf("/keys/list-key-%d/create", i)), map[string]any{
			"kty": "RSA", "keySize": 2048,
		})
		tc.assertOK(t, resp, http.StatusOK)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp := tc.do(http.MethodGet, tc.dataURL("/keys"), nil)
	tc.assertOK(t, resp, http.StatusOK)
	body := tc.decodeJSON(resp)
	items, _ := body["value"].([]any)
	if len(items) == 0 {
		t.Error("key list empty")
	}
	t.Logf("key list: %d items", len(items))
}

func TestKeyEncryptDecrypt(t *testing.T) {
	name := "enc-dec-key"
	resp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/create"), map[string]any{
		"kty":     "RSA",
		"keySize": 2048,
		"key_ops": []string{"encrypt", "decrypt"},
	})
	tc.assertOK(t, resp, http.StatusOK)
	created := tc.decodeJSON(resp)
	kid, _ := created["key"].(map[string]any)["kid"].(string)
	version := extractVersion(kid)

	plaintext := base64.RawURLEncoding.EncodeToString([]byte("hello kvemu"))

	// encrypt
	encResp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/"+version+"/encrypt"), map[string]any{
		"alg":   "RSA-OAEP",
		"value": plaintext,
	})
	tc.assertOK(t, encResp, http.StatusOK)
	encBody := tc.decodeJSON(encResp)
	ciphertext, _ := encBody["value"].(string)
	if ciphertext == "" {
		t.Fatal("encrypt: empty ciphertext")
	}

	// decrypt
	decResp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/"+version+"/decrypt"), map[string]any{
		"alg":   "RSA-OAEP",
		"value": ciphertext,
	})
	tc.assertOK(t, decResp, http.StatusOK)
	decBody := tc.decodeJSON(decResp)
	recovered, _ := decBody["value"].(string)
	if recovered != plaintext {
		t.Errorf("decrypt: got %q, want %q", recovered, plaintext)
	}
}

func TestKeySignVerify(t *testing.T) {
	name := "sign-verify-key"
	resp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/create"), map[string]any{
		"kty":     "RSA",
		"keySize": 2048,
		"key_ops": []string{"sign", "verify"},
	})
	tc.assertOK(t, resp, http.StatusOK)
	created := tc.decodeJSON(resp)
	kid, _ := created["key"].(map[string]any)["kid"].(string)
	version := extractVersion(kid)

	// digest SHA-256 de "test-data" em base64url
	h := sha256.Sum256([]byte("test-data"))
	digest := base64.RawURLEncoding.EncodeToString(h[:])

	signResp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/"+version+"/sign"), map[string]any{
		"alg":   "RS256",
		"value": digest,
	})
	tc.assertOK(t, signResp, http.StatusOK)
	signBody := tc.decodeJSON(signResp)
	sig, _ := signBody["value"].(string)
	if sig == "" {
		t.Fatal("sign: empty signature")
	}

	verifyResp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/"+version+"/verify"), map[string]any{
		"alg":    "RS256",
		"digest": digest,
		"value":  sig,
	})
	tc.assertOK(t, verifyResp, http.StatusOK)
	verifyBody := tc.decodeJSON(verifyResp)
	if verifyBody["value"] != true {
		t.Errorf("verify: expected true, got %v", verifyBody["value"])
	}
}

func TestKeyDeleteAndRecover(t *testing.T) {
	name := "del-rec-key"
	resp := tc.do(http.MethodPost, tc.dataURL("/keys/"+name+"/create"), map[string]any{
		"kty": "RSA", "keySize": 2048,
	})
	tc.assertOK(t, resp, http.StatusOK)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	del := tc.do(http.MethodDelete, tc.dataURL("/keys/"+name), nil)
	tc.assertOK(t, del, http.StatusOK)
	delBody := tc.decodeJSON(del)
	if delBody["recoveryId"] == nil {
		t.Error("delete: missing recoveryId")
	}

	gd := tc.do(http.MethodGet, tc.dataURL("/deletedkeys/"+name), nil)
	tc.assertOK(t, gd, http.StatusOK)
	io.Copy(io.Discard, gd.Body)
	gd.Body.Close()

	rec := tc.do(http.MethodPost, tc.dataURL("/deletedkeys/"+name+"/recover"), nil)
	tc.assertOK(t, rec, http.StatusOK)
	io.Copy(io.Discard, rec.Body)
	rec.Body.Close()

	got := tc.do(http.MethodGet, tc.dataURL("/keys/"+name), nil)
	tc.assertOK(t, got, http.StatusOK)
	io.Copy(io.Discard, got.Body)
	got.Body.Close()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func extractVersion(kid string) string {
	parts := splitBySlash(kid)
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
