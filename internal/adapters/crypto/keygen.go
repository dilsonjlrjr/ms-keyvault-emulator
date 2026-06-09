package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

// GenerateKeyPair gera um par de chaves e retorna (pubJWK, privJWK, error).
// kty: RSA | EC | oct
// crv: P-256 | P-384 | P-521 | P-256K  (EC)
// keySize: 2048 | 3072 | 4096           (RSA)
func GenerateKeyPair(kty, crv string, keySize int) (pubJWK, privJWK map[string]any, err error) {
	switch kty {
	case "RSA", "RSA-HSM":
		return generateRSA(keySize)
	case "EC", "EC-HSM":
		return generateEC(crv)
	case "oct", "oct-HSM":
		return generateOct(keySize)
	default:
		return nil, nil, fmt.Errorf("unsupported kty: %q", kty)
	}
}

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func generateRSA(bits int) (map[string]any, map[string]any, error) {
	if bits == 0 {
		bits = 2048
	}
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	pub := map[string]any{
		"kty": "RSA",
		"n":   b64url(k.N.Bytes()),
		"e":   b64url(big.NewInt(int64(k.E)).Bytes()),
	}
	priv := map[string]any{
		"d":  b64url(k.D.Bytes()),
		"p":  b64url(k.Primes[0].Bytes()),
		"q":  b64url(k.Primes[1].Bytes()),
		"dp": b64url(k.Precomputed.Dp.Bytes()),
		"dq": b64url(k.Precomputed.Dq.Bytes()),
		"qi": b64url(k.Precomputed.Qinv.Bytes()),
	}
	return pub, priv, nil
}

func generateEC(crv string) (map[string]any, map[string]any, error) {
	var curve elliptic.Curve
	switch crv {
	case "P-256", "":
		curve = elliptic.P256()
	case "P-384":
		curve = elliptic.P384()
	case "P-521":
		curve = elliptic.P521()
	default:
		return nil, nil, fmt.Errorf("unsupported crv: %q", crv)
	}
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pub := map[string]any{
		"kty": "EC",
		"crv": crv,
		"x":   b64url(k.X.Bytes()),
		"y":   b64url(k.Y.Bytes()),
	}
	priv := map[string]any{
		"d": b64url(k.D.Bytes()),
	}
	return pub, priv, nil
}

func generateOct(bits int) (map[string]any, map[string]any, error) {
	if bits == 0 {
		bits = 256
	}
	k := make([]byte, bits/8)
	if _, err := rand.Read(k); err != nil {
		return nil, nil, err
	}
	pub := map[string]any{"kty": "oct"}
	priv := map[string]any{"k": b64url(k)}
	return pub, priv, nil
}

// ImportJWK importa um JWK externo e separa pub/priv.
func ImportJWK(jwk map[string]any) (pubJWK, privJWK map[string]any, err error) {
	kty, _ := jwk["kty"].(string)
	pub := map[string]any{"kty": kty}
	priv := map[string]any{}

	switch kty {
	case "RSA":
		for _, f := range []string{"n", "e"} {
			pub[f] = jwk[f]
		}
		for _, f := range []string{"d", "p", "q", "dp", "dq", "qi"} {
			if v, ok := jwk[f]; ok {
				priv[f] = v
			}
		}
	case "EC":
		pub["crv"] = jwk["crv"]
		for _, f := range []string{"x", "y"} {
			pub[f] = jwk[f]
		}
		if v, ok := jwk["d"]; ok {
			priv["d"] = v
		}
	case "oct":
		if v, ok := jwk["k"]; ok {
			priv["k"] = v
		}
	default:
		return nil, nil, fmt.Errorf("unsupported kty: %q", kty)
	}
	return pub, priv, nil
}
