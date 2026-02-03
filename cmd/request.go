package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trustctl/trustctl/internal/ca"
	"github.com/trustctl/trustctl/internal/creds"
	"github.com/trustctl/trustctl/internal/dns"
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

		// Load credentials directory; ensure permissions
		if err := creds.AssertPermissions(credentialsPath); err != nil {
			return fmt.Errorf("credentials permission check failed: %w", err)
		}

		// Resolve CA
		resolver := ca.NewResolver(credentialsPath)
		caClient, err := resolver.Resolve(serverURLFlag, hmacIDFlag, hmacKeyFlag)
		if err != nil {
			return fmt.Errorf("CA resolution failed: %w", err)
		}

		// Detect validation method
		vtype := strings.ToLower(validationFlag)
		if vtype == "" {
			vtype = "http"
		}

		// DNS plugin loader (only needed for dns validation)
		var dnsProvider dns.DNSProvider
		if vtype == "dns" {
			if dnsProviderFlag == "" {
				return errors.New("--dns-provider is required for dns validation")
			}
			loader := dns.NewPluginLoader(pluginsPath, credentialsPath)
			dnsProvider, err = loader.Load(dnsProviderFlag)
			if err != nil {
				return fmt.Errorf("failed to load dns provider: %w", err)
			}
		}

		// Run validation
		validator := validation.NewValidator(vtype, dnsProvider)
		if err := validator.Validate(domains); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		// Request certificate from CA
		certMeta, err := caClient.RequestCertificate(domains)
		if err != nil {
			return fmt.Errorf("certificate request failed: %w", err)
		}

		// Install certificate (installer is a stub for now)
		if err := ca.InstallCertificate(certMeta); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}

		fmt.Println("Certificate issued and installed for:", strings.Join(domains, ","))
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
