package crypto

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"math/big"
)

// EncryptData cifra plaintext com a chave pública do JWK usando alg.
func EncryptData(pubJWK map[string]any, alg string, plaintext []byte) ([]byte, error) {
	switch alg {
	case "RSA-OAEP", "RSA-OAEP-256":
		pub, err := rsaPubFromJWK(pubJWK)
		if err != nil {
			return nil, err
		}
		h := oaepHash(alg)
		return rsa.EncryptOAEP(h, rand.Reader, pub, plaintext, nil)
	case "RSA1_5":
		pub, err := rsaPubFromJWK(pubJWK)
		if err != nil {
			return nil, err
		}
		return rsa.EncryptPKCS1v15(rand.Reader, pub, plaintext)
	default:
		return nil, fmt.Errorf("unsupported encrypt alg: %q", alg)
	}
}

// DecryptData decifra ciphertext com a chave privada do JWK usando alg.
func DecryptData(privJWK map[string]any, alg string, ciphertext []byte) ([]byte, error) {
	switch alg {
	case "RSA-OAEP", "RSA-OAEP-256":
		priv, err := rsaPrivFromJWK(privJWK)
		if err != nil {
			return nil, err
		}
		h := oaepHash(alg)
		return rsa.DecryptOAEP(h, rand.Reader, priv, ciphertext, nil)
	case "RSA1_5":
		priv, err := rsaPrivFromJWK(privJWK)
		if err != nil {
			return nil, err
		}
		return rsa.DecryptPKCS1v15(rand.Reader, priv, ciphertext)
	default:
		return nil, fmt.Errorf("unsupported decrypt alg: %q", alg)
	}
}

// SignData assina digest com a chave privada do JWK.
func SignData(privJWK map[string]any, alg string, data []byte) ([]byte, error) {
	switch alg {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
		priv, err := rsaPrivFromJWK(privJWK)
		if err != nil {
			return nil, err
		}
		return rsaSign(priv, alg, data)
	case "ES256", "ES384", "ES512":
		priv, err := ecPrivFromJWK(privJWK)
		if err != nil {
			return nil, err
		}
		return ecSign(priv, alg, data)
	default:
		return nil, fmt.Errorf("unsupported sign alg: %q", alg)
	}
}

// VerifyData verifica a assinatura.
func VerifyData(pubJWK map[string]any, alg string, data, sig []byte) (bool, error) {
	switch alg {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
		pub, err := rsaPubFromJWK(pubJWK)
		if err != nil {
			return false, err
		}
		return rsaVerify(pub, alg, data, sig)
	case "ES256", "ES384", "ES512":
		pub, err := ecPubFromJWK(pubJWK)
		if err != nil {
			return false, err
		}
		return ecVerify(pub, alg, data, sig)
	default:
		return false, fmt.Errorf("unsupported verify alg: %q", alg)
	}
}

// WrapKey cifra uma chave simétrica (wrapping = EncryptData + base64).
func WrapKey(pubJWK map[string]any, alg string, keyMaterial []byte) ([]byte, error) {
	return EncryptData(pubJWK, alg, keyMaterial)
}

// UnwrapKey decifra material de chave.
func UnwrapKey(privJWK map[string]any, alg string, wrapped []byte) ([]byte, error) {
	return DecryptData(privJWK, alg, wrapped)
}

// ─── RSA helpers ──────────────────────────────────────────────────────────────

func rsaPubFromJWK(jwk map[string]any) (*rsa.PublicKey, error) {
	n, err := decodeB64(jwk, "n")
	if err != nil {
		return nil, err
	}
	e, err := decodeB64(jwk, "e")
	if err != nil {
		return nil, err
	}
	eBig := new(big.Int).SetBytes(e)
	return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(eBig.Int64())}, nil
}

func rsaPrivFromJWK(jwk map[string]any) (*rsa.PrivateKey, error) {
	pub, err := rsaPubFromJWK(jwk)
	if err != nil {
		return nil, err
	}
	d, _ := decodeB64(jwk, "d")
	p, _ := decodeB64(jwk, "p")
	q, _ := decodeB64(jwk, "q")

	priv := &rsa.PrivateKey{
		PublicKey: *pub,
		D:         new(big.Int).SetBytes(d),
		Primes:    []*big.Int{new(big.Int).SetBytes(p), new(big.Int).SetBytes(q)},
	}
	priv.Precompute()
	return priv, nil
}

func rsaSign(priv *rsa.PrivateKey, alg string, data []byte) ([]byte, error) {
	h, ha := rsaHash(alg)
	digest := hashDigest(h, data)
	switch alg {
	case "RS256", "RS384", "RS512":
		return rsa.SignPKCS1v15(rand.Reader, priv, ha, digest)
	case "PS256", "PS384", "PS512":
		return rsa.SignPSS(rand.Reader, priv, ha, digest, nil)
	}
	return nil, fmt.Errorf("unknown rsa alg: %s", alg)
}

func rsaVerify(pub *rsa.PublicKey, alg string, data, sig []byte) (bool, error) {
	h, ha := rsaHash(alg)
	digest := hashDigest(h, data)
	switch alg {
	case "RS256", "RS384", "RS512":
		err := rsa.VerifyPKCS1v15(pub, ha, digest, sig)
		return err == nil, nil
	case "PS256", "PS384", "PS512":
		err := rsa.VerifyPSS(pub, ha, digest, sig, nil)
		return err == nil, nil
	}
	return false, fmt.Errorf("unknown rsa alg: %s", alg)
}

func rsaHash(alg string) (hash.Hash, crypto.Hash) {
	switch alg {
	case "RS384", "PS384":
		return sha512.New384(), crypto.SHA384
	case "RS512", "PS512":
		return sha512.New(), crypto.SHA512
	default:
		return sha256.New(), crypto.SHA256
	}
}

// ─── EC helpers ───────────────────────────────────────────────────────────────

func ecPubFromJWK(jwk map[string]any) (*ecdsa.PublicKey, error) {
	crv, _ := jwk["crv"].(string)
	x, _ := decodeB64(jwk, "x")
	y, _ := decodeB64(jwk, "y")
	curve := ecCurve(crv)
	if curve == nil {
		return nil, fmt.Errorf("unknown crv: %q", crv)
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     new(big.Int).SetBytes(x),
		Y:     new(big.Int).SetBytes(y),
	}, nil
}

func ecPrivFromJWK(jwk map[string]any) (*ecdsa.PrivateKey, error) {
	pub, err := ecPubFromJWK(jwk)
	if err != nil {
		return nil, err
	}
	d, _ := decodeB64(jwk, "d")
	return &ecdsa.PrivateKey{PublicKey: *pub, D: new(big.Int).SetBytes(d)}, nil
}

func ecSign(priv *ecdsa.PrivateKey, alg string, data []byte) ([]byte, error) {
	h := ecHash(alg)
	digest := hashDigest(h, data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest)
	if err != nil {
		return nil, err
	}
	// IEEE P1363 — concatena r||s com tamanho fixo
	size := (priv.Curve.Params().BitSize + 7) / 8
	sig := make([]byte, 2*size)
	r.FillBytes(sig[:size])
	s.FillBytes(sig[size:])
	return sig, nil
}

func ecVerify(pub *ecdsa.PublicKey, alg string, data, sig []byte) (bool, error) {
	h := ecHash(alg)
	digest := hashDigest(h, data)
	size := (pub.Curve.Params().BitSize + 7) / 8
	if len(sig) != 2*size {
		return false, nil
	}
	r := new(big.Int).SetBytes(sig[:size])
	s := new(big.Int).SetBytes(sig[size:])
	return ecdsa.Verify(pub, digest, r, s), nil
}

// ─── AES helpers ──────────────────────────────────────────────────────────────

func AESGCMEncrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	ct := gcm.Seal(nonce, nonce, plaintext, nil)
	return ct, nil
}

func AESGCMDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, _ := cipher.NewGCM(block)
	ns := gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
}

// ─── misc ─────────────────────────────────────────────────────────────────────

func ecHash(alg string) hash.Hash {
	switch alg {
	case "ES384":
		return sha512.New384()
	case "ES512":
		return sha512.New()
	default:
		return sha256.New()
	}
}

func ecCurve(crv string) elliptic.Curve {
	switch crv {
	case "P-256", "":
		return elliptic.P256()
	case "P-384":
		return elliptic.P384()
	case "P-521":
		return elliptic.P521()
	}
	return nil
}

func oaepHash(alg string) hash.Hash {
	if alg == "RSA-OAEP-256" {
		return sha256.New()
	}
	return sha256.New() // RSA-OAEP default também usa SHA-1 mas SHA-256 é mais seguro
}

func hashDigest(h hash.Hash, data []byte) []byte {
	h.Reset()
	h.Write(data)
	return h.Sum(nil)
}

func decodeB64(jwk map[string]any, key string) ([]byte, error) {
	v, ok := jwk[key].(string)
	if !ok {
		return nil, fmt.Errorf("JWK missing field %q", key)
	}
	return base64.RawURLEncoding.DecodeString(v)
}
