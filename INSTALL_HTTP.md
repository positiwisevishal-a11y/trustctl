# Running trustctl on Linux Server with HTTP Validation

Complete step-by-step guide to build, install, and run trustctl with HTTP validation for certificate issuance.

## Prerequisites

- Linux server (Ubuntu 20.04+, CentOS 8+, Debian 10+)
- Go 1.20 or higher installed
- Root or sudo access
- Port 80 accessible from internet for the domain(s)
- Web server installed (Nginx or Apache) running on port 80

## Step 1: Clone/Get trustctl Source

```bash
# On your local machine or server
cd /tmp
git clone <your-trustctl-repo> trustctl
cd trustctl
```

Or if already in `/opt/trustctl` development directory, proceed to Step 2.

## Step 2: Build the Binary

```bash
cd /path/to/trustctl
go mod download
go build -o trustctl .
```

Verify the binary was created:
```bash
ls -lah trustctl
```

Expected output: `-rwxr-xr-x 1 user user XXM trustctl`

## Step 3: Create Installation Layout

Install to production location (`/opt/trustctl`):

```bash
sudo ./scripts/install.sh
```

Or manually:
```bash
sudo mkdir -p /opt/trustctl/{bin,plugins,credentials,certs,configs/servers,logs}
sudo install -m 700 ./trustctl /opt/trustctl/bin/trustctl
sudo chown -R root:root /opt/trustctl
sudo chmod 700 /opt/trustctl/plugins /opt/trustctl/certs /opt/trustctl/credentials
```

Verify installation:
```bash
sudo ls -lah /opt/trustctl/
sudo stat -c '%U %G %a' /opt/trustctl/{bin/trustctl,plugins,credentials,certs,logs}
```

## Step 4: Prepare HTTP Validation Path

HTTP validation requires serving ACME challenge files at `/.well-known/acme-challenge/`:

### For Nginx:

Add this to your nginx config (e.g., `/etc/nginx/sites-available/default` or your domain vhost):

```nginx
server {
    listen 80;
    server_name example.com www.example.com;

    # ACME challenge path for certificate validation
    location /.well-known/acme-challenge/ {
        root /var/www/html;
        default_type text/plain;
    }

    # Your other config here...
}
```

Create the challenge directory:
```bash
sudo mkdir -p /var/www/html/.well-known/acme-challenge
sudo chown www-data:www-data /var/www/html/.well-known -R
sudo chmod 755 /var/www/html/.well-known
```

Reload nginx:
```bash
sudo nginx -t
sudo systemctl reload nginx
```

### For Apache:

Create the challenge directory:
```bash
sudo mkdir -p /var/www/html/.well-known/acme-challenge
sudo chown www-data:www-data /var/www/html/.well-known -R
sudo chmod 755 /var/www/html/.well-known
```

Apache typically serves all files under `DocumentRoot` by default, so no additional config needed.

Reload apache:
```bash
sudo systemctl reload apache2
```

Test the path is accessible:
```bash
curl http://example.com/.well-known/acme-challenge/test
# Should return 404 or permission denied (file doesn't exist yet, which is OK)
```

## Step 5: Create Credentials Directory (Optional for Let's Encrypt)

For **Let's Encrypt** (default), no credentials needed. But ensure the directory exists with proper permissions:

```bash
sudo mkdir -p /opt/trustctl/credentials
sudo chmod 700 /opt/trustctl/credentials
sudo chown root:root /opt/trustctl/credentials
```

For **enterprise CA** (Sectigo/DigiCert), create credential file:

```bash
sudo cat > /opt/trustctl/credentials/sectigo.yaml << 'EOF'
# Sectigo/DigiCert API credentials
hmac_id: YOUR_HMAC_ID
hmac_key: YOUR_HMAC_SECRET
api_endpoint: https://api.sectigo.com/v1/certificates
EOF

sudo chmod 600 /opt/trustctl/credentials/sectigo.yaml
sudo chown root:root /opt/trustctl/credentials/sectigo.yaml
```

## Step 6: Run Request with HTTP Validation

### Simple Request (Single Domain - Let's Encrypt Default)

```bash
sudo /opt/trustctl/bin/trustctl request \
  --domains example.com \
  --validation http
```

Expected output:
```
ℹ️  Checking credential permissions in /opt/trustctl/credentials...
🔄 Resolving Certificate Authority...
ℹ️  Using Let's Encrypt (ACME v2) as default CA
✔️  CA resolved
🔄 Starting HTTP validation for example.com
✅ Validation successful for: example.com
🔄 Requesting certificate from CA...
✅ Certificate issued by Let's Encrypt
🔄 Installing certificate for example.com
✅ Certificate installed for: example.com
🔄 Saving certificate metadata for renewal...
✅ Metadata saved for renewal
```

### Multi-Domain (SAN - Subject Alternative Names)

```bash
sudo /opt/trustctl/bin/trustctl request \
  --domains "example.com,www.example.com,api.example.com" \
  --validation http
```

### With Enterprise CA (Sectigo - HMAC Auth)

```bash
sudo /opt/trustctl/bin/trustctl request \
  --domains example.com \
  --validation http \
  --serverurl https://api.sectigo.com/v1/certificates \
  --hmac-id YOUR_HMAC_ID \
  --hmac-key YOUR_HMAC_SECRET
```

**Important:** Keep HMAC key out of shell history. Instead, use credential file and pass only the file path:
```bash
# Store in secure credential file instead
sudo cat > /opt/trustctl/credentials/sectigo.yaml << 'EOF'
hmac_id: YOUR_HMAC_ID
hmac_key: YOUR_HMAC_SECRET
EOF
sudo chmod 600 /opt/trustctl/credentials/sectigo.yaml
```

## Step 7: What Happens During HTTP Validation

1. **Challenge Generation**: ACME server generates a challenge token for each domain.
2. **File Placement**: trustctl writes token file to `/var/www/html/.well-known/acme-challenge/`.
3. **ACME Verification**: ACME server makes HTTP GET request to `http://example.com/.well-known/acme-challenge/<token>`.
4. **Validation Success**: If token matches, domain is validated.
5. **Certificate Issuance**: CA issues certificate for validated domain(s).
6. **Installation**: Certificate is stored in `/opt/trustctl/certs/example.com/`.
7. **Metadata Save**: Configuration saved for automatic renewal.

## Step 8: Verify Certificate Installation

```bash
sudo ls -lah /opt/trustctl/certs/example.com/
```

Expected files:
- `fullchain.pem` - Certificate chain
- `privkey.pem` - Private key
- `metadata.json` - Renewal configuration

View metadata:
```bash
sudo cat /opt/trustctl/certs/example.com/metadata.json
```

Expected metadata:
```json
{
  "domains": ["example.com"],
  "validation_method": "http",
  "server_url": "",
  "credentials_path": "/opt/trustctl/credentials",
  "cert_path": "/opt/trustctl/certs/example.com/fullchain.pem",
  "key_path": "/opt/trustctl/certs/example.com/privkey.pem",
  "issued_at": "2026-02-03T12:34:56Z",
  "renewal_attempts": 0
}
```

## Step 9: Setup Automatic Renewal (Optional)

Create cron job for daily renewal checks at 3 AM:

```bash
sudo cat > /etc/cron.d/trustctl-renew << 'EOF'
# Renew certificates daily at 03:00 AM
0 3 * * * root /opt/trustctl/bin/trustctl renew >> /opt/trustctl/logs/trustctl.log 2>&1
EOF

sudo chmod 644 /etc/cron.d/trustctl-renew
```

Verify cron is set:
```bash
sudo cat /etc/cron.d/trustctl-renew
```

View renewal logs:
```bash
sudo tail -f /opt/trustctl/logs/trustctl.log
```

## Step 10: Configure Web Server to Use Certificate (Nginx)

After certificate is issued, update vhost to use HTTPS (443):

```bash
sudo cat > /etc/nginx/sites-available/example.com << 'EOF'
# HTTP to HTTPS redirect
server {
    listen 80;
    server_name example.com www.example.com;
    return 301 https://$server_name$request_uri;
}

# HTTPS with trustctl certificate
server {
    listen 443 ssl;
    server_name example.com www.example.com;

    ssl_certificate /opt/trustctl/certs/example.com/fullchain.pem;
    ssl_certificate_key /opt/trustctl/certs/example.com/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://localhost:8080;  # or your app
        proxy_set_header Host $host;
    }
}
EOF

sudo nginx -t
sudo systemctl reload nginx
```

Test HTTPS:
```bash
curl -I https://example.com/
```

## Troubleshooting

### 1. Challenge File Not Accessible

```bash
# Test if challenge path is served
curl -v http://example.com/.well-known/acme-challenge/test
# Should get 404 or 403, NOT connection refused

# Check directory permissions
sudo ls -lah /var/www/html/.well-known/acme-challenge/
# Should be: drwxr-xr-x www-data:www-data
```

### 2. Validation Failed Error

```
❌ validation failed: ...
```

**Causes:**
- Port 80 not accessible from internet (firewall/ISP blocking)
- DNS not pointing to server
- Web server not running

**Fix:**
```bash
# Check DNS resolution
nslookup example.com
dig example.com

# Check port 80 is listening
sudo netstat -tlnp | grep :80

# Check web server running
sudo systemctl status nginx   # or apache2
sudo systemctl restart nginx
```

### 3. Credential Permission Check Failed

```
❌ credentials permission check failed: ...
```

**Fix:**
```bash
sudo chmod 600 /opt/trustctl/credentials/*
sudo chown root:root /opt/trustctl/credentials/*
sudo ls -lah /opt/trustctl/credentials/
```

### 4. View Full Logs

```bash
sudo tail -100 /opt/trustctl/logs/trustctl.log
```

## HTTP Validation Limitations

❌ **Cannot use for wildcard domains** (`*.example.com`)
- Wildcard validation requires DNS challenge (`--validation dns`)

✅ **Can use for:**
- Single domain: `example.com`
- Multiple domains (SAN): `example.com,www.example.com,api.example.com`

If you need wildcard, switch to DNS validation:
```bash
sudo /opt/trustctl/bin/trustctl request \
  --domains "*.example.com,example.com" \
  --validation dns \
  --dns-provider cloudflare
```

## Quick Reference

| Task | Command |
|------|---------|
| Request HTTP cert | `sudo /opt/trustctl/bin/trustctl request --domains example.com --validation http` |
| Renew certificates | `sudo /opt/trustctl/bin/trustctl renew` |
| Check cert status | `sudo cat /opt/trustctl/certs/example.com/metadata.json` |
| View logs | `sudo tail -f /opt/trustctl/logs/trustctl.log` |
| Verify cert | `openssl x509 -in /opt/trustctl/certs/example.com/fullchain.pem -text -noout` |
| List all certs | `sudo ls -d /opt/trustctl/certs/*/` |

