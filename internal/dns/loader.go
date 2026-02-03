package dns

import (
	"fmt"
	"path/filepath"
	"plugin"
	"runtime"
)

// PluginLoader loads DNS provider plugins from a configured plugins directory.
type PluginLoader struct {
	pluginsDir     string
	credentialsDir string
}

func NewPluginLoader(pluginsDir, credentialsDir string) *PluginLoader {
	return &PluginLoader{pluginsDir: pluginsDir, credentialsDir: credentialsDir}
}

// Load loads provider plugin by name (cloudflare -> cloudflare.so)
func (l *PluginLoader) Load(name string) (DNSProvider, error) {
	// Go plugins only supported on linux; return error on unsupported OS
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("go plugin loading only supported on linux: current=%s", runtime.GOOS)
	}

	path := filepath.Join(l.pluginsDir, fmt.Sprintf("%s.so", name))
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open plugin %s: %w", path, err)
	}
	sym, err := p.Lookup("Provider")
	if err != nil {
		return nil, fmt.Errorf("provider symbol not found in %s: %w", path, err)
	}
	prov, ok := sym.(DNSProvider)
	if !ok {
		// Try pointer cast as plugin authors may export *Provider
		if ptr, ok2 := sym.(*DNSProvider); ok2 {
			return *ptr, nil
		}
		return nil, fmt.Errorf("unexpected provider type in %s", path)
	}
	return prov, nil
}
