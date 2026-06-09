package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/dilsonrabelo/kvemu/internal/adapters/persistence/sqlite"
	"github.com/dilsonrabelo/kvemu/internal/domain"
	"github.com/dilsonrabelo/kvemu/internal/ports"
)

type CertService struct {
	certRepo  *sqlite.CertRepo
	secretSvc *SecretService
	keySvc    *KeyService
	keyRepo   ports.KeyRepository
	vaultID   string
}

func NewCertService(certRepo *sqlite.CertRepo, secretSvc *SecretService, keySvc *KeyService, vaultID string) *CertService {
	return &CertService{certRepo: certRepo, secretSvc: secretSvc, keySvc: keySvc, keyRepo: keySvc.Repo, vaultID: vaultID}
}

// Create cria um certificado self-signed de acordo com a policy.
func (s *CertService) Create(ctx context.Context, name string, policy map[string]any, attrs domain.Attributes) (*domain.CertVersion, error) {
	ktyStr, crv, keySize := extractKeyProps(policy)
	keyOps := []string{"sign", "verify", "encrypt", "decrypt", "wrapKey", "unwrapKey"}

	// gera a key de suporte
	keyAttrs := domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable}
	backingKey, err := s.keySvc.Create(ctx, name, ktyStr, crv, keySize, keyOps, keyAttrs)
	if err != nil {
		return nil, fmt.Errorf("cert backing key: %w", err)
	}

	// gera o certificado X.509
	privJWK, err := s.keyRepo.GetPriv(ctx, s.vaultID, backingKey.Name, backingKey.Version)
	if err != nil {
		return nil, err
	}

	x509Props := extractX509Props(policy)
	certDER, err := s.generateSelfSigned(backingKey.PubJWK, privJWK, x509Props)
	if err != nil {
		return nil, fmt.Errorf("cert generation: %w", err)
	}

	// backing secret (PFX/PEM)
	secretAttrs := domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable}
	pemContent := encodeCertPEM(certDER)
	backingSecret, err := s.secretSvc.Set(ctx, name, pemContent, "application/x-pem-file", nil, secretAttrs)
	if err != nil {
		return nil, fmt.Errorf("cert backing secret: %w", err)
	}

	x5t, x5ts256 := thumbprints(certDER)

	cv := &domain.CertVersion{
		Name:             name,
		CerDER:           certDER,
		X5T:              x5t,
		X5TS256:          x5ts256,
		BackingKeyVer:    backingKey.Version,
		BackingSecretVer: backingSecret.Version,
		Attributes:       attrs,
	}
	if cv.Attributes.RecoveryLevel == "" {
		cv.Attributes.RecoveryLevel = domain.RecoveryLevelPurgeable
	}
	if err := s.certRepo.Upsert(ctx, s.vaultID, cv, policy); err != nil {
		return nil, err
	}
	return cv, nil
}

// Import importa um certificado PEM ou PFX.
func (s *CertService) Import(ctx context.Context, name string, certPEM string, policy map[string]any, attrs domain.Attributes) (*domain.CertVersion, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, domain.ErrBadParam{Msg: "invalid PEM content"}
	}

	x5t, x5ts256 := thumbprints(block.Bytes)

	backingSecret, err := s.secretSvc.Set(ctx, name, certPEM, "application/x-pem-file", nil,
		domain.Attributes{Enabled: true, RecoveryLevel: domain.RecoveryLevelPurgeable})
	if err != nil {
		return nil, err
	}

	cv := &domain.CertVersion{
		Name:             name,
		CerDER:           block.Bytes,
		X5T:              x5t,
		X5TS256:          x5ts256,
		BackingSecretVer: backingSecret.Version,
		Attributes:       attrs,
	}
	if cv.Attributes.RecoveryLevel == "" {
		cv.Attributes.RecoveryLevel = domain.RecoveryLevelPurgeable
	}
	if err := s.certRepo.Upsert(ctx, s.vaultID, cv, policy); err != nil {
		return nil, err
	}
	return cv, nil
}

func (s *CertService) Get(ctx context.Context, name, version string) (*domain.CertVersion, map[string]any, error) {
	if version == "" {
		return s.certRepo.GetCurrent(ctx, s.vaultID, name)
	}
	return s.certRepo.Get(ctx, s.vaultID, name, version)
}

func (s *CertService) GetPolicy(ctx context.Context, name string) (map[string]any, error) {
	return s.certRepo.GetPolicy(ctx, s.vaultID, name)
}

func (s *CertService) UpdatePolicy(ctx context.Context, name string, policy map[string]any) error {
	return s.certRepo.UpdatePolicy(ctx, s.vaultID, name, policy)
}

func (s *CertService) List(ctx context.Context, max int, tok string) ([]*domain.CertVersion, string, error) {
	return s.certRepo.List(ctx, s.vaultID, max, tok)
}

func (s *CertService) ListVersions(ctx context.Context, name string, max int, tok string) ([]*domain.CertVersion, string, error) {
	return s.certRepo.ListVersions(ctx, s.vaultID, name, max, tok)
}

func (s *CertService) Delete(ctx context.Context, name string) (*domain.DeletedCert, error) {
	schedPurge := time.Now().AddDate(0, 0, defaultRetentionDays).Unix()
	return s.certRepo.SoftDelete(ctx, s.vaultID, name, schedPurge)
}

func (s *CertService) GetDeleted(ctx context.Context, name string) (*domain.DeletedCert, error) {
	return s.certRepo.GetDeleted(ctx, s.vaultID, name)
}

func (s *CertService) ListDeleted(ctx context.Context, max int, tok string) ([]*domain.DeletedCert, string, error) {
	return s.certRepo.ListDeleted(ctx, s.vaultID, max, tok)
}

func (s *CertService) Recover(ctx context.Context, name string) (*domain.CertVersion, error) {
	return s.certRepo.Recover(ctx, s.vaultID, name)
}

func (s *CertService) Purge(ctx context.Context, name string) error {
	return s.certRepo.Purge(ctx, s.vaultID, name)
}

// ─── geração X.509 ────────────────────────────────────────────────────────────

type x509Props struct {
	Subject        string
	DNSNames       []string
	ValidityMonths int
}

func extractX509Props(policy map[string]any) x509Props {
	props := x509Props{Subject: "CN=kvemu", ValidityMonths: 12}
	x509, ok := policy["x509_props"].(map[string]any)
	if !ok {
		return props
	}
	if s, ok := x509["subject"].(string); ok {
		props.Subject = s
	}
	if m, ok := x509["validity_months"].(float64); ok {
		props.ValidityMonths = int(m)
	}
	if sans, ok := x509["sans"].(map[string]any); ok {
		if dns, ok := sans["dns_names"].([]any); ok {
			for _, d := range dns {
				if s, ok := d.(string); ok {
					props.DNSNames = append(props.DNSNames, s)
				}
			}
		}
	}
	return props
}

func extractKeyProps(policy map[string]any) (kty, crv string, keySize int) {
	kty = "RSA"
	keySize = 2048
	kp, ok := policy["key_props"].(map[string]any)
	if !ok {
		return
	}
	if k, ok := kp["kty"].(string); ok {
		kty = k
	}
	if s, ok := kp["key_size"].(float64); ok {
		keySize = int(s)
	}
	if c, ok := kp["crv"].(string); ok {
		crv = c
	}
	return
}

func (s *CertService) generateSelfSigned(pubJWK, privJWK map[string]any, props x509Props) ([]byte, error) {
	privKey, err := rsaPrivFromJWK(privJWK, pubJWK)
	if err != nil {
		return nil, err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: extractCN(props.Subject)},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(0, props.ValidityMonths, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     props.DNSNames,
	}
	return x509.CreateCertificate(rand.Reader, tmpl, tmpl, &privKey.PublicKey, privKey)
}

func rsaPrivFromJWK(priv, pub map[string]any) (*rsa.PrivateKey, error) {
	merged := make(map[string]any)
	for k, v := range pub {
		merged[k] = v
	}
	for k, v := range priv {
		merged[k] = v
	}
	dec := func(k string) []byte {
		s, _ := merged[k].(string)
		b, _ := base64.RawURLEncoding.DecodeString(s)
		return b
	}
	n := new(big.Int).SetBytes(dec("n"))
	e := new(big.Int).SetBytes(dec("e"))
	d := new(big.Int).SetBytes(dec("d"))
	p := new(big.Int).SetBytes(dec("p"))
	q := new(big.Int).SetBytes(dec("q"))
	key := &rsa.PrivateKey{
		PublicKey: rsa.PublicKey{N: n, E: int(e.Int64())},
		D:         d,
		Primes:    []*big.Int{p, q},
	}
	key.Precompute()
	return key, nil
}

func extractCN(subject string) string {
	for _, part := range strings.Split(subject, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToUpper(part), "CN=") {
			return part[3:]
		}
	}
	return subject
}

func thumbprints(certDER []byte) (x5t, x5ts256 string) {
	h1 := sha1.Sum(certDER)
	h2 := sha256.Sum256(certDER)
	x5t = base64.RawURLEncoding.EncodeToString(h1[:])
	x5ts256 = base64.RawURLEncoding.EncodeToString(h2[:])
	return
}

func encodeCertPEM(certDER []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
}

// ─── helpers (SANs IP) ────────────────────────────────────────────────────────

var _ = net.ParseIP // garante import usado em futuras extensões de SAN

// certToJSON monta o objeto CertificateBundle para a resposta.
func CertToJSON(cv *domain.CertVersion, policy map[string]any, vaultHost string) map[string]any {
	cerB64 := base64.StdEncoding.EncodeToString(cv.CerDER)

	out := map[string]any{
		"id":  domain.ID(vaultHost, "certificates", cv.Name, cv.Version),
		"cer": cerB64,
		"x5t": cv.X5T,
		"attributes": cv.Attributes,
	}
	if cv.BackingKeyVer != "" {
		out["kid"] = domain.ID(vaultHost, "keys", cv.Name, cv.BackingKeyVer)
	}
	if cv.BackingSecretVer != "" {
		out["sid"] = domain.ID(vaultHost, "secrets", cv.Name, cv.BackingSecretVer)
	}
	if policy != nil {
		policyWithID := make(map[string]any)
		for k, v := range policy {
			policyWithID[k] = v
		}
		policyWithID["id"] = domain.ID(vaultHost, "certificates", cv.Name, "") + "/policy"
		out["policy"] = policyWithID
	}
	return out
}

// policyJSON retorna bytes da policy para uso em respostas.
func PolicyJSON(policy map[string]any, vaultHost, name string) map[string]any {
	if policy == nil {
		policy = map[string]any{}
	}
	out := make(map[string]any, len(policy)+1)
	for k, v := range policy {
		out[k] = v
	}
	out["id"] = domain.ID(vaultHost, "certificates", name, "") + "/policy"
	return out
}

// jsonRoundTrip usa json marshal/unmarshal para converter struct em map.
func jsonRoundTrip(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	json.Unmarshal(b, &m)
	return m
}
