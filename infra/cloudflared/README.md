# Cloudflare Tunnel Configuration

## Config Files

- `config.yml` — tunnel routing rules (safe, committed to GitHub)
- `cert.pem` — Cloudflare origin certificate (SENSITIVE, NOT in GitHub)
- `b3105abc-*.json` — tunnel credentials with TunnelSecret (SENSITIVE, NOT in GitHub)

## Restore Procedure

If cloudflared credentials are lost, you must:
1. Go to https://dash.cloudflare.com > Zero Trust > Networks > Tunnels
2. Find the "vibes" tunnel
3. Re-install the connector or extract the tunnel token
4. Place credentials in `~/.cloudflared/`

## Tunnel Routes

| Hostname | Service | Port |
|----------|---------|------|
| vibes.vibestribe.rocks | VibePilot UI | 8090 |
| api.vibestribe.rocks | API gateway | 8642 |
| studio.vibestribe.rocks | Studio | 3002 |
| webhooks.vibestribe.rocks | Governor webhook API | 8080 |
| graphs.vibestribe.rocks | Knowledge Hub (Flask) | 8888 |

## Systemd Service

Service file: `infra/systemd/cloudflared.service`
Install: `cp infra/systemd/cloudflared.service ~/.config/systemd/user/ && systemctl --user daemon-reload && systemctl --user enable --now cloudflared`
