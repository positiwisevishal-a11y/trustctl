package creds

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// AssertPermissions checks that credential files exist and permissions are secure.
func AssertPermissions(dir string) error {
	// Directory must exist
	fi, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("credentials directory %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return errors.New("credentials path is not a directory")
	}

	// Check files in directory have at most 0600 permissions
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		info, err := os.Stat(p)
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode&0o077 != 0 {
			return fmt.Errorf("insecure permissions on %s: %o (expected owner-only)", p, mode)
		}
	}
	return nil
}
