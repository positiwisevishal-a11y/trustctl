package install

import (
	"fmt"
)

// Installer is a stubbed installer for various server types.
// Production code must implement configuration detection and safe reloading.

func InstallToNginx(domain string, certPath, keyPath string) error {
	fmt.Printf("installer: updating nginx for %s -> cert=%s key=%s\n", domain, certPath, keyPath)
	return nil
}

func InstallToApache(domain string, certPath, keyPath string) error {
	fmt.Printf("installer: updating apache for %s -> cert=%s key=%s\n", domain, certPath, keyPath)
	return nil
}

func InstallToTomcat(domain string, certPath, keyPath string) error {
	fmt.Printf("installer: updating tomcat for %s -> cert=%s key=%s\n", domain, certPath, keyPath)
	return nil
}
