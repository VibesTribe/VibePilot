#!/usr/bin/env python3
"""
PIF Scaffold System — Phase A
=============================
Creates a fully isolated project per the Project Isolation Framework spec.
Every project gets: directory structure, vibepilot.toml, export.sh, restore.sh,
README.md, Hermes profile, git repo, backup repo, SQLite database.

Usage:
  python3 pif_scaffold.py --slug sealed
  python3 pif_scaffold.py --slug sealed --display-name "Sealed" --description "..."
  python3 pif_scaffold.py --slug sealed --skip-github  # local only, no GitHub repos

Called from the governor (handleProjectCreate) as:
  python3 pif_scaffold.py --slug sealed --json  # outputs JSON result for the API

Spec: vibepilot/docs/PROJECT_ISOLATION_FRAMEWORK.md
All decisions are binding (see RESOLVED DECISIONS at end of spec).
"""

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tomllib  # Python 3.11+
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

PROJECTS_BASE = Path.home() / "projects"
HERMES_HOME = Path.home() / ".hermes"
VIBEPILOT_HOME = Path.home() / "vibepilot"
GITHUB_OWNER = "VibesTribe"

# Directory structure from spec section 1
PROJECT_DIRS = [
    "repo",
    "database",
    "skills",
    "memories",
    "knowledgebase",
    "research",
    "config",
    "backups",
    "logs",
]

# Subdirectories for memories (namespaced per spec)
MEMORY_NAMESPACES = ["planner", "architect", "engineer", "reviewer"]


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def run(cmd: str, check: bool = True, capture: bool = True, cwd: Optional[str] = None) -> subprocess.CompletedProcess:
    """Run a shell command, return CompletedProcess."""
    return subprocess.run(
        cmd, shell=True, check=check, capture_output=capture, text=True, cwd=cwd
    )


def get_github_token() -> Optional[str]:
    """Extract GitHub PAT from ~/.git-credentials."""
    cred_file = Path.home() / ".git-credentials"
    if not cred_file.exists():
        return None
    content = cred_file.read_text()
    # Format: https://VibesTribe:TOKEN@github.com
    match = re.search(r'VibesTribe:([^@]+)@github\.com', content)
    if match:
        return match.group(1)
    # Try generic format
    match = re.search(r':([^@]+)@github\.com', content)
    if match:
        return match.group(1)
    return None


def github_api(method: str, endpoint: str, data: Optional[dict] = None) -> dict:
    """Call GitHub API with PAT auth."""
    token = get_github_token()
    if not token:
        raise RuntimeError("No GitHub PAT found in ~/.git-credentials")
    
    cmd = f'curl -s -X {method} -H "Authorization: token {token}" -H "Accept: application/vnd.github+json"'
    if data:
        json_str = json.dumps(data).replace("'", "'\\''")
        cmd += f" -d '{json_str}'"
    cmd += f' https://api.github.com{endpoint}'
    
    result = run(cmd, check=False)
    return json.loads(result.stdout) if result.stdout.strip() else {}


def slug_valid(slug: str) -> bool:
    """Validate slug: lowercase, alphanumeric, hyphens only."""
    return bool(re.match(r'^[a-z][a-z0-9-]*$', slug))


def slug_exists(slug: str) -> bool:
    """Check if project directory already exists."""
    return (PROJECTS_BASE / slug).exists()


# ---------------------------------------------------------------------------
# Step 1: Create directory structure
# ---------------------------------------------------------------------------

def create_directories(project_dir: Path, slug: str) -> list[str]:
    """Create the full PIF directory structure."""
    created = []
    for d in PROJECT_DIRS:
        path = project_dir / d
        path.mkdir(parents=True, exist_ok=True)
        created.append(str(path))
    
    # Create memory namespaces
    for ns in MEMORY_NAMESPACES:
        path = project_dir / "memories" / ns
        path.mkdir(parents=True, exist_ok=True)
    
    # Create .gitkeep files so empty dirs survive git
    for d in PROJECT_DIRS:
        keepfile = project_dir / d / ".gitkeep"
        if not keepfile.exists():
            keepfile.touch()
    
    return created


# ---------------------------------------------------------------------------
# Step 2: Generate vibepilot.toml
# ---------------------------------------------------------------------------

def generate_manifest(project_dir: Path, slug: str, display_name: str,
                      description: str, deploy_target: str = "cloudflare",
                      deploy_url: str = "", github_owner: str = GITHUB_OWNER) -> Path:
    """Generate the vibepilot.toml manifest from template."""
    
    if not deploy_url:
        deploy_url = f"https://{slug}.icu"
    
    backend_url = f"https://api.{slug}.icu"
    
    toml_content = f'''# vibepilot.toml — Project Manifest
# Auto-generated by PIF Scaffold System on {datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")}
# Spec: vibepilot/docs/PROJECT_ISOLATION_FRAMEWORK.md
# This file is the contract between VibePilot and the project. Do not rename sections.

[manifest]
version = 1
framework_min = "1.0"
framework_max = "2.x"

[project]
slug = "{slug}"
display_name = "{display_name}"
description = "{description}"
status = "active"

[repo]
remote = "git@github.com:{github_owner}/{slug}.git"
local_path = "repo/"
default_branch = "main"
protected_branches = ["main"]

[agent]
runtime = "hermes"          # "hermes" | "claude-code" | "opencode" | "kilo" | "custom"
profile = "{slug}"          # Hermes profile name
working_dir = "repo/"

[execution]
cost_limit_usd = 5.00
retry_policy = "conservative"  # "conservative" | "aggressive" | "none"
approval_required = true

[database]
type = "sqlite"             # "sqlite" | "postgres" | "supabase" | "none"
edge_path = "database/{slug}.db"
cloud_provider = "supabase" # "supabase" | "neon" | "none"
cloud_url_ref = "{slug.upper()}_SUPABASE_URL"

[deploy]
target = "{deploy_target}"  # "cloudflare" | "vercel" | "docker" | "none"
frontend_url = "{deploy_url}"
backend_url = "{backend_url}"
edge_node = "x220"

[model_keys]
keys = []                   # e.g. ["{slug.upper()}_OPENAI_KEY", "{slug.upper()}_STRIPE_KEY"]

[network]
egress_allow = []           # e.g. ["api.stripe.com:443", "github.com:443"]

[isolation]
database_separate = true
kb_separate = true
research_separate = true
skills_separate = true
memories_separate = true

[backup]
enabled = true
destination = "github"      # "github" | "s3" | "local"
repo = "{github_owner}/{slug}-backup"
schedule = "0 3 * * *"
'''
    
    manifest_path = project_dir / "vibepilot.toml"
    manifest_path.write_text(toml_content)
    return manifest_path


# ---------------------------------------------------------------------------
# Step 3: Generate export.sh (with secret scrub + signing)
# ---------------------------------------------------------------------------

def generate_export_sh(project_dir: Path, slug: str) -> Path:
    """Generate export.sh — packages project for transfer."""
    # NOTE: Using regular string (not f-string) because shell scripts contain
    # many {} characters that conflict with Python f-string syntax.
    export_script = '''#!/bin/bash
# export.sh — Package __SLUG__ project for transfer/exit
# Generated by PIF Scaffold System
# Usage: ./export.sh [--include-db] [--include-secrets]
#
# Guardrail 5: Every project must have export.sh
# Guardrail 9: Secrets are scrubbed on export (unless --include-secrets)
# Guardrail 11: Export archives are signed

set -euo pipefail

SLUG="__SLUG__"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARCHIVE_NAME="${SLUG}-export-$(date +%Y%m%d-%H%M%S).tar.gz"

INCLUDE_DB=false
INCLUDE_SECRETS=false

for arg in "$@"; do
    case $arg in
        --include-db) INCLUDE_DB=true ;;
        --include-secrets) INCLUDE_SECRETS=true ;;
        --help|-h)
            echo "Usage: ./export.sh [--include-db] [--include-secrets]"
            echo ""
            echo "  --include-db       Include database dumps"
            echo "  --include-secrets  Include decrypted secrets (DANGEROUS)"
            exit 0
            ;;
    esac
done

echo "==> Packaging project: $SLUG"

EXCLUDES=(
    --exclude=./backups
    --exclude=./logs
    --exclude=./.git
    --exclude=./repo/.git
    --exclude=*.pyc
    --exclude=__pycache__
    --exclude=.DS_Store
    --exclude=node_modules
    --exclude=.env
    --exclude=*.db
)

if [ "$INCLUDE_DB" = true ]; then
    EXCLUDES=("${EXCLUDES[@]/--exclude=*.db}")
    echo "==> Including database files"
fi

if [ "$INCLUDE_SECRETS" = false ]; then
    echo "==> Scrubbing secrets..."
    SCRUB_DIR=$(mktemp -d)
    cp -r "$PROJECT_DIR"/* "$SCRUB_DIR/" 2>/dev/null || true
    find "$SCRUB_DIR" -type f \\( \\
        -name '*.py' -o -name '*.go' -o -name '*.js' -o -name '*.ts' \\
        -o -name '*.json' -o -name '*.yaml' -o -name '*.yml' -o -name '*.toml' \\
        -o -name '*.env' -o -name '*.sh' -o -name '*.md' -o -name '*.txt' \\) -exec sed -i \\
        -e 's/sk_live_[0-9a-zA-Z]*/REDACTED_STRIPE_KEY/g' \\
        -e 's/sk_test_[0-9a-zA-Z]*/REDACTED_STRIPE_KEY/g' \\
        -e 's/ghp_[0-9a-zA-Z]*/REDACTED_GITHUB_TOKEN/g' \\
        -e 's/AIza[0-9A-Za-z_-]*/REDACTED_GOOGLE_KEY/g' \\
        -e 's/sk-[0-9a-zA-Z]*/REDACTED_OPENAI_KEY/g' \\
        {} \\;
    TARBALL_DIR="$SCRUB_DIR"
else
    echo "==> WARNING: Including secrets in archive!"
    TARBALL_DIR="$PROJECT_DIR"
fi

ARCHIVE_PATH="$PROJECT_DIR/$ARCHIVE_NAME"
echo "==> Creating archive: $ARCHIVE_NAME"
tar czf "$ARCHIVE_PATH" -C "$TARBALL_DIR" "${EXCLUDES[@]}" . 2>/dev/null || \\
    tar czf "$ARCHIVE_PATH" -C "$TARBALL_DIR" . 2>/dev/null

echo "==> Signing archive..."
SHA256=$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')
echo "$SHA256" > "$ARCHIVE_PATH.sha256"
echo "    SHA256: $SHA256"

if [ "$INCLUDE_SECRETS" = false ]; then
    rm -rf "$SCRUB_DIR"
fi

echo ""
echo "==> Export complete: $ARCHIVE_PATH"
echo "    Signature: $ARCHIVE_PATH.sha256"
echo "    To restore: ./restore.sh $ARCHIVE_NAME"
'''.replace("__SLUG__", slug)
    
    export_path = project_dir / "export.sh"
    export_path.write_text(export_script)
    export_path.chmod(0o755)
    return export_path


# ---------------------------------------------------------------------------
# Step 4: Generate restore.sh (with signature verify)
# ---------------------------------------------------------------------------

def generate_restore_sh(project_dir: Path, slug: str) -> Path:
    """Generate restore.sh — unpacks project on a clean machine."""
    # NOTE: Regular string, not f-string, to avoid shell {} conflicts.
    restore_script = '''#!/bin/bash
# restore.sh — Restore __SLUG__ project from an export archive
# Generated by PIF Scaffold System
# Usage: ./restore.sh <archive.tar.gz>
#
# Guardrail 5: Every project must have restore.sh
# Guardrail 11: Verifies signature before extracting

set -euo pipefail

SLUG="__SLUG__"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"

if [ -z "$1" ]; then
    echo "Usage: ./restore.sh <archive.tar.gz>"
    echo ""
    echo "  Restores the project from an export archive."
    echo "  Verifies SHA256 signature before extraction."
    exit 1
fi

ARCHIVE="$1"
ARCHIVE_PATH="$PROJECT_DIR/$ARCHIVE"

if [ ! -f "$ARCHIVE_PATH" ]; then
    if [ -f "$ARCHIVE" ]; then
        ARCHIVE_PATH="$ARCHIVE"
    else
        echo "ERROR: Archive not found: $ARCHIVE"
        exit 1
    fi
fi

SIG_FILE="${ARCHIVE_PATH}.sha256"

if [ -f "$SIG_FILE" ]; then
    echo "==> Verifying archive signature..."
    EXPECTED=$(cat "$SIG_FILE" | awk '{print $1}')
    ACTUAL=$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')
    if [ "$EXPECTED" != "$ACTUAL" ]; then
        echo "ERROR: Signature verification FAILED!"
        echo "  Expected: $EXPECTED"
        echo "  Actual:   $ACTUAL"
        echo "  The archive may be corrupted or tampered with."
        exit 1
    fi
    echo "    Signature verified: $ACTUAL"
else
    echo "WARNING: No signature file found. Proceeding without verification."
fi

echo "==> Extracting archive..."
tar xzf "$ARCHIVE_PATH" -C "$PROJECT_DIR"

echo ""
echo "==> Restore complete."
echo "    Project restored to: $PROJECT_DIR"
echo "    Next steps:"
echo "      1. Review config/ for any placeholder secrets"
echo "      2. Initialize database if needed"
echo "      3. Install dependencies in repo/"
echo "      4. Run the project"
'''.replace("__SLUG__", slug)
    
    restore_path = project_dir / "restore.sh"
    restore_path.write_text(restore_script)
    restore_path.chmod(0o755)
    return restore_path


# ---------------------------------------------------------------------------
# Step 5: Generate README.md
# ---------------------------------------------------------------------------

def generate_readme(project_dir: Path, slug: str, display_name: str,
                    description: str) -> Path:
    """Generate project README."""
    
    readme = f'''# {display_name}

{description}

## Project Structure

This project follows the VibePilot Project Isolation Framework (PIF).

```
{slug}/
  vibepilot.toml          # Project manifest
  repo/                   # Application codebase
  database/               # Database files, migrations
  skills/                 # Agent skills (project-specific)
  memories/               # Agent memories (namespaced)
  knowledgebase/          # Stable project knowledge
  research/               # Research findings (temporary)
  config/                 # Configuration, model keys, deploy config
  backups/                # Automated snapshots
  logs/                   # Operational logs, audit trail
  export.sh               # Package for transfer
  restore.sh              # Restore from archive
```

## Manifest

See `vibepilot.toml` for the full project configuration including:
- Agent runtime and profile
- Database configuration
- Deploy target
- Network egress allowlist
- Isolation settings
- Backup configuration

## Export / Transfer

```bash
./export.sh                    # Package without secrets or DB
./export.sh --include-db       # Include database dumps
./export.sh --include-secrets  # Include decrypted secrets (full transfer)
```

## Restore

```bash
./restore.sh <archive-name>.tar.gz
```

---
Generated by PIF Scaffold System on {datetime.now(timezone.utc).strftime("%Y-%m-%d")}
'''
    
    readme_path = project_dir / "README.md"
    readme_path.write_text(readme)
    return readme_path


# ---------------------------------------------------------------------------
# Step 6: Initialize Hermes profile
# ---------------------------------------------------------------------------

def create_hermes_profile(slug: str) -> Path:
    """Create Hermes profile directory for the project."""
    profile_dir = HERMES_HOME / "profiles" / slug
    
    # Create profile subdirectories matching default profile structure
    (profile_dir / "skills").mkdir(parents=True, exist_ok=True)
    (profile_dir / "memories").mkdir(parents=True, exist_ok=True)
    (profile_dir / "cron").mkdir(parents=True, exist_ok=True)
    
    # Create a profile marker file with metadata
    profile_meta = {
        "slug": slug,
        "created_at": datetime.now(timezone.utc).isoformat(),
        "created_by": "pif_scaffold",
        "description": f"Isolated profile for project: {slug}",
    }
    (profile_dir / ".profile.json").write_text(json.dumps(profile_meta, indent=2))
    
    return profile_dir


# ---------------------------------------------------------------------------
# Step 7: Initialize git repo + GitHub remote
# ---------------------------------------------------------------------------

def init_git_repo(project_dir: Path, slug: str, skip_github: bool = False) -> dict:
    """Initialize git repo, create GitHub remote, push initial commit."""
    result = {"local_repo": False, "github_repo": False, "github_url": "", "error": ""}
    
    repo_dir = project_dir / "repo"
    
    # Initialize local git repo
    try:
        if not (repo_dir / ".git").exists():
            run(f"git init", cwd=str(repo_dir))
            run(f"git checkout -b main", cwd=str(repo_dir), check=False)
        
        # Create initial commit if repo is empty
        commit_check = run("git log --oneline -1 2>/dev/null", cwd=str(repo_dir), check=False)
        if commit_check.returncode != 0:
            # Create a minimal initial file
            (repo_dir / ".gitignore").write_text(
                "# Dependencies\nnode_modules/\nvendor/\n\n"
                "# Build output\ndist/\nbuild/\n*.exe\n\n"
                "# Environment\n.env\n.env.*\n\n"
                "# OS files\n.DS_Store\nThumbs.db\n\n"
                "# IDE\n.vscode/\n.idea/\n*.swp\n"
            )
            run("git add .gitignore", cwd=str(repo_dir))
            run(f'git commit -m "Initial commit — PIF scaffold for {slug}"', cwd=str(repo_dir))
        
        result["local_repo"] = True
    except Exception as e:
        result["error"] = f"Local repo init failed: {e}"
        return result
    
    if skip_github:
        return result
    
    # Create GitHub repo
    token = get_github_token()
    if not token:
        result["error"] = "No GitHub PAT found, skipping GitHub repo creation"
        return result
    
    try:
        # Check if repo already exists
        existing = github_api("GET", f"/repos/{GITHUB_OWNER}/{slug}")
        if "id" in existing:
            result["github_repo"] = True
            result["github_url"] = existing.get("html_url", "")
        else:
            # Create the repo
            new_repo = github_api("POST", "/user/repos", {
                "name": slug,
                "description": f"PIF project: {slug}",
                "private": False,
                "auto_init": False,
            })
            if "id" in new_repo:
                result["github_repo"] = True
                result["github_url"] = new_repo.get("html_url", "")
            else:
                result["error"] = f"GitHub repo creation failed: {json.dumps(new_repo)}"
                return result
        
        # Add remote and push (use HTTPS URL without token — git credential helper provides auth)
        remote_url = f"https://github.com/{GITHUB_OWNER}/{slug}.git"
        run("git remote remove origin 2>/dev/null || true", cwd=str(repo_dir), check=False)
        run(f"git remote add origin {remote_url}", cwd=str(repo_dir))
        run("git push -u origin main", cwd=str(repo_dir), check=False)
        
    except Exception as e:
        result["error"] = f"GitHub setup failed: {e}"
    
    return result


# ---------------------------------------------------------------------------
# Step 8: Create backup repo
# ---------------------------------------------------------------------------

def create_backup_repo(slug: str, skip_github: bool = False) -> dict:
    """Create private backup repo on GitHub."""
    result = {"backup_repo": False, "backup_url": "", "error": ""}
    
    if skip_github:
        return result
    
    token = get_github_token()
    if not token:
        result["error"] = "No GitHub PAT found, skipping backup repo"
        return result
    
    backup_name = f"{slug}-backup"
    
    try:
        existing = github_api("GET", f"/repos/{GITHUB_OWNER}/{backup_name}")
        if "id" in existing:
            result["backup_repo"] = True
            result["backup_url"] = existing.get("html_url", "")
        else:
            new_repo = github_api("POST", "/user/repos", {
                "name": backup_name,
                "description": f"Backup repository for PIF project: {slug}",
                "private": True,  # Backups are always private
                "auto_init": True,  # Initialize with README so it's not empty
            })
            if "id" in new_repo:
                result["backup_repo"] = True
                result["backup_url"] = new_repo.get("html_url", "")
            else:
                result["error"] = f"Backup repo creation failed: {json.dumps(new_repo)}"
    except Exception as e:
        result["error"] = f"Backup repo setup failed: {e}"
    
    return result


# ---------------------------------------------------------------------------
# Step 9: Initialize SQLite database
# ---------------------------------------------------------------------------

def init_database(project_dir: Path, slug: str) -> Path:
    """Initialize the project's SQLite database with audit_log table."""
    import sqlite3
    
    db_path = project_dir / "database" / f"{slug}.db"
    conn = sqlite3.connect(str(db_path))
    
    # Create audit log table (Merkle DAG — Decision 3)
    # Each entry references the hash of the previous entry (cryptographic chain)
    conn.executescript('''
        CREATE TABLE IF NOT EXISTS audit_log (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT NOT NULL DEFAULT (datetime('now')),
            actor_type TEXT NOT NULL,      -- 'agent' | 'human' | 'system'
            actor_id TEXT NOT NULL,         -- session ID, agent runtime name, or username
            action TEXT NOT NULL,           -- what happened
            entity_type TEXT,               -- 'task' | 'config' | 'deploy' | etc.
            entity_id TEXT,                 -- ID of affected entity
            details TEXT,                   -- JSON blob with context
            prev_hash TEXT,                 -- hash of previous entry (Merkle chain)
            entry_hash TEXT NOT NULL,       -- hash of THIS entry
            compensation_for INTEGER,       -- if this is a correction, references original entry id
            FOREIGN KEY (compensation_for) REFERENCES audit_log(id)
        );
        
        CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
        CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor_type, actor_id);
        CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log(entity_type, entity_id);
        
        -- Project metadata table
        CREATE TABLE IF NOT EXISTS project_meta (
            key TEXT PRIMARY KEY,
            value TEXT,
            updated_at TEXT DEFAULT (datetime('now'))
        );
    ''')
    
    # Insert initial metadata
    conn.execute(
        "INSERT OR REPLACE INTO project_meta (key, value) VALUES (?, ?)",
        ("created_at", datetime.now(timezone.utc).isoformat())
    )
    conn.execute(
        "INSERT OR REPLACE INTO project_meta (key, value) VALUES (?, ?)",
        ("pif_version", "1.0")
    )
    conn.execute(
        "INSERT OR REPLACE INTO project_meta (key, value) VALUES (?, ?)",
        ("scaffold_date", datetime.now(timezone.utc).strftime("%Y-%m-%d"))
    )
    
    # Create genesis audit entry (chain starts here)
    import hashlib as h
    genesis_data = json.dumps({
        "action": "project_created",
        "actor_type": "system",
        "actor_id": "pif_scaffold",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "details": {"slug": slug, "method": "pif_scaffold.py"}
    })
    genesis_hash = h.sha256(genesis_data.encode()).hexdigest()
    conn.execute(
        "INSERT INTO audit_log (actor_type, actor_id, action, details, entry_hash) VALUES (?, ?, ?, ?, ?)",
        ("system", "pif_scaffold", "project_created", genesis_data, genesis_hash)
    )
    
    conn.commit()
    conn.close()
    
    return db_path


# ---------------------------------------------------------------------------
# Step 10: Create .hermes.md for the project (agent context)
# ---------------------------------------------------------------------------

def create_hermes_md(project_dir: Path, slug: str, display_name: str,
                     description: str) -> Path:
    """Create .hermes.md — the agent context file loaded into every session."""
    
    content = f'''# {display_name} — Agent Context

> {description}

## Project Identity
- **Slug:** {slug}
- **PIF Version:** 1.0
- **Isolated:** Yes — this project has its own database, skills, memories, KB, and research.
- **Agent Runtime:** Hermes (declared in vibepilot.toml)

## Rules
1. You are working on {display_name}, NOT VibePilot. Do not touch VibePilot code.
2. This project is isolated. Do not access other projects' directories, databases, or secrets.
3. Secrets are in the encrypted vault. Reference them by env var name, never by value.
4. Read vibepilot.toml for all configuration. Do not assume values.
5. Network egress is allowlisted. Check [network] in vibepilot.toml before external calls.
6. Every action is logged to the audit trail (append-only Merkle DAG).

## Key Files
- `vibepilot.toml` — Full project manifest
- `database/{slug}.db` — Project SQLite database (audit log, project_meta)
- `export.sh` / `restore.sh` — Transfer/restore scripts
- `config/` — Configuration files
- `skills/` — Project-specific agent skills
- `memories/` — Project memories (namespaced)
- `knowledgebase/` — Domain knowledge

## Audit Log
This project uses an append-only Merkle DAG audit trail (Decision 3).
Corrections are appended as compensation events, never edited.
Query: `sqlite3 database/{slug}.db "SELECT * FROM audit_log ORDER BY id DESC LIMIT 20"`
'''
    
    hermes_md_path = project_dir / ".hermes.md"
    hermes_md_path.write_text(content)
    return hermes_md_path


# ---------------------------------------------------------------------------
# Main scaffold function
# ---------------------------------------------------------------------------

def scaffold_project(slug: str, display_name: str = "", description: str = "",
                     deploy_target: str = "cloudflare", deploy_url: str = "",
                     skip_github: bool = False, json_output: bool = False) -> dict:
    """
    Execute the full PIF scaffold for a project.
    Returns a result dict with all created paths and statuses.
    """
    result = {
        "slug": slug,
        "display_name": display_name or slug.title(),
        "description": description,
        "project_dir": str(PROJECTS_BASE / slug),
        "steps": {},
        "errors": [],
        "success": False,
    }
    
    # Validate slug
    if not slug_valid(slug):
        result["errors"].append(f"Invalid slug: '{slug}'. Must be lowercase, start with a letter, alphanumeric + hyphens only.")
        if json_output:
            print(json.dumps(result, indent=2))
        return result
    
    # Check if already exists
    if slug_exists(slug):
        result["errors"].append(f"Project directory already exists: {PROJECTS_BASE / slug}")
        if json_output:
            print(json.dumps(result, indent=2))
        return result
    
    project_dir = PROJECTS_BASE / slug
    
    try:
        # Step 1: Directories
        dirs = create_directories(project_dir, slug)
        result["steps"]["directories"] = {"status": "ok", "count": len(dirs)}
        
        # Step 2: vibepilot.toml
        manifest = generate_manifest(project_dir, slug, display_name, description,
                                     deploy_target, deploy_url)
        result["steps"]["manifest"] = {"status": "ok", "path": str(manifest)}
        
        # Step 3: export.sh
        export_sh = generate_export_sh(project_dir, slug)
        result["steps"]["export_sh"] = {"status": "ok", "path": str(export_sh)}
        
        # Step 4: restore.sh
        restore_sh = generate_restore_sh(project_dir, slug)
        result["steps"]["restore_sh"] = {"status": "ok", "path": str(restore_sh)}
        
        # Step 5: README.md
        readme = generate_readme(project_dir, slug, display_name, description)
        result["steps"]["readme"] = {"status": "ok", "path": str(readme)}
        
        # Step 6: Hermes profile
        profile = create_hermes_profile(slug)
        result["steps"]["hermes_profile"] = {"status": "ok", "path": str(profile)}
        
        # Step 7: Git repo + GitHub
        git_result = init_git_repo(project_dir, slug, skip_github)
        result["steps"]["git_repo"] = git_result
        if git_result.get("error"):
            result["errors"].append(f"Git: {git_result['error']}")
        
        # Step 8: Backup repo
        backup_result = create_backup_repo(slug, skip_github)
        result["steps"]["backup_repo"] = backup_result
        if backup_result.get("error"):
            result["errors"].append(f"Backup: {backup_result['error']}")
        
        # Step 9: SQLite database
        db_path = init_database(project_dir, slug)
        result["steps"]["database"] = {"status": "ok", "path": str(db_path)}
        
        # Step 10: .hermes.md
        hermes_md = create_hermes_md(project_dir, slug, display_name, description)
        result["steps"]["hermes_md"] = {"status": "ok", "path": str(hermes_md)}
        
        result["success"] = True
        
    except Exception as e:
        result["errors"].append(f"Scaffold failed: {e}")
        import traceback
        result["errors"].append(traceback.format_exc())
    
    if json_output:
        print(json.dumps(result, indent=2))
    
    return result


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="PIF Scaffold System — Create an isolated project"
    )
    parser.add_argument("--slug", required=True, help="Project slug (lowercase, hyphens)")
    parser.add_argument("--display-name", default="", help="Display name")
    parser.add_argument("--description", default="", help="Project description")
    parser.add_argument("--deploy-target", default="cloudflare",
                       choices=["cloudflare", "vercel", "docker", "none"])
    parser.add_argument("--deploy-url", default="", help="Frontend URL")
    parser.add_argument("--skip-github", action="store_true",
                       help="Skip GitHub repo creation (local only)")
    parser.add_argument("--json", action="store_true",
                       help="Output result as JSON (for API integration)")
    
    args = parser.parse_args()
    
    result = scaffold_project(
        slug=args.slug,
        display_name=args.display_name,
        description=args.description,
        deploy_target=args.deploy_target,
        deploy_url=args.deploy_url,
        skip_github=args.skip_github,
        json_output=args.json,
    )
    
    if not args.json:
        if result["success"]:
            print(f"\n{'='*60}")
            print(f"  PIF SCAFFOLD COMPLETE: {result['slug']}")
            print(f"{'='*60}")
            print(f"  Project dir: {result['project_dir']}")
            print(f"  Steps completed: {len(result['steps'])}")
            for step, info in result["steps"].items():
                status = info.get("status", "?")
                if status == "ok":
                    print(f"    [OK] {step}")
                else:
                    detail = info.get("github_url") or info.get("error") or ""
                    print(f"    [{status}] {step} {detail}")
            if result["errors"]:
                print(f"\n  Warnings ({len(result['errors'])}):")
                for e in result["errors"]:
                    print(f"    - {e}")
            print(f"{'='*60}\n")
        else:
            print(f"\nSCAFFOLD FAILED for '{args.slug}':")
            for e in result["errors"]:
                print(f"  - {e}")
            sys.exit(1)


if __name__ == "__main__":
    main()
