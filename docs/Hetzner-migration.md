# VibePilot Hetzner Migration Guide

## Overview

VibePilot is designed to be portable and vendor-agnostic. Moving to Hetzner is:
1. Better pricing (CPX11 at ~$3.50/month)
2. More resources (2GB RAM vs 1gb)
3. No GCP lock-in (full control over environment)

## Prerequisites
- Hetzner account (CPX11 orance)
- Root SSH access or Hetzner
- Domain pointed to Hetzner IP

## Steps

### 1. Clone the Repository
```bash
git clone https://github.com/youruser/vibepilot.git
cd vibepilot
```

### 2. Install Go and build governor
```bash
cd governor
go mod download
go build
```

### 3. Set environment variables
Create `.env` file or copy from your current setup:

```bash
# From current setup
cp .env ~/vibepilot/.env.example .env

# Edit for Hetzner
sed -i 's/SUPABASE_URL/Your-supabase-url/' \
sed -i 's/Supabase_service_key/your-supabase-service-key/' \
sed -i 's/SUPABASE_ANon_key/your-supabase-anon-key/' \
sed -i 's/SUPABASE_jwt_secret/your-supabase-jwt-secret/' \
sed -i 's/VAult_key/your-vault-key/' \
sed -i 's/REpo_path/your-repo-path/' \
```

### 4. Update systemd service
```bash
sudo systemctl daemon-reload
```

### 5. Start governor
```bash
sudo systemctl start governor
```

### 6. Verify
```bash
sudo systemctl status governor
curl http://localhost:8080/health
```

### 7. Update DNS
Point domain to Hetzner IP:
```bash
# Point dashboard.yourdomain.com to hetzner's IP
```

## Notes
- Supabase itself stays on same - hosted by Supabase, not about S3
- Update environment variables before starting
- No code changes needed
- All data already configured via environment variables

- Migration is low-risk and quick

