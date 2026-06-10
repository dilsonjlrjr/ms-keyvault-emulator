package domain

import (
	"fmt"
	"strings"
)

type Vault struct {
	Name        string
	Host        string
	DisplayName string
	TenantID    string
	Created     int64
	Updated     int64
}

func VaultNameFromHost(host, baseDomain string) string {
	if h, _, ok := strings.Cut(host, ":"); ok {
		host = h
	}
	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	name := host[:len(host)-len(suffix)]
	if name == "" {
		return ""
	}
	return name
}

func BuildVaultHost(name, baseDomain, port string) string {
	return fmt.Sprintf("%s.%s:%s", name, baseDomain, port)
}

const VaultNamePattern = `^[a-z0-9]([a-z0-9-]{0,22}[a-z0-9])?$`

var ErrVaultNotFound = &ErrNotFound{Kind: "vault"}
var ErrVaultExists = &ErrConflict{Kind: "vault"}

type VaultExport struct {
	Version      string             `json:"version"`
	Vault        VaultExportMeta    `json:"vault"`
	Secrets      []SecretExportItem `json:"secrets"`
	Keys         []KeyExportItem    `json:"keys"`
	Certificates []CertExportItem   `json:"certificates"`
}

type VaultExportMeta struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	TenantID    string `json:"tenant_id"`
}

type SecretExportItem struct {
	Name        string            `json:"name"`
	Value       string            `json:"value"`
	ContentType string            `json:"content_type,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Attributes  ExportAttributes  `json:"attributes,omitempty"`
}

type KeyExportItem struct {
	Name    string            `json:"name"`
	Kty     string            `json:"kty"`
	KeySize int               `json:"key_size,omitempty"`
	Crv     string            `json:"crv,omitempty"`
	KeyOps  []string          `json:"key_ops,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
}

type CertExportItem struct {
	Name       string         `json:"name"`
	PolicyJSON map[string]any `json:"policy_json,omitempty"`
	CerBase64  string         `json:"cer_base64,omitempty"`
	Tags       map[string]any `json:"tags,omitempty"`
}

type ExportAttributes struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Nbf     *int64 `json:"nbf,omitempty"`
	Exp     *int64 `json:"exp,omitempty"`
}

type VaultImportResult struct {
	Vault          string `json:"vault"`
	SecretsCreated int    `json:"secrets_created"`
	SecretsSkipped int    `json:"secrets_skipped"`
	KeysCreated    int    `json:"keys_created"`
	KeysSkipped    int    `json:"keys_skipped"`
	CertsCreated   int    `json:"certs_created"`
	CertsSkipped   int    `json:"certs_skipped"`
}
