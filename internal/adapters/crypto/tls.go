package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TLSBundle struct {
	Cert    tls.Certificate
	CACert  *x509.Certificate
	CAPEM   []byte
	LeafPEM []byte
}

// GenerateOrLoad gera CA+leaf se não existirem; senão carrega do disco.
func GenerateOrLoad(certDir, vaultHost, extraSANs, baseDomain string) (*TLSBundle, error) {
	caPath := filepath.Join(certDir, "ca.pem")
	caKeyPath := filepath.Join(certDir, "ca-key.pem")
	leafPath := filepath.Join(certDir, "leaf.pem")
	leafKeyPath := filepath.Join(certDir, "leaf-key.pem")

	if _, err := os.Stat(caPath); err == nil {
		return loadBundle(caPath, leafPath, leafKeyPath)
	}

	if err := os.MkdirAll(certDir, 0700); err != nil {
		return nil, err
	}
	return generate(caPath, caKeyPath, leafPath, leafKeyPath, vaultHost, extraSANs, baseDomain)
}

// LoadBYO carrega cert+key PEM fornecidos externamente.
func LoadBYO(certPEM, keyPEM, caPEM string) (*TLSBundle, error) {
	cert, err := tls.LoadX509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	b := &TLSBundle{Cert: cert}
	if caPEM != "" {
		data, err := os.ReadFile(caPEM)
		if err != nil {
			return nil, err
		}
		b.CAPEM = data
	}
	return b, nil
}

func generate(caPath, caKeyPath, leafPath, leafKeyPath, vaultHost, extraSANs, baseDomain string) (*TLSBundle, error) {
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	caSerial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	caTmpl := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{CommonName: "kvemu-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	dns, ips := buildSANs(vaultHost, extraSANs, baseDomain)

	leafSerial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	leafTmpl := &x509.Certificate{
		SerialNumber: leafSerial,
		Subject:      pkix.Name{CommonName: primaryDNS(dns)},
		DNSNames:     dns,
		IPAddresses:  ips,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(caKey)})
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	leafKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)})

	if err := os.WriteFile(caPath, caPEM, 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(leafPath, leafPEM, 0600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(leafKeyPath, leafKeyPEM, 0600); err != nil {
		return nil, err
	}

	tlsCert, err := tls.X509KeyPair(leafPEM, leafKeyPEM)
	if err != nil {
		return nil, err
	}
	return &TLSBundle{Cert: tlsCert, CACert: caCert, CAPEM: caPEM, LeafPEM: leafPEM}, nil
}

func loadBundle(caPath, leafPath, leafKeyPath string) (*TLSBundle, error) {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(caPEM)
	caCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	tlsCert, err := tls.LoadX509KeyPair(leafPath, leafKeyPath)
	if err != nil {
		return nil, err
	}
	leafPEM, _ := os.ReadFile(leafPath)
	return &TLSBundle{Cert: tlsCert, CACert: caCert, CAPEM: caPEM, LeafPEM: leafPEM}, nil
}

func buildSANs(vaultHost, extras, baseDomain string) ([]string, []net.IP) {
	seen := map[string]bool{}
	dns := []string{}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		host := s
		if h, _, err := net.SplitHostPort(s); err == nil {
			host = h
		}
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
		} else {
			dns = append(dns, host)
		}
	}

	add(vaultHost)
	add("localhost")
	add("kvemu")
	for _, e := range strings.Split(extras, ",") {
		add(e)
	}
	if baseDomain != "" {
		add("*." + baseDomain)
	}
	return dns, ips
}

func primaryDNS(dns []string) string {
	if len(dns) > 0 {
		return dns[0]
	}
	return "localhost"
}
