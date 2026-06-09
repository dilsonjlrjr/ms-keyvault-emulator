//go:build e2e

package e2e_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestSecretSetAndGet(t *testing.T) {
	name := "e2e-secret-basic"

	// set
	resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{
		"value":       "minha-senha-segura",
		"contentType": "text/plain",
	})
	tc.assertOK(t, resp, http.StatusOK)
	body := tc.decodeJSON(resp)
	if body["value"] != "minha-senha-segura" {
		t.Errorf("set: value mismatch: %v", body["value"])
	}
	id, _ := body["id"].(string)
	if id == "" {
		t.Fatal("set: id missing in response")
	}
	t.Logf("set id=%s", id)

	// get
	resp2 := tc.do(http.MethodGet, tc.dataURL("/secrets/"+name), nil)
	tc.assertOK(t, resp2, http.StatusOK)
	body2 := tc.decodeJSON(resp2)
	if body2["value"] != "minha-senha-segura" {
		t.Errorf("get: value mismatch: %v", body2["value"])
	}
}

func TestSecretGetNotFound(t *testing.T) {
	resp := tc.do(http.MethodGet, tc.dataURL("/secrets/nao-existe-never"), nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		io.Copy(io.Discard, resp.Body)
		t.Fatalf("want 404, got %d", resp.StatusCode)
	}
	body := tc.decodeJSON(resp)
	errObj, _ := body["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("missing error envelope")
	}
	if errObj["code"] == nil {
		t.Error("error.code missing in envelope")
	}
}

func TestSecretList(t *testing.T) {
	// cria 3 secrets
	for i := range 3 {
		resp := tc.do(http.MethodPut, tc.dataURL(fmt.Sprintf("/secrets/list-test-%d", i)), map[string]any{
			"value": fmt.Sprintf("valor-%d", i),
		})
		tc.assertOK(t, resp, http.StatusOK)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// list
	resp := tc.do(http.MethodGet, tc.dataURL("/secrets")+"&maxresults=10", nil)
	tc.assertOK(t, resp, http.StatusOK)
	body := tc.decodeJSON(resp)
	items, _ := body["value"].([]any)
	if len(items) == 0 {
		t.Error("expected at least one secret in list")
	}
	t.Logf("list returned %d items", len(items))

	// cada item não deve ter o campo "value" (comportamento Azure)
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			if _, hasValue := m["value"]; hasValue {
				t.Error("list items must not include 'value' field")
				break
			}
		}
	}
}

func TestSecretListVersions(t *testing.T) {
	name := "versioned-secret"
	// 3 versões
	for i := range 3 {
		resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{
			"value": fmt.Sprintf("v%d", i+1),
		})
		tc.assertOK(t, resp, http.StatusOK)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp := tc.do(http.MethodGet, tc.dataURL("/secrets/"+name+"/versions"), nil)
	tc.assertOK(t, resp, http.StatusOK)
	body := tc.decodeJSON(resp)
	items, _ := body["value"].([]any)
	if len(items) < 3 {
		t.Errorf("want >= 3 versions, got %d", len(items))
	}
}

func TestSecretUpdate(t *testing.T) {
	name := "to-update"
	// cria
	resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{"value": "original"})
	tc.assertOK(t, resp, http.StatusOK)
	created := tc.decodeJSON(resp)
	version, _ := created["id"].(string)

	// extrai versão da URL  (formato: https://host/secrets/name/version)
	parts := splitBySlash(version)
	ver := parts[len(parts)-1]

	// atualiza atributos
	resp2 := tc.do(http.MethodPatch, tc.dataURL("/secrets/"+name+"/"+ver), map[string]any{
		"contentType": "application/json",
		"attributes":  map[string]any{"enabled": false},
	})
	tc.assertOK(t, resp2, http.StatusOK)
	updated := tc.decodeJSON(resp2)
	attrs, _ := updated["attributes"].(map[string]any)
	if attrs["enabled"] != false {
		t.Errorf("want enabled=false, got %v", attrs["enabled"])
	}
}

func TestSecretDeleteAndRecover(t *testing.T) {
	name := "delete-recover"
	resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{"value": "precious"})
	tc.assertOK(t, resp, http.StatusOK)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// delete
	del := tc.do(http.MethodDelete, tc.dataURL("/secrets/"+name), nil)
	tc.assertOK(t, del, http.StatusOK)
	delBody := tc.decodeJSON(del)
	if delBody["recoveryId"] == nil {
		t.Error("delete response missing recoveryId")
	}

	// get after delete → 404
	gone := tc.do(http.MethodGet, tc.dataURL("/secrets/"+name), nil)
	defer gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound && gone.StatusCode != http.StatusConflict {
		t.Errorf("after delete: want 404 or 409, got %d", gone.StatusCode)
	}
	io.Copy(io.Discard, gone.Body)

	// get deleted
	gd := tc.do(http.MethodGet, tc.dataURL("/deletedsecrets/"+name), nil)
	tc.assertOK(t, gd, http.StatusOK)
	io.Copy(io.Discard, gd.Body)
	gd.Body.Close()

	// recover
	rec := tc.do(http.MethodPost, tc.dataURL("/deletedsecrets/"+name+"/recover"), nil)
	tc.assertOK(t, rec, http.StatusOK)
	recBody := tc.decodeJSON(rec)
	if recBody["value"] == nil && recBody["id"] == nil {
		t.Error("recover response missing id")
	}

	// leitura normal voltou
	got := tc.do(http.MethodGet, tc.dataURL("/secrets/"+name), nil)
	tc.assertOK(t, got, http.StatusOK)
	gotBody := tc.decodeJSON(got)
	if gotBody["value"] != "precious" {
		t.Errorf("after recover: value mismatch: %v", gotBody["value"])
	}
}

func TestSecretPurge(t *testing.T) {
	name := "to-purge-e2e"
	resp := tc.do(http.MethodPut, tc.dataURL("/secrets/"+name), map[string]any{"value": "bye"})
	tc.assertOK(t, resp, http.StatusOK)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	del := tc.do(http.MethodDelete, tc.dataURL("/secrets/"+name), nil)
	tc.assertOK(t, del, http.StatusOK)
	io.Copy(io.Discard, del.Body)
	del.Body.Close()

	purge := tc.do(http.MethodDelete, tc.dataURL("/deletedsecrets/"+name), nil)
	if purge.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(purge.Body)
		purge.Body.Close()
		t.Fatalf("purge: want 204, got %d: %s", purge.StatusCode, body)
	}
	purge.Body.Close()

	// não existe mais nem como deletado
	gone := tc.do(http.MethodGet, tc.dataURL("/deletedsecrets/"+name), nil)
	defer gone.Body.Close()
	if gone.StatusCode != http.StatusNotFound {
		t.Errorf("after purge: want 404, got %d", gone.StatusCode)
	}
	io.Copy(io.Discard, gone.Body)
}

// ─── helper ───────────────────────────────────────────────────────────────────

func splitBySlash(s string) []string {
	result := make([]string, 0)
	cur := ""
	for _, c := range s {
		if c == '/' {
			if cur != "" {
				result = append(result, cur)
				cur = ""
			}
		} else {
			cur += string(c)
		}
	}
	if cur != "" {
		result = append(result, cur)
	}
	return result
}
