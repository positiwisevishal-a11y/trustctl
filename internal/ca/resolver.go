package ca

import (
	"errors"
	"fmt"
	"time"
)

// CertificateMeta holds certificate metadata and files locations.
type CertificateMeta struct {
	Domains []string
	PEM     []byte
	Key     []byte
	Issuer  string
}

// CAClient represents a CA implementation (Let's Encrypt or Enterprise)
type CAClient interface {
	RequestCertificate(domains []string) (*CertificateMeta, error)
}

// Resolver chooses CA implementation based on flags/credentials
type Resolver struct {
	credsDir string
}

func NewResolver(credsDir string) *Resolver {
	return &Resolver{credsDir: credsDir}
}

// Resolve chooses LE (ACME v2) if serverURL is empty, else returns an enterprise client.
func (r *Resolver) Resolve(serverURL, hmacID, hmacKey string) (CAClient, error) {
	if serverURL == "" {
		// Default to Let's Encrypt ACME v2 client (scaffold)
		return &letsencryptClient{}, nil
	}
	if hmacID == "" || hmacKey == "" {
		return nil, errors.New("hmac-id and hmac-key are required for enterprise CA")
	}
	// Return an enterprise client that communicates with the provided server
	return &enterpriseClient{serverURL: serverURL, hmacID: hmacID, hmacKey: hmacKey}, nil
}

type letsencryptClient struct{}

func (l *letsencryptClient) RequestCertificate(domains []string) (*CertificateMeta, error) {
	// Here one would integrate with an ACME library (e.g. lego) to actually request certs.
	// This scaffold returns placeholder data.
	return &CertificateMeta{Domains: domains, PEM: []byte("---BEGIN CERT---\n..."), Key: []byte("---KEY---"), Issuer: "Let's Encrypt"}, nil
}

type enterpriseClient struct {
	serverURL string
	hmacID    string
	hmacKey   string
}

func (e *enterpriseClient) RequestCertificate(domains []string) (*CertificateMeta, error) {
	// Implement HMAC authenticated REST calls to the enterprise CA (Sectigo/DigiCert).
	// Scaffold: simulate a request and response.
	time.Sleep(1 * time.Second)
	if e.serverURL == "" {
		return nil, fmt.Errorf("serverURL required for enterprise client")
	}
	return &CertificateMeta{Domains: domains, PEM: []byte("---BEGIN CERT ENTERPRISE---\n..."), Key: []byte("---KEY---"), Issuer: "EnterpriseCA"}, nil
}

// InstallCertificate persists the certificate into the file system atomically and returns error on failure.
func InstallCertificate(meta *CertificateMeta) error {
	// Production implementation must atomically replace certs and support rollback.
	// This is a scaffold that prints where it would write certs.
	if meta == nil {
		return errors.New("nil certificate meta")
	}
	// In a real implementation write to /opt/trustctl/certs/<domain>/ with chmod 0700 and owner root.
	fmt.Printf("install: would install cert for %v issued by %s\n", meta.Domains, meta.Issuer)
	return nil
}
