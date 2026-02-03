package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/trustctl/trustctl/internal/account"
	"github.com/trustctl/trustctl/internal/ca"
	"github.com/trustctl/trustctl/internal/creds"
	"github.com/trustctl/trustctl/internal/dns"
	"github.com/trustctl/trustctl/internal/keygen"
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
	webrootFlag     string
	emailFlag       string
	credentialsPath = "/opt/trustctl/credentials"
	pluginsPath     = "/opt/trustctl/plugins"
	certsPath       = "/opt/trustctl/certs"
)

var requestCmd = &cobra.Command{
	Use:   "request",
	Short: "Request a certificate (like certbot)",
	Long:  "Request and install a certificate, auto-generating keys and storing account credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if domainsFlag == "" {
			return errors.New("--domains is required")
		}

		domains := strings.Split(domainsFlag, ",")
		for i := range domains {
			domains[i] = strings.TrimSpace(domains[i])
		}

		primaryDomain := domains[0]
		certDir := fmt.Sprintf("%s/%s", certsPath, primaryDomain)

		ui.StepStart("🤝 trustctl - Certificate Automation Agent")
		ui.Info("Processing %d domain(s): %s", len(domains), strings.Join(domains, ", "))

		// Setup directory structure
		ui.StepStart("Creating certificate directory: %s", certDir)
		if err := os.MkdirAll(certDir, 0700); err != nil {
			ui.Error("failed to create cert directory: %v", err)
			return err
		}
		ui.Success("Directory created with chmod 700")

		// Generate private key
		ui.StepStart("Generating 2048-bit RSA private key...")
		privateKey, err := keygen.GeneratePrivateKey()
		if err != nil {
			ui.Error("failed to generate private key: %v", err)
			return err
		}

		keyPath := fmt.Sprintf("%s/privkey.pem", certDir)
		if err := keygen.SavePrivateKey(privateKey, keyPath); err != nil {
			ui.Error("failed to save private key: %v", err)
			return err
		}
		ui.Success("Private key saved: %s (chmod 600)", keyPath)

		// Generate CSR
		ui.StepStart("Generating Certificate Signing Request (CSR)...")
		csr, err := keygen.GenerateCSR(privateKey, domains)
		if err != nil {
			ui.Error("failed to generate CSR: %v", err)
			return err
		}

		csrPath := fmt.Sprintf("%s/csr.pem", certDir)
		if err := keygen.SaveCSR(csr, csrPath); err != nil {
			ui.Error("failed to save CSR: %v", err)
			return err
		}
		ui.Success("CSR generated and saved: %s", csrPath)

		// Setup HTTP validation
		if vtype := strings.ToLower(validationFlag); vtype == "" || vtype == "http" {
			if webrootFlag == "" {
				webrootFlag = "/var/www/html"
			}
			ui.StepStart("Setting up HTTP validation with webroot: %s", webrootFlag)
			challengeDir := fmt.Sprintf("%s/.well-known/acme-challenge", webrootFlag)
			if err := os.MkdirAll(challengeDir, 0755); err != nil {
				ui.Error("failed to create challenge directory: %v", err)
				return err
			}
			ui.Success("Challenge directory ready: %s", challengeDir)
		}

		// Check/create account credentials
		caName := "letsencrypt"
		if serverURLFlag != "" {
			caName = "enterprise-ca"
		}

		ui.StepStart("Checking %s account...", caName)
		var acc *account.AccountInfo
		if account.Exists(caName) {
			ui.Info("Account found for %s", caName)
			acc, _ = account.Load(caName)
		} else {
			ui.StepStart("Creating new %s account...", caName)
			if emailFlag == "" {
				emailFlag = "admin@" + primaryDomain
			}
			acc, err = account.Create(caName, emailFlag)
			if err != nil {
				ui.Error("failed to create account: %v", err)
				return err
			}
			if err := acc.Store(); err != nil {
				ui.Error("failed to store account: %v", err)
				return err
			}
			ui.Success("Account created and stored: %s", acc.AccountURL)
		}

		ui.Info("Checking credential permissions...")
		if err := creds.AssertPermissions(credentialsPath); err != nil {
			ui.Error("credentials permission check failed: %v", err)
			return fmt.Errorf("credentials permission check failed: %w", err)
		}

		// Resolve CA
		ui.StepStart("Resolving Certificate Authority...")
		resolver := ca.NewResolver(credentialsPath)
		caClient, err := resolver.Resolve(serverURLFlag, hmacIDFlag, hmacKeyFlag)
		if err != nil {
			ui.Error("CA resolution failed: %v", err)
			return fmt.Errorf("CA resolution failed: %w", err)
		}
		if serverURLFlag == "" {
			ui.Info("Using Let's Encrypt (ACME v2)")
		} else {
			ui.Info("Using enterprise CA: %s", serverURLFlag)
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
		ui.StepStart("🔐 Validating domains via %s...", strings.ToUpper(vtype))
		validator := validation.NewValidator(vtype, dnsProvider)
		if vtype == "http" && webrootFlag != "" {
			// Pass webroot to validator (if implemented)
			ui.Info("Using webroot: %s", webrootFlag)
		}
		if err := validator.Validate(domains); err != nil {
			ui.Error("validation failed: %v", err)
			return fmt.Errorf("validation failed: %w", err)
		}
		ui.Success("✅ Validation successful for: %s", strings.Join(domains, ", "))

		// Request certificate from CA
		ui.StepStart("📝 Requesting certificate from CA...")
		certMeta, err := caClient.RequestCertificate(domains)
		if err != nil {
			ui.Error("certificate request failed: %v", err)
			return fmt.Errorf("certificate request failed: %w", err)
		}
		ui.Success("📜 Certificate issued by %s", certMeta.Issuer)

		// Save certificate files
		ui.StepStart("💾 Saving certificate files...")
		fullchainPath := fmt.Sprintf("%s/fullchain.pem", certDir)
		if err := os.WriteFile(fullchainPath, certMeta.PEM, 0644); err != nil {
			ui.Error("failed to save certificate: %v", err)
			return err
		}
		ui.Success("Certificate saved: %s", fullchainPath)

		// Install certificate (installer is a stub for now)
		ui.StepStart("🔗 Installing certificate for %s", strings.Join(domains, ", "))
		if err := ca.InstallCertificate(certMeta); err != nil {
			ui.Error("installation failed: %v", err)
			return fmt.Errorf("installation failed: %w", err)
		}
		ui.Success("Certificate installed")

		// Save metadata for renewal
		ui.StepStart("📋 Saving certificate metadata for renewal...")
		meta := &metadata.CertMetadata{
			Domains:          domains,
			ValidationMethod: vtype,
			DNSProvider:      dnsProviderFlag,
			ServerURL:        serverURLFlag,
			HMACIDCred:       hmacIDFlag,
			CredentialsPath:  credentialsPath,
			CertPath:         fullchainPath,
			KeyPath:          keyPath,
			IssuedAt:         time.Now(),
			RenewalAttempts:  0,
		}
		if err := meta.Store(); err != nil {
			ui.Warning("failed to save metadata: %v", err)
		} else {
			ui.Success("Metadata saved for renewal")
		}

		ui.Success("✨ Certificate request complete!")
		ui.Info("Files stored in: %s", certDir)
		ui.Info("Next: Configure your web server to use %s and %s", fullchainPath, keyPath)
		ui.Info("To renew: trustctl renew")

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
	requestCmd.Flags().StringVar(&webrootFlag, "webroot", "/var/www/html", "Webroot for HTTP validation (default /var/www/html)")
	requestCmd.Flags().StringVar(&emailFlag, "email", "", "Email for CA account (default admin@<domain>)")

	rootCmd.AddCommand(requestCmd)

	// Ensure logs directory exists
	if err := os.MkdirAll("/opt/trustctl/logs", 0700); err != nil {
		log.Println("warning: couldn't create logs dir:", err)
	}
}
