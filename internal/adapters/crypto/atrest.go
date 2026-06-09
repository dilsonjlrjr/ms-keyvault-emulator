package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
)

const devMasterKey = "kvemu-dev-insecure-master-key-000"

// DeriveKey deriva uma chave AES-256 de masterKey via SHA-256.
func DeriveKey(masterKey string) []byte {
	if masterKey == "" {
		slog.Warn("KV_MASTER_KEY não definida; usando chave de dev — não use em produção")
		masterKey = devMasterKey
	}
	h := sha256.Sum256([]byte(masterKey))
	return h[:]
}

// Seal cifra plaintext com AES-256-GCM. Retorna (ciphertext, nonce, err).
func Seal(key, plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return ct, nonce, nil
}

// Open decifra ciphertext com AES-256-GCM.
func Open(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("at-rest decrypt: %w", err)
	}
	return plain, nil
}
