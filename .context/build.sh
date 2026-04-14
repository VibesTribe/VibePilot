#!/bin/bash
# .context/build.sh - Generate compressed knowledge layer for VibePilot
# Any agent, any tool, zero dependencies beyond lean-ctx + jcodemunch
# Run: bash .context/build.sh [--full]

set -euo pipefail
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CTX_DIR="$REPO_ROOT/.context"
COMMIT=$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$CTX_DIR"

echo "[.context] Building knowledge layer at $COMMIT..."

# 1. Generate map.md - lean-ctx map mode for all Go source
echo "[.context] Generating map.md (lean-ctx map)..."
{
  echo "# VibePilot Code Map"
  echo "# Generated: $TIMESTAMP | Commit: $COMMIT"
  echo "# Format: lean-ctx map mode (deps + API signatures)"
  echo ""
  
  for f in $(find "$REPO_ROOT/governor" -name "*.go" | sort); do
    rel="${f#$REPO_ROOT/}"
    pkg=$(dirname "$rel")
    echo "## $rel"
    lean-ctx read "$f" -m map 2>/dev/null | grep -v "^\[" | grep -v "tok saved"
    echo ""
  done
} > "$CTX_DIR/map.md"
MAP_SIZE=$(wc -c < "$CTX_DIR/map.md")
echo "[.context] map.md: $MAP_SIZE bytes"

# 2. Generate index.db - copy from jCodeMunch
echo "[.context] Generating index.db (jCodeMunch)..."
MUNCH_DB=$(ls -t ~/.code-index/local-VibePilot-*.db 2>/dev/null | head -1)
if [ -n "$MUNCH_DB" ]; then
  cp "$MUNCH_DB" "$CTX_DIR/index.db"
  echo "[.context] index.db: $(du -sh "$CTX_DIR/index.db" | cut -f1)"
else
  echo "[.context] WARNING: No jCodeMunch index found. Run: jcodemunch-mcp index $REPO_ROOT"
fi

# 3. Generate boot.md - the 2K-token bootstrap file
echo "[.context] Generating boot.md..."
{
  echo "# VibePilot Bootstrap"
  echo "# Generated: $TIMESTAMP | Commit: $COMMIT"
  echo "# Read THIS FIRST. ~2K tokens. Everything else is lazy-loaded from map.md or index.db"
  echo ""
  echo "## What Is VibePilot"
  echo "Sovereign AI execution engine on ThinkPad X220 (i5-2520M, 16GB RAM, no AVX2, no GPU)."
  echo "Transforms PRDs -> production code via multi-agent orchestration."
  echo "Target app: Webs of Wisdom (global multilingual social media platform)."
  echo "Runtime: Single Go binary (governor). Event-driven via Supabase state transitions + realtime."
  echo ""
  echo "## Stack"
  echo "- Language: Go (governor binary)"
  echo "- Database: Supabase (Postgres + RPC + Realtime + Vault)"
  echo "- Config: JSON/YAML in governor/config/"
  echo "- Agent connectors: CLI runners (codex, opencode) + API runners"
  echo "- Webhooks: GitHub + Supabase Realtime for event triggers"
  echo "- Tunnel: Cloudflared at vibestribe.rocks (DO NOT TOUCH)"
  echo "- TTS: edge-tts only"
  echo "- Frontend: VibeDashboard (Supabase + chat panel)"
  echo ""
  echo "## Architecture (packages)"
  echo "- cmd/governor/ - entry point, event handlers, adapters"
  echo "- internal/core/ - state machine (task lifecycle), checkpoint, analyst"
  echo "- internal/runtime/ - session factory, agent pool, context builder, router, tool registry"
  echo "- internal/connectors/ - CLI and API agent runners"
  echo "- internal/dag/ - DAG pipeline engine (YAML-defined workflows)"
  echo "- internal/db/ - Supabase client, RPC calls, state queries"
  echo "- internal/gitree/ - git operations (branch, commit, PR)"
  echo "- internal/vault/ - secrets via Supabase vault"
  echo "- internal/webhooks/ - GitHub webhook server"
  echo "- internal/realtime/ - Supabase Realtime subscription client"
  echo "- internal/mcp/ - external MCP server registry + tool bridge"
  echo "- internal/security/ - secret leak detection"
  echo "- internal/tools/ - tool implementations (db, file, git, vault, web, sandbox)"
  echo "- pkg/types/ - shared type definitions"
  echo ""
  echo "## Constraints"
  echo "- NO local LLM inference (too slow on x220). Cloud free tiers only."
  echo "- NO hardcoded values. Everything in config/ JSON files."
  echo "- NO .env files. Secrets in Supabase vault (get_vault_secret RPC)."
  echo "- RAM is for agent sessions, not model inference."
  echo "- Agent swap velocity: works with Hermes/Claude/Codex/OpenCode/Kimi/Kilo. Must be agent-agnostic."
  echo "- Branch: research-update-april2026"
  echo "- Service: vibepilot-governor (systemd --user)"
  echo "- Logs: journalctl --user -u vibepilot-governor"
  echo ""
  echo "## How To Use This .context/ Directory"
  echo "1. Read boot.md (this file) for orientation (~2K tokens)"
  echo "2. Read map.md for code structure (~12K tokens, all function signatures)"
  echo "3. Query index.db with sqlite3 for targeted searches:"
  echo "   sqlite3 .context/index.db \"SELECT * FROM symbols WHERE name LIKE '%vault%'\""
  echo "4. Read raw source files only when you need implementation details"
  echo ""
  echo "## Event Flow"
  echo "GitHub webhook / Dashboard action -> Supabase DB insert -> Realtime event ->"
  echo "EventRouter -> Handler (task/plan/council/research/maint) -> SessionFactory ->"
  echo "Agent connector (CLI/API) -> Result -> DB update -> Next state"
  echo ""
  echo "## Current Status"
  echo "See CURRENT_STATE.md for full details. Summary:"
  echo "- Governor running as systemd user service"
  echo "- Free model cascade: Google AI Studio -> Groq -> SambaNova -> OpenRouter"
  echo "- .context/ knowledge layer: THIS DIRECTORY (new!)"
} > "$CTX_DIR/boot.md"
BOOT_SIZE=$(wc -c < "$CTX_DIR/boot.md")
echo "[.context] boot.md: $BOOT_SIZE bytes"

# 4. Write meta.yaml
echo "[.context] Writing meta.yaml..."
cat > "$CTX_DIR/meta.yaml" << EOF
# .context metadata - do not edit, auto-generated
commit: $COMMIT
generated: $TIMESTAMP
files:
  boot.md: bootstrap orientation (~2K tokens)
  map.md: compressed code map (~12K tokens)
  index.db: jCodeMunch SQLite search index
  meta.yaml: this file
tools:
  lean_ctx: $(lean-ctx --version 2>/dev/null || echo "not found")
  jcodemunch: installed
EOF

echo ""
echo "[.context] Done! Files:"
ls -lh "$CTX_DIR/"
echo ""
echo "[.context] Total size: $(du -sh "$CTX_DIR/" | cut -f1)"
echo "[.context] Agent bootstrap cost: boot.md only = ~$(wc -w < "$CTX_DIR/boot.md") words"
