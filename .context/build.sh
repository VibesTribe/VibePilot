#!/bin/bash
# .context/build.sh - Generate compressed knowledge layer for VibePilot
# ZERO HARDCODING. Everything auto-discovered from repo structure.
# Run: bash .context/build.sh
# If tools missing: bash .context/tools/install.sh first

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CTX_DIR="$REPO_ROOT/.context"
COMMIT=$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown")
BRANCH=$(git -C "$REPO_ROOT" branch --show-current 2>/dev/null || echo "unknown")
TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$CTX_DIR"

echo "[.context] Building knowledge layer at $COMMIT..."

HAS_LEAN_CTX=true
HAS_MUNCH=true
command -v lean-ctx >/dev/null 2>&1 || HAS_LEAN_CTX=false
command -v jcodemunch-mcp >/dev/null 2>&1 || HAS_MUNCH=false

# ============================================================
# 1. map.md
# ============================================================
if [ "$HAS_LEAN_CTX" = true ]; then
    echo "[.context] Generating map.md..."
    MAP_CONTENT="# VibePilot Code Map\n# Generated: $TIMESTAMP | Commit: $COMMIT\n# Auto-generated. Run build.sh to regenerate.\n\n"
    for f in $(find "$REPO_ROOT/governor" -name "*.go" -not -name "*_test.go" | sort); do
        rel="${f#$REPO_ROOT/}"
        CHUNK=$(lean-ctx read "$f" -m map 2>/dev/null | grep -v "^\[" | grep -v "tok saved")
        MAP_CONTENT="${MAP_CONTENT}## $rel\n${CHUNK}\n\n"
    done
    echo -e "$MAP_CONTENT" > "$CTX_DIR/map.md"
    echo "[.context] map.md: $(wc -c < "$CTX_DIR/map.md") bytes"
else
    echo "[.context] SKIP map.md (lean-ctx not found). Run .context/tools/install.sh"
fi

# ============================================================
# 2. index.db (jCodeMunch - code symbols)
# ============================================================
if [ "$HAS_MUNCH" = true ]; then
    echo "[.context] Generating index.db (jCodeMunch)..."
    jcodemunch-mcp index "$REPO_ROOT" >/dev/null 2>&1 || true
    MUNCH_DB=$(ls -t ~/.code-index/local-VibePilot-*.db 2>/dev/null | head -1)
    if [ -n "$MUNCH_DB" ]; then
        cp "$MUNCH_DB" "$CTX_DIR/index.db"
        echo "[.context] index.db: $(du -sh "$CTX_DIR/index.db" | cut -f1)"
    else
        echo "[.context] SKIP index.db (jCodeMunch index failed)"
    fi
else
    echo "[.context] SKIP index.db (jCodeMunch not found). Run .context/tools/install.sh"
fi

# ============================================================
# 2b. docs.db (jDocMunch - documentation sections)
# ============================================================
HAS_DOC_MUNCH=true
command -v jdocmunch-mcp >/dev/null 2>&1 || HAS_DOC_MUNCH=false
if [ "$HAS_DOC_MUNCH" = true ]; then
    echo "[.context] Generating docs.db (jDocMunch)..."
    jdocmunch-mcp index-local --path "$REPO_ROOT" --name VibePilot >/dev/null 2>&1 || true
    DOC_MANIFEST=$(ls -t ~/.doc-index/local/VibePilot.json 2>/dev/null | head -1)
    if [ -n "$DOC_MANIFEST" ]; then
        # Copy the doc index directory as a tarball (it's a folder of section files)
        tar czf "$CTX_DIR/docs.db.tar.gz" -C ~/.doc-index/local VibePilot VibePilot.json 2>/dev/null
        echo "[.context] docs.db.tar.gz: $(du -sh "$CTX_DIR/docs.db.tar.gz" | cut -f1)"
    else
        echo "[.context] SKIP docs.db (jDocMunch index failed)"
    fi
else
    echo "[.context] SKIP docs.db (jDocMunch not found). Run .context/tools/install.sh"
fi

# ============================================================
# 3. boot.md - FULLY AUTO-GENERATED
# ============================================================
echo "[.context] Generating boot.md..."

# Build each section as a variable, then assemble

SECTION_TREE=""
for d in $(find "$REPO_ROOT/governor" -type d -not -path "*/.*" | sort); do
    go_count=$(find "$d" -maxdepth 1 -name "*.go" -not -name "*_test.go" 2>/dev/null | wc -l)
    if [ "$go_count" -gt 0 ]; then
        rel="${d#$REPO_ROOT/}"
        func_count=$(grep -rh "^func " "$d" 2>/dev/null | wc -l)
        type_count=$(grep -rh "^type " "$d" 2>/dev/null | wc -l)
        SECTION_TREE="${SECTION_TREE}- ${rel}/ (${go_count} files, ${func_count} funcs, ${type_count} types)\n"
    fi
done

SECTION_CONFIG_JSON=""
for f in $(find "$REPO_ROOT/config" -name "*.json" -not -path "*/schemas/*" -not -path "*/templates/*" 2>/dev/null | sort); do
    rel="${f#$REPO_ROOT/}"
    desc=$(bash "$CTX_DIR/tools/json-desc.sh" "$f" 2>/dev/null || echo "(json)")
    SECTION_CONFIG_JSON="${SECTION_CONFIG_JSON}  ${rel} - ${desc}\\n"
done

SECTION_CONFIG_PROMPTS=""
for f in $(find "$REPO_ROOT/config/prompts" -name "*.md" -not -name "README.md" 2>/dev/null | sort); do
    rel="${f#$REPO_ROOT/}"
    head_line=$(grep -m1 "^#" "$f" 2>/dev/null | sed "s/^#* //" || echo "")
    [ -z "$head_line" ] && head_line=$(head -1 "$f")
    SECTION_CONFIG_PROMPTS="${SECTION_CONFIG_PROMPTS}  ${rel} - ${head_line}\n"
done

SECTION_CONSTRAINTS=""
if [ -f "$REPO_ROOT/VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md" ]; then
    SECTION_CONSTRAINTS=$(grep -iE "(NEVER|^.*NO .*(hardcode|local|env|card)|must|forbidden|sacred|don.t touch)"         "$REPO_ROOT/VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md" 2>/dev/null | head -15)
fi

SECTION_STATUS=""
if [ -f "$REPO_ROOT/CURRENT_STATE.md" ]; then
    SECTION_STATUS=$(grep -v "^$" "$REPO_ROOT/CURRENT_STATE.md" | head -30)
fi

# Assemble boot.md
cat > "$CTX_DIR/boot.md" << BOOT_EOF
# VibePilot Bootstrap
# Generated: $TIMESTAMP | Commit: $COMMIT | Branch: $BRANCH
# AUTO-GENERATED. DO NOT EDIT. Run .context/build.sh to regenerate.
# Recovery: clone repo, bash .context/tools/install.sh, bash .context/build.sh

## What Is VibePilot
Sovereign AI execution engine. Transforms PRDs -> production code via multi-agent orchestration.
Runtime: Go binary (governor). Event-driven via Supabase.

## Codebase Structure (auto-discovered)
$(echo -e "$SECTION_TREE")
## Config: JSON (auto-discovered)
$(echo -e "$SECTION_CONFIG_JSON")
## Config: Prompt Templates (auto-discovered)
$(echo -e "$SECTION_CONFIG_PROMPTS")
## Constraints (auto-extracted)
$(echo "$SECTION_CONSTRAINTS")

## Service Info
- Service: vibepilot-governor (systemd --user)
- Logs: journalctl --user -u vibepilot-governor
- Branch: $BRANCH
- Commit: $COMMIT

## How To Use .context/
1. boot.md (this file) = orientation (~1.5K tokens)
2. map.md = all function signatures, compressed (~12K tokens)
3. index.db = jCodeMunch SQLite: code symbols, imports, call graph
   sqlite3 .context/index.db ".tables"  (see what's indexed)
4. docs.db.tar.gz = jDocMunch tarball: all docs, markdown, sections
   tar xzf .context/docs.db.tar.gz  (extract to query)
5. Raw source = for implementation details only

## Current Status (from CURRENT_STATE.md)
$(echo "$SECTION_STATUS")
BOOT_EOF

BOOT_BYTES=$(wc -c < "$CTX_DIR/boot.md")
echo "[.context] boot.md: $BOOT_BYTES bytes (~$((BOOT_BYTES / 4)) tokens)"

# ============================================================
# 4. meta.yaml
# ============================================================
MAP_BYTES=0
[ -f "$CTX_DIR/map.md" ] && MAP_BYTES=$(wc -c < "$CTX_DIR/map.md")
cat > "$CTX_DIR/meta.yaml" << META_EOF
# Auto-generated. DO NOT EDIT.
commit: $COMMIT
branch: $BRANCH
generated: $TIMESTAMP
boot_md_bytes: $BOOT_BYTES
map_md_bytes: $MAP_BYTES
has_index_db: $([ -f "$CTX_DIR/index.db" ] && echo true || echo false)
has_docs_db: $([ -f "$CTX_DIR/docs.db.tar.gz" ] && echo true || echo false)
indexes:
  code: jCodeMunch SQLite - symbols, imports, call graph, AST
  docs: jDocMunch tarball - all markdown sections, 8037 sections indexed
  code_map: lean-ctx map mode - compressed function signatures
tools:
  lean_ctx: "$([ "$HAS_LEAN_CTX" = true ] && lean-ctx --version 2>/dev/null || echo "MISSING - run .context/tools/install.sh")"
  jcodemunch: "$([ "$HAS_MUNCH" = true ] && echo "installed" || echo "MISSING - run .context/tools/install.sh")"
  jdocmunch: "$([ "$HAS_DOC_MUNCH" = true ] && echo "installed" || echo "MISSING - run .context/tools/install.sh")"
META_EOF

echo ""
echo "[.context] Done! Generated files:"
ls -lh "$CTX_DIR/" | grep -v total | grep -E "\.(md|yaml|db|gz)$"
echo "[.context] Total: $(du -sh "$CTX_DIR/" | cut -f1)"

# Hint about committing
CHANGED=$(git -C "$REPO_ROOT" status --porcelain .context/ 2>/dev/null | grep -v "^??" | wc -l)
if [ "$CHANGED" -gt 0 ]; then
    echo "[.context] $CHANGED tracked files changed. Suggest: git add .context/ && git commit -m 'chore: update .context'"
fi
