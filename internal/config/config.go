package config

import (
	"os"
	"strconv"
)

type Config struct {
	Addr       string // KV_ADDR — bind do servidor, ex: 0.0.0.0:13000
	VaultHost  string // KV_VAULT_HOST — host:port usado no challenge/SAN/IDs
	TenantID   string // KV_TENANT_ID — GUID do tenant do AAD fake
	DataPath   string // KV_DATA — caminho do arquivo SQLite
	TLSAuto    bool   // KV_TLS_AUTO — gera CA+leaf no boot
	TLSSANs    string // KV_TLS_SAN — SANs extras (CSV)
	TLSCert    string // KV_TLS_CERT — BYO cert PEM
	TLSKey     string // KV_TLS_KEY — BYO key PEM
	CAOut      string // KV_CA_OUT — onde exportar a CA pública
	CertDir    string // KV_CERT_DIR — dir de persistência dos certs TLS gerados
	AuthStrict bool   // KV_AUTH_STRICT — validação estrita do JWT
	MasterKey  string // KV_MASTER_KEY — chave derivada para cifra at-rest
}

func Load() Config {
	c := Config{
		Addr:      env("KV_ADDR", "0.0.0.0:13000"),
		VaultHost: env("KV_VAULT_HOST", "localhost:13000"),
		TenantID:  env("KV_TENANT_ID", "a0c2a3f5-e1b3-4d6a-9c41-2cdd1f2c7e0f"),
		DataPath:  env("KV_DATA", "./data/kv.db"),
		TLSSANs:   env("KV_TLS_SAN", ""),
		TLSCert:   env("KV_TLS_CERT", ""),
		TLSKey:    env("KV_TLS_KEY", ""),
		CAOut:     env("KV_CA_OUT", "./certs/ca.pem"),
		CertDir:   env("KV_CERT_DIR", "./certs"),
		MasterKey: env("KV_MASTER_KEY", ""),
	}
	c.TLSAuto = envBool("KV_TLS_AUTO", true)
	c.AuthStrict = envBool("KV_AUTH_STRICT", false)
	return c
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
