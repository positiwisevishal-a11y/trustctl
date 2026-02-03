package install

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/trustctl/trustctl/internal/ui"
)

// Installer performs simple, safe edits to Apache/Nginx vhost files:
// - Detects vhost(s) serving port 80 for each domain
// - Shows which vhost file(s) will be used
// - If a 443 vhost exists for the same domain, replaces the SSL cert paths
// - Otherwise creates a new 443 vhost block per domain in the same file
// Files are backed up and written atomically. This is a practical, text-based approach
// and should be reviewed before use in production.

var (
	nginxSitesDirs  = []string{"/etc/nginx/sites-enabled", "/etc/nginx/sites-available", "/etc/nginx/conf.d"}
	apacheSitesDirs = []string{"/etc/apache2/sites-enabled", "/etc/apache2/sites-available", "/etc/httpd/conf.d"}
)

// InstallForDomains installs/updates certificates for the provided domains.
func InstallForDomains(domains []string, certPath, keyPath string) error {
	if len(domains) == 0 {
		return errors.New("no domains provided")
	}
	// Prefer detecting a running server
	srv, _ := detectRunningServer()
	if srv == "nginx" {
		for _, d := range domains {
			if err := installNginxForDomain(d, certPath, keyPath); err != nil {
				return err
			}
		}
		ui.Success("Detected running nginx. Updated config files; reload with: sudo systemctl reload nginx")
		return nil
	}
	if srv == "apache" {
		for _, d := range domains {
			if err := installApacheForDomain(d, certPath, keyPath); err != nil {
				return err
			}
		}
		ui.Success("Detected running apache. Updated config files; reload with: sudo systemctl reload apache2")
		return nil
	}

	// Fallback to config directories
	if hasAnyDir(nginxSitesDirs) {
		for _, d := range domains {
			if err := installNginxForDomain(d, certPath, keyPath); err != nil {
				return err
			}
		}
		ui.Success("No running server detected; updated nginx configs. Reload: sudo systemctl reload nginx")
		return nil
	}
	if hasAnyDir(apacheSitesDirs) {
		for _, d := range domains {
			if err := installApacheForDomain(d, certPath, keyPath); err != nil {
				return err
			}
		}
		ui.Success("No running server detected; updated apache configs. Reload: sudo systemctl reload apache2")
		return nil
	}

	return errors.New("no supported web server configuration directories found (nginx/apache)")
}

// detectRunningServer tries to detect which webserver is currently running.
// It prefers `systemctl` checks and falls back to scanning process list.
func detectRunningServer() (string, error) {
	// Check via systemctl if available
	if _, err := exec.LookPath("systemctl"); err == nil {
		// check nginx
		if err := exec.Command("systemctl", "is-active", "--quiet", "nginx").Run(); err == nil {
			return "nginx", nil
		}
		// check apache variants
		if err := exec.Command("systemctl", "is-active", "--quiet", "apache2").Run(); err == nil {
			return "apache", nil
		}
		if err := exec.Command("systemctl", "is-active", "--quiet", "httpd").Run(); err == nil {
			return "apache", nil
		}
	}

	// Fallback: scan process list
	out, err := exec.Command("ps", "ax").Output()
	if err == nil {
		s := string(out)
		if strings.Contains(s, "nginx: master") || strings.Contains(s, "nginx") {
			return "nginx", nil
		}
		if strings.Contains(s, "apache2") || strings.Contains(s, "httpd") {
			return "apache", nil
		}
	}
	return "", errors.New("no running web server detected")
}

func hasAnyDir(paths []string) bool {
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

// installNginxForDomain finds the 80 vhost file containing the domain and creates/updates 443 vhost.
func installNginxForDomain(domain, certPath, keyPath string) error {
	files := collectFiles(nginxSitesDirs)
	matched := false
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s := string(content)
		if strings.Contains(s, "listen 80") && strings.Contains(s, domain) {
			matched = true
			fmt.Printf("Found HTTP vhost in %s for %s\n", f, domain)
			if strings.Contains(s, "listen 443") && strings.Contains(s, domain) {
				// Update existing ssl_certificate lines
				new := updateNginxSSL(s, certPath, keyPath, domain)
				if new == s {
					fmt.Printf("No change required for 443 vhost in %s\n", f)
				} else {
					if err := backupAndWriteFile(f, []byte(new)); err != nil {
						return err
					}
					fmt.Printf("Updated 443 vhost SSL paths in %s\n", f)
				}
			} else {
				// Create new 443 server block for this domain
				serverName := extractNginxServerName(s, domain)
				block := buildNginx443Block(serverName, certPath, keyPath)
				new := s + "\n\n" + block + "\n"
				if err := backupAndWriteFile(f, []byte(new)); err != nil {
					return err
				}
				fmt.Printf("Appended new 443 vhost for %s into %s\n", domain, f)
			}
		}
	}
	if !matched {
		fmt.Printf("No nginx HTTP vhost found for %s; skipping\n", domain)
	}
	return nil
}

func updateNginxSSL(content, certPath, keyPath, domain string) string {
	// Replace ssl_certificate and ssl_certificate_key for blocks containing domain
	reCert := regexp.MustCompile(`(?m)^\s*ssl_certificate\s+\S+;`)
	reKey := regexp.MustCompile(`(?m)^\s*ssl_certificate_key\s+\S+;`)
	new := reCert.ReplaceAllString(content, fmt.Sprintf("    ssl_certificate %s;", certPath))
	new = reKey.ReplaceAllString(new, fmt.Sprintf("    ssl_certificate_key %s;", keyPath))
	return new
}

func extractNginxServerName(content, domain string) string {
	// Try to extract server_name line containing the domain; fall back to domain
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "server_name") && strings.Contains(line, domain) {
			// return the value portion
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return strings.Join(parts[1:], " ")
			}
		}
	}
	return domain
}

func buildNginx443Block(serverName, certPath, keyPath string) string {
	return fmt.Sprintf(`server {
	listen 443 ssl;
	server_name %s;
	ssl_certificate %s;
	ssl_certificate_key %s;
	# proxy/serve static content as appropriate
}
`, serverName, certPath, keyPath)
}

// installApacheForDomain performs similar operations for Apache vhost files.
func installApacheForDomain(domain, certPath, keyPath string) error {
	files := collectFiles(apacheSitesDirs)
	matched := false
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s := string(content)
		if (strings.Contains(s, "<VirtualHost") && strings.Contains(s, ":80")) && (strings.Contains(s, "ServerName "+domain) || strings.Contains(s, "ServerAlias "+domain)) {
			matched = true
			fmt.Printf("Found HTTP vhost in %s for %s\n", f, domain)
			if strings.Contains(s, ":443") {
				new := updateApacheSSL(s, certPath, keyPath, domain)
				if new == s {
					fmt.Printf("No change required for 443 vhost in %s\n", f)
				} else {
					if err := backupAndWriteFile(f, []byte(new)); err != nil {
						return err
					}
					fmt.Printf("Updated 443 vhost SSL paths in %s\n", f)
				}
			} else {
				// Append new 443 VirtualHost
				serverName := extractApacheServerName(s, domain)
				block := buildApache443Block(serverName, certPath, keyPath)
				new := s + "\n\n" + block + "\n"
				if err := backupAndWriteFile(f, []byte(new)); err != nil {
					return err
				}
				fmt.Printf("Appended new 443 vhost for %s into %s\n", domain, f)
			}
		}
	}
	if !matched {
		fmt.Printf("No apache HTTP vhost found for %s; skipping\n", domain)
	}
	return nil
}

func updateApacheSSL(content, certPath, keyPath, domain string) string {
	// Replace SSLCertificateFile and SSLCertificateKeyFile occurrences
	reCert := regexp.MustCompile(`(?m)^\s*SSLCertificateFile\s+\S+`)
	reKey := regexp.MustCompile(`(?m)^\s*SSLCertificateKeyFile\s+\S+`)
	new := reCert.ReplaceAllString(content, fmt.Sprintf("    SSLCertificateFile %s", certPath))
	new = reKey.ReplaceAllString(new, fmt.Sprintf("    SSLCertificateKeyFile %s", keyPath))
	return new
}

func extractApacheServerName(content, domain string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "ServerName ") && strings.Contains(line, domain) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return domain
}

func buildApache443Block(serverName, certPath, keyPath string) string {
	return fmt.Sprintf(`<VirtualHost *:443>
	ServerName %s
	SSLEngine on
	SSLCertificateFile %s
	SSLCertificateKeyFile %s
	# DocumentRoot /var/www/html
</VirtualHost>
`, serverName, certPath, keyPath)
}

func collectFiles(dirs []string) []string {
	var out []string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			out = append(out, filepath.Join(d, e.Name()))
		}
	}
	return out
}

func backupAndWriteFile(path string, data []byte) error {
	// create backup
	bak := fmt.Sprintf("%s.bak.%d", path, time.Now().Unix())
	if err := copyFile(path, bak); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	// write to temp and rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
