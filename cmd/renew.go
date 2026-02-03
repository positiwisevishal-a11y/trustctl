package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trustctl/trustctl/internal/ca"
	"github.com/trustctl/trustctl/internal/creds"
	"github.com/trustctl/trustctl/internal/dns"
	"github.com/trustctl/trustctl/internal/metadata"
	"github.com/trustctl/trustctl/internal/ui"
	"github.com/trustctl/trustctl/internal/validation"
)

var renewCmd = &cobra.Command{
	Use:   "renew",
	Short: "Renew certificates for registered domains",
	Long:  "Automatically renew certificates using stored metadata (domains, validation method, credentials, installer type)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.StepStart("Checking for certificates to renew...")

		domains, err := metadata.ListAll()
		if err != nil {
			ui.Error("failed to list certificates: %v", err)
			return fmt.Errorf("failed to list certificates: %w", err)
		}
		if len(domains) == 0 {
			ui.Warning("No certificates found for renewal")
			return nil
		}

		ui.Info("Found %d certificate(s) to check for renewal", len(domains))

		for _, domain := range domains {
			if err := renewDomain(domain); err != nil {
				ui.Error("renewal failed for %s: %v", domain, err)
				// Continue with next domain instead of stopping
			}
		}

		ui.Success("Renewal check complete")
		return nil
	},
}

func renewDomain(domain string) error {
	ui.StepStart("Renewing certificate for %s", domain)

	// Load metadata
	meta, err := metadata.Load(domain)
	if err != nil {
		return fmt.Errorf("failed to load metadata for %s: %w", domain, err)
	}

	ui.Info("Validation method: %s | Domains: %s | CA: %s",
		meta.ValidationMethod, strings.Join(meta.Domains, ","),
		func() string {
			if meta.ServerURL != "" {
				return meta.ServerURL
			}
			return "Let's Encrypt"
		}())

	// Verify credentials exist
	if err := creds.AssertPermissions(meta.CredentialsPath); err != nil {
		return fmt.Errorf("credentials check failed: %w", err)
	}
	ui.StepDone("Credentials verified")

	// Resolve CA using stored settings
	resolver := ca.NewResolver(meta.CredentialsPath)
	caClient, err := resolver.Resolve(meta.ServerURL, meta.HMACIDCred, "")
	if err != nil {
		return fmt.Errorf("CA resolution failed: %w", err)
	}

	// Setup validation using stored method
	var dnsProvider dns.DNSProvider
	if meta.ValidationMethod == "dns" {
		if meta.DNSProvider == "" {
			return fmt.Errorf("dns validation configured but no dns_provider in metadata")
		}
		ui.StepStart("Loading DNS provider: %s", meta.DNSProvider)
		loader := dns.NewPluginLoader(pluginsPath, meta.CredentialsPath)
		dnsProvider, err = loader.Load(meta.DNSProvider)
		if err != nil {
			return fmt.Errorf("failed to load dns provider: %w", err)
		}
		ui.Success("DNS provider loaded")
	}

	// Validate domains
	ui.StepStart("Validating domains for renewal...")
	validator := validation.NewValidator(meta.ValidationMethod, dnsProvider)
	if err := validator.Validate(meta.Domains); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	ui.Success("Validation successful")

	// Request renewed certificate
	ui.StepStart("Requesting renewed certificate...")
	certMeta, err := caClient.RequestCertificate(meta.Domains)
	if err != nil {
		return fmt.Errorf("certificate request failed: %w", err)
	}
	ui.Success("Certificate renewed by %s", certMeta.Issuer)

	// Install renewed certificate
	ui.StepStart("Installing renewed certificate...")
	if err := ca.InstallCertificate(certMeta); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	ui.Success("Certificate reinstalled")

	// Update metadata with renewal timestamp
	meta.LastRenewalAt = time.Now()
	meta.RenewalAttempts++
	if err := meta.Store(); err != nil {
		ui.Warning("failed to update renewal metadata: %v", err)
	}

	ui.Success("Renewal complete for %s", domain)
	return nil
}

func init() {
	rootCmd.AddCommand(renewCmd)
}
