# trustctl

trustctl is a Go-based certificate automation agent scaffolded to support Let's Encrypt (ACME v2) by default and enterprise CAs via a server URL and HMAC credentials.

Key features implemented in this scaffold:
- CLI with `request` command
- CA resolver: defaults to Let's Encrypt when `--serverurl` is omitted; supports enterprise CA when `--serverurl`, `--hmac-id`, and `--hmac-key` are provided
- DNS plugin loader (Go `plugin`-based) and sample plugin source `plugins_src/cloudflare.go`
- Validation engine scaffolds for `dns` and `http` (DNS uses plugins)
- Installer stubs for `nginx`, `apache`, and `tomcat`
- `scripts/install.sh` to create `/opt/trustctl` layout and permissions

Files of note:
- `cmd/` - CLI commands
- `internal/ca` - CA resolver and client scaffolds
- `internal/dns` - plugin interface and loader
- `internal/validation` - validation flows
- `plugins_src` - sample plugin source to build .so externally

Installation (development):

1. Build

```bash
go build -o trustctl .
```

2. Install (creates directories under `/opt/trustctl`)

```bash
sudo ./scripts/install.sh
```

Plugin packaging:
- Plugins must be built as Go plugins (`-buildmode=plugin`) and installed into `/opt/trustctl/plugins/`.
- Plugin packages (deb/rpm) should only drop files under `/opt/trustctl/plugins`.

Security notes:
- Credentials should live in `/opt/trustctl/credentials/` with `chmod 600` and owned by root.
- Plugins and certs should be `chmod 700` and owned by root.
- CLI avoids printing raw secrets; never pass secrets in logs.

Next steps to reach production-grade:
- Implement full ACME client integration (lego or equivalent) for Let's Encrypt ACME v2.
- Implement enterprise CA HMAC REST client for Sectigo/DigiCert.
- Implement robust DNS plugin loader that verifies plugin signatures/trust.
- Implement atomic certificate writes and rollback, server-specific installers, and a renewal scheduler.
