package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
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

var (
	domainsFlag     string
	validationFlag  string
	dnsProviderFlag string
	serverURLFlag   string
	hmacIDFlag      string
	hmacKeyFlag     string
	credentialsPath = "/opt/trustctl/credentials"
	pluginsPath     = "/opt/trustctl/plugins"
)

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request a certificate",
	RunE: func(cmd *cobra.Command, args []string) error {
		if domainsFlag == "" {
			return errors.New("--domains is required")
		}

		domains := strings.Split(domainsFlag, ",")
		for i := range domains {
			domains[i] = strings.TrimSpace(domains[i])
		}

		ui.Info("Checking credential permissions in %s...", credentialsPath)
		// Load credentials directory; ensure permissions
		if err := creds.AssertPermissions(credentialsPath); err != nil {
			ui.Error("credentials permission check failed: %v", err)
			return fmt.Errorf("credentials permission check failed: %w", err)
		}

		ui.StepStart("Resolving Certificate Authority...")
		// Resolve CA
		resolver := ca.NewResolver(credentialsPath)
		caClient, err := resolver.Resolve(serverURLFlag, hmacIDFlag, hmacKeyFlag)
		if err != nil {
			ui.Error("CA resolution failed: %v", err)
			return fmt.Errorf("CA resolution failed: %w", err)
		}
		if serverURLFlag == "" {
			ui.Info("Using Let's Encrypt (ACME v2) as default CA")
		} else {
			ui.Info("Using enterprise CA: %s (HMAC auth)", serverURLFlag)
		}
		ui.StepDone("CA resolved")

		// Detect validation method
		vtype := strings.ToLower(validationFlag)
		if vtype == "" {
			vtype = "http"
		}

		// DNS plugin loader (only needed for dns validation)
		var dnsProvider dns.DNSProvider
		if vtype == "dns" {
			if dnsProviderFlag == "" {
				ui.Error("--dns-provider is required for dns validation")
				return errors.New("--dns-provider is required for dns validation")
			}
			ui.StepStart("Loading DNS provider plugin: %s", dnsProviderFlag)
			loader := dns.NewPluginLoader(pluginsPath, credentialsPath)
			dnsProvider, err = loader.Load(dnsProviderFlag)
			if err != nil {
				ui.Error("failed to load dns provider: %v", err)
				return fmt.Errorf("failed to load dns provider: %w", err)
			}
			ui.Success("Loaded DNS provider: %s", dnsProviderFlag)
		}

		// Run validation
		ui.StepStart("Starting %s validation for %s", strings.ToUpper(vtype), strings.Join(domains, ","))
		validator := validation.NewValidator(vtype, dnsProvider)
		if err := validator.Validate(domains); err != nil {
			ui.Error("validation failed: %v", err)
			return fmt.Errorf("validation failed: %w", err)
		}
		ui.Success("Validation successful for: %s", strings.Join(domains, ","))

		// Request certificate from CA
		ui.StepStart("Requesting certificate from CA...")
		certMeta, err := caClient.RequestCertificate(domains)
		if err != nil {
			ui.Error("certificate request failed: %v", err)
			return fmt.Errorf("certificate request failed: %w", err)
		}
		ui.Success("Certificate issued by %s", certMeta.Issuer)

		// Install certificate (installer is a stub for now)
		ui.StepStart("Installing certificate for %s", strings.Join(domains, ","))
		if err := ca.InstallCertificate(certMeta); err != nil {
			ui.Error("installation failed: %v", err)
			return fmt.Errorf("installation failed: %w", err)
		}
		ui.Success("Certificate installed for: %s", strings.Join(domains, ","))

		// Save metadata for renewal
		ui.StepStart("Saving certificate metadata for renewal...")
		meta := &metadata.CertMetadata{
			Domains:          domains,
			ValidationMethod: vtype,
			DNSProvider:      dnsProviderFlag,
			ServerURL:        serverURLFlag,
			HMACIDCred:       hmacIDFlag,
			CredentialsPath:  credentialsPath,
			CertPath:         "/opt/trustctl/certs/" + domains[0] + "/fullchain.pem",
			KeyPath:          "/opt/trustctl/certs/" + domains[0] + "/privkey.pem",
			IssuedAt:         time.Now(),
			RenewalAttempts:  0,
		}
		if err := meta.Store(); err != nil {
			ui.Warning("failed to save metadata: %v", err)
		} else {
			ui.Success("Metadata saved for renewal")
		}
		return nil
	},
}

func init() {
	requestCmd.Flags().StringVar(&domainsFlag, "domains", "", "Comma-separated domains (required)")
	requestCmd.Flags().StringVar(&validationFlag, "validation", "", "Validation method: dns|http|email (default http)")
	requestCmd.Flags().StringVar(&dnsProviderFlag, "dns-provider", "", "DNS provider name (for dns validation)")
	requestCmd.Flags().StringVar(&serverURLFlag, "serverurl", "", "Enterprise CA server URL (optional)")
	requestCmd.Flags().StringVar(&hmacIDFlag, "hmac-id", "", "HMAC ID for enterprise CA (optional)")
	requestCmd.Flags().StringVar(&hmacKeyFlag, "hmac-key", "", "HMAC key for enterprise CA (optional)")

	rootCmd.AddCommand(requestCmd)

	// Ensure logs directory exists
	if err := os.MkdirAll("/opt/trustctl/logs", 0700); err != nil {
		log.Println("warning: couldn't create logs dir:", err)
	}
}
