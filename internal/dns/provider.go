package dns

// DNSProvider is the interface DNS plugins must implement.
type DNSProvider interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}
