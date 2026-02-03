package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CertMetadata stores the configuration and state for a certificate for renewal.
type CertMetadata struct {
	Domains          []string  `json:"domains"`
	ValidationMethod string    `json:"validation_method"` // http, dns, email
	DNSProvider      string    `json:"dns_provider,omitempty"`
	ServerURL        string    `json:"server_url,omitempty"`
	HMACIDCred       string    `json:"hmac_id_cred,omitempty"` // path to creds file
	CredentialsPath  string    `json:"credentials_path"`
	InstallerType    string    `json:"installer_type,omitempty"` // nginx, apache, tomcat
	CertPath         string    `json:"cert_path"`
	KeyPath          string    `json:"key_path"`
	ChainPath        string    `json:"chain_path,omitempty"`
	IssuedAt         time.Time `json:"issued_at"`
	ExpiresAt        time.Time `json:"expires_at,omitempty"`
	RenewalAttempts  int       `json:"renewal_attempts"`
	LastRenewalAt    time.Time `json:"last_renewal_at,omitempty"`
}

// Store saves metadata to a JSON file in /opt/trustctl/certs/<domain>/metadata.json
func (m *CertMetadata) Store() error {
	if len(m.Domains) == 0 {
		return fmt.Errorf("no domains in metadata")
	}
	primaryDomain := m.Domains[0]
	metadataDir := filepath.Join("/opt/trustctl/certs", primaryDomain)
	if err := os.MkdirAll(metadataDir, 0700); err != nil {
		return err
	}
	metadataFile := filepath.Join(metadataDir, "metadata.json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metadataFile, data, 0600); err != nil {
		return err
	}
	return nil
}

// Load loads metadata from /opt/trustctl/certs/<domain>/metadata.json
func Load(domain string) (*CertMetadata, error) {
	metadataFile := filepath.Join("/opt/trustctl/certs", domain, "metadata.json")
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, err
	}
	var m CertMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListAll returns all domains that have stored certificates/metadata
func ListAll() ([]string, error) {
	certsDir := "/opt/trustctl/certs"
	entries, err := os.ReadDir(certsDir)
	if err != nil {
		return nil, err
	}
	var domains []string
	for _, e := range entries {
		if e.IsDir() {
			// Check if metadata.json exists
			if _, err := os.Stat(filepath.Join(certsDir, e.Name(), "metadata.json")); err == nil {
				domains = append(domains, e.Name())
			}
		}
	}
	return domains, nil
}
