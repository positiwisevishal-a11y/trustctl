package main

import "fmt"

// This file is a skeleton for building a DNS plugin as a Go plugin (.so).
// Build with: `go build -buildmode=plugin -o cloudflare.so cloudflare.go`

type cfProvider struct{}

func (c *cfProvider) Present(domain, token, keyAuth string) error {
	fmt.Printf("[cloudflare plugin] present TXT for %s\n", domain)
	return nil
}

func (c *cfProvider) CleanUp(domain, token, keyAuth string) error {
	fmt.Printf("[cloudflare plugin] cleanup TXT for %s\n", domain)
	return nil
}

// Provider is the exported symbol the loader expects.
var Provider cfProvider
