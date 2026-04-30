# VibePilot Hetzner Migration Guide

## Overview
VibePilot is designed to be portable and vendor-agnostic. Moving to Hetzner is:
1. Better pricing (CPX11 at ~$3.50/month)
2. More resources (2GB RAM vs 1GB)
3. No GCP lock-in (full control over environment)

## Prerequisites
- Hetzner account (CPX11 or cax)
- Domain pointed to Hetzner IP
- Root SSH access to Hetzner

## Steps

### 1. Create Hetzner server
- Type: CPX11
- Location: Choose closest region
- Image: Ubuntu 24.04 or Debian 12
- SSH keys: Add your existing key

### 2. Clone Repository
```bash
git clone https://github.com/youruser/vibepilot.git
cd vibepilot
```

### 3. Install Go
```bash
# Ubuntu
sudo apt update install golang-go
# Debian
sudo apt install golang
```

Or install manually: https://go.dev/doc/install

### 4. Set Environment Variables
Create `.env` file:

```bash
cp .env.example .env
```

Edit `.env`:
```
SUPABASE_URL=your-supabase-url
SUPABASE_SERVICE_KEY=your-service-key
SUPABASE_ANON_KEY=your-anon-key
VAULT_KEY=your-vault-key
```

### 5. Build and Deploy Governor
```bash
cd governor
go mod download
go build

# Copy systemd service
sudo cp scripts/governor.service /etc/systemd/system/

# Reload and start
sudo systemctl daemon-reload
sudo systemctl start governor
```

### 6. Verify
```bash
sudo systemctl status governor
```

### 7. Update DNS
Point dashboard domain to Hetzner IP:
```bash
# In your DNS provider, create A record:
dashboard.yourdomain.com  A   <hetzner-ip>
```

## Notes
- Supabase itself stays on same - hosted by Supabase, not about s3)
- Update environment variables before starting
- No code changes needed
- All data already configured via environment variables
- Migration is low-risk and quick
