package validation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/trustctl/trustctl/internal/dns"
)

type Validator struct {
	vtype       string
	dnsProvider dns.DNSProvider
}

func NewValidator(vtype string, provider dns.DNSProvider) *Validator {
	return &Validator{vtype: vtype, dnsProvider: provider}
}

// Validate performs validation for provided domains according to vtype.
func (v *Validator) Validate(domains []string) error {
	switch v.vtype {
	case "dns":
		if v.dnsProvider == nil {
			return errors.New("dns provider not configured")
		}
		return v.doDNS(domains)
	case "http":
		return v.doHTTP(domains)
	case "email":
		return errors.New("email validation not implemented yet")
	default:
		return fmt.Errorf("unknown validation type: %s", v.vtype)
	}
}

func (v *Validator) doDNS(domains []string) error {
	// Parallel Present
	var wg sync.WaitGroup
	errs := make(chan error, len(domains))
	for _, d := range domains {
		wg.Add(1)
		go func(domain string) {
			defer wg.Done()
			token := "acme-token"
			keyAuth := "key-auth"
			if err := v.dnsProvider.Present(domain, token, keyAuth); err != nil {
				errs <- err
				return
			}
		}(d)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		return e
	}

	// Wait for propagation (simple fixed sleep for scaffold)
	time.Sleep(5 * time.Second)

	// Cleanup should be handled after issuance; for scaffold, perform cleanup now
	for _, d := range domains {
		_ = v.dnsProvider.CleanUp(d, "acme-token", "key-auth")
	}
	return nil
}

func (v *Validator) doHTTP(domains []string) error {
	// Place challenge token under /.well-known/acme-challenge/<token>
	base := "/var/www/html/.well-known/acme-challenge"
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}
	for _, d := range domains {
		tokenFile := filepath.Join(base, fmt.Sprintf("%s.token", d))
		if err := os.WriteFile(tokenFile, []byte("token-placeholder"), 0644); err != nil {
			return err
		}
	}
	// Give user/ACME client time to validate
	time.Sleep(2 * time.Second)
	return nil
}
