#!/bin/bash
# .context/tools/install.sh - One-command install of .context tooling
# Usage: bash .context/tools/install.sh
# Requires: curl, pipx

set -euo pipefail

echo "[.context-tools] Installing compression toolchain..."

# 1. lean-ctx
if command -v lean-ctx >/dev/null 2>&1; then
    echo "[.context-tools] lean-ctx already installed: $(lean-ctx --version 2>/dev/null)"
else
    echo "[.context-tools] Installing lean-ctx..."
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64) BINARY="lean-ctx-x86_64-unknown-linux-gnu.tar.gz" ;;
        aarch64) BINARY="lean-ctx-aarch64-unknown-linux-gnu.tar.gz" ;;
        *) echo "ERROR: Unsupported arch $ARCH"; exit 1 ;;
    esac
    
    # Get latest release URL
    URL=$(curl -fsSL https://api.github.com/repos/yvgude/lean-ctx/releases/latest | \
          python3 -c "import json,sys; [print(a["browser_download_url"]) for a in json.load(sys.stdin)["assets"] if "$BINARY" in a["name"]]" 2>/dev/null)
    
    if [ -z "$URL" ]; then
        echo "ERROR: Could not find lean-ctx release for $ARCH"
        echo "Download manually from: https://github.com/yvgude/lean-ctx/releases"
        exit 1
    fi
    
    mkdir -p ~/.local/bin
    TMPDIR=$(mktemp -d)
    curl -L -o "$TMPDIR/lean-ctx.tar.gz" "$URL"
    tar xzf "$TMPDIR/lean-ctx.tar.gz" -C "$TMPDIR/"
    cp "$TMPDIR/lean-ctx" ~/.local/bin/lean-ctx
    chmod +x ~/.local/bin/lean-ctx
    rm -rf "$TMPDIR"
    echo "[.context-tools] lean-ctx installed: $(lean-ctx --version)"
fi

# 2. jCodeMunch
if command -v jcodemunch-mcp >/dev/null 2>&1; then
    echo "[.context-tools] jCodeMunch already installed"
else
    echo "[.context-tools] Installing jCodeMunch..."
    pipx install jcodemunch-mcp 2>/dev/null || pip install --user jcodemunch-mcp 2>/dev/null
    if command -v jcodemunch-mcp >/dev/null 2>&1; then
        echo "[.context-tools] jCodeMunch installed"
    else
        echo "WARNING: jCodeMunch install failed. index.db will not be generated."
        echo "Try manually: pipx install jcodemunch-mcp"
    fi
fi

# 3. Verify
echo ""
echo "[.context-tools] Verification:"
command -v lean-ctx >/dev/null 2>&1 && echo "  lean-ctx: $(lean-ctx --version 2>/dev/null)" || echo "  lean-ctx: MISSING"
command -v jcodemunch-mcp >/dev/null 2>&1 && echo "  jCodeMunch: installed" || echo "  jCodeMunch: MISSING"
echo ""
echo "[.context-tools] Run 'bash .context/build.sh' to generate the knowledge layer."
