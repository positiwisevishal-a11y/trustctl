package keygen

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// GeneratePrivateKey creates a 2048-bit RSA private key
func GeneratePrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// SavePrivateKey saves RSA private key to PEM file with chmod 600
func SavePrivateKey(key *rsa.PrivateKey, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	privKeyBytes := x509.MarshalPKCS1PrivateKey(key)
	privKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privKeyBytes,
	})

	// Write with restricted permissions
	if err := os.WriteFile(path, privKeyPEM, 0600); err != nil {
		return err
	}
	return nil
}

// GenerateCSR creates a Certificate Signing Request for domains
func GenerateCSR(key *rsa.PrivateKey, domains []string) ([]byte, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one domain required for CSR")
	}

	template := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: domains[0],
		},
		DNSNames: domains,
	}

	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, key)
	if err != nil {
		return nil, err
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrBytes,
	})

	return csrPEM, nil
}

// SaveCSR saves CSR to file (informational, not required by trustctl)
func SaveCSR(csr []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(path, csr, 0644)
}

// LoadPrivateKey loads a PEM-encoded private key from file
func LoadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return key, nil
}
