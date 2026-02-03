package account

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AccountInfo stores ACME account credentials (for Let's Encrypt or other ACME-compliant CAs)
type AccountInfo struct {
	CA            string    `json:"ca"` // e.g., "letsencrypt", "sectigo"
	Email         string    `json:"email"`
	AccountURL    string    `json:"account_url"`
	AccountKey    string    `json:"account_key"` // path to account private key
	CreatedAt     time.Time `json:"created_at"`
	LastUpdatedAt time.Time `json:"last_updated_at"`
}

// Store saves account info to /opt/trustctl/credentials/<ca>-account.json with chmod 600
func (a *AccountInfo) Store() error {
	if a.CA == "" {
		return fmt.Errorf("CA name required")
	}

	credDir := "/opt/trustctl/credentials"
	if err := os.MkdirAll(credDir, 0700); err != nil {
		return err
	}

	accountFile := filepath.Join(credDir, fmt.Sprintf("%s-account.json", a.CA))
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}

	// Write with restricted permissions
	if err := os.WriteFile(accountFile, data, 0600); err != nil {
		return err
	}

	return nil
}

// Load loads account info from /opt/trustctl/credentials/<ca>-account.json
func Load(ca string) (*AccountInfo, error) {
	accountFile := filepath.Join("/opt/trustctl/credentials", fmt.Sprintf("%s-account.json", ca))
	data, err := os.ReadFile(accountFile)
	if err != nil {
		return nil, fmt.Errorf("account file not found for CA %s: %w", ca, err)
	}

	var a AccountInfo
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}

	return &a, nil
}

// Exists checks if account info exists for a CA
func Exists(ca string) bool {
	accountFile := filepath.Join("/opt/trustctl/credentials", fmt.Sprintf("%s-account.json", ca))
	_, err := os.Stat(accountFile)
	return err == nil
}

// Create creates a new account (scaffold - will integrate with ACME library)
func Create(ca, email string) (*AccountInfo, error) {
	if ca == "" || email == "" {
		return nil, fmt.Errorf("CA name and email required")
	}

	account := &AccountInfo{
		CA:        ca,
		Email:     email,
		CreatedAt: time.Now(),
	}

	// In production, integrate with lego or similar to register account with ACME server
	// For now, scaffold returns account ready to be used
	account.AccountURL = "https://acme-v02.api.letsencrypt.org/acme/acct/12345" // placeholder
	account.AccountKey = filepath.Join("/opt/trustctl/credentials", ca+"-account-key.pem")

	return account, nil
}
