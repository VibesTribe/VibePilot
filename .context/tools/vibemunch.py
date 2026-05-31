#!/usr/bin/env python3
"""
vibemunch.py - Universal code indexer for VibePilot.

Replaces jcodemunch/jdocmunch with a single Python script that indexes:
  - Go (.go)         -- via go-indexer binary or AST fallback
  - TypeScript (.ts) -- functions, classes, interfaces, types, exports
  - React TSX (.tsx) -- components, hooks, props interfaces
  - SQL (.sql)       -- tables, functions, indexes, triggers
  - Python (.py)     -- functions, classes, methods, imports
  - Shell (.sh)      -- functions
  - YAML (.yaml/.yml)-- keys at depth 1-2
  - JSON (.json)     -- top-level keys + type summary

Outputs: .context/index.db (SQLite) with tables:
  - symbols  (kind, name, qualified_name, package, signature, docstring, line_start, line_end, receiver, parent, exports, file)
  - edges    (source_qualified, target_qualified, kind, line, file)
  - files    (path, language, symbol_count, indexed_at)
  - meta     (key-value pairs about this index run)

Usage:
  python3 .context/tools/vibemunch.py [/path/to/repo] [--index ts,go,py]
  python3 .context/tools/vibemunch.py /home/vibes/vibepilot
  python3 .context/tools/vibemunch.py /home/vibes/vibeflow --index ts,tsx
"""

import json
import os
import re
import sqlite3
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

# ============================================================
# Configuration
# ============================================================
SKIP_DIRS = {
    'node_modules', '.git', 'dist', 'build', '.next', '__pycache__',
    'vendor', 'venv', '.venv', 'coverage', '.turbo', 'tmp', 'temp',
    'legacy', 'archive', 'backup'
}
SKIP_SUFFIXES = ('_test.go', '.test.ts', '.test.tsx', '.spec.ts', '.spec.tsx', '.min.js')

EXT_MAP = {
    '.go': 'go',
    '.ts': 'typescript',
    '.tsx': 'typescript',
    '.sql': 'sql',
    '.py': 'python',
    '.sh': 'shell',
    '.bash': 'shell',
    '.yaml': 'yaml',
    '.yml': 'yaml',
    '.json': 'json',
}

# ============================================================
# Indexers by language
# ============================================================

def index_go_file(filepath, rel_path, root):
    """Index a Go file. Tries go-indexer binary first, falls back to regex."""
    go_indexer = root / '.context' / 'tools' / 'go-indexer-bin'
    if not go_indexer.exists():
        go_indexer = Path('/home/vibes/knowledgebase/indexer/go-indexer-bin')

    if go_indexer.exists():
        try:
            result = subprocess.run(
                [str(go_indexer), str(root), rel_path],
                capture_output=True, text=True, timeout=30, cwd=str(root)
            )
            if result.returncode == 0 and result.stdout.strip():
                return parse_go_indexer_json(result.stdout, rel_path)
        except (subprocess.TimeoutExpired, Exception):
            pass

    # Regex fallback
    return index_go_regex(filepath, rel_path)


def parse_go_indexer_json(json_str, rel_path):
    """Parse JSON output from go-indexer binary."""
    symbols = []
    edges = []
    try:
        data = json.loads(json_str)
        if isinstance(data, list):
            for file_result in data:
                for s in file_result.get('symbols', []):
                    symbols.append({
                        'kind': s.get('kind', 'function'),
                        'name': s.get('name', ''),
                        'qualified_name': s.get('qualified_name', ''),
                        'package': s.get('package', ''),
                        'signature': s.get('signature', ''),
                        'docstring': s.get('docstring', ''),
                        'line_start': s.get('line_start', 0),
                        'line_end': s.get('line_end', 0),
                        'receiver': s.get('receiver', ''),
                        'parent': s.get('parent', ''),
                        'file': file_result.get('file_path', rel_path),
                    })
                for e in file_result.get('edges', []):
                    edges.append({
                        'source_qualified': e.get('source_qualified', ''),
                        'target_qualified': e.get('target_qualified', ''),
                        'kind': e.get('kind', ''),
                        'line': e.get('line', 0),
                        'file': file_result.get('file_path', rel_path),
                    })
    except json.JSONDecodeError:
        pass
    return symbols, edges


def index_go_regex(filepath, rel_path):
    """Regex-based Go indexing fallback."""
    symbols = []
    edges = []
    try:
        text = Path(filepath).read_text(errors='replace')
    except:
        return symbols, edges

    pkg_match = re.search(r'^package\s+(\w+)', text, re.MULTILINE)
    pkg = pkg_match.group(1) if pkg_match else ''

    # Functions
    for m in re.finditer(r'^func\s+(\w+)\s*\(([^)]*)\)\s*(?:\(([^)]*)\))?', text, re.MULTILINE):
        name = m.group(1)
        params = m.group(2).strip()
        returns = m.group(3).strip() if m.group(3) else ''
        sig = f"func {name}({params})"
        if returns:
            sig += f" ({returns})"
        symbols.append({
            'kind': 'function', 'name': name,
            'qualified_name': f"{pkg}.{name}", 'package': pkg,
            'signature': sig, 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # Methods
    for m in re.finditer(r'^func\s+\(([^)]+)\)\s+(\w+)\s*\(([^)]*)\)\s*(?:\(([^)]*)\))?', text, re.MULTILINE):
        receiver = m.group(1).strip()
        name = m.group(2)
        params = m.group(3).strip()
        returns = m.group(4).strip() if m.group(4) else ''
        recv_type = re.sub(r'\*?', '', receiver.split()[-1] if receiver else '')
        qname = f"{pkg}.({recv_type}).{name}"
        sig = f"func ({receiver}) {name}({params})"
        if returns:
            sig += f" ({returns})"
        symbols.append({
            'kind': 'method', 'name': name,
            'qualified_name': qname, 'package': pkg,
            'signature': sig, 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': receiver, 'parent': recv_type, 'file': rel_path,
        })

    # Types
    for m in re.finditer(r'^type\s+(\w+)\s+(struct|interface|string|int\b|func)', text, re.MULTILINE):
        name = m.group(1)
        kind = 'struct' if m.group(2) == 'struct' else ('interface' if m.group(2) == 'interface' else 'type')
        symbols.append({
            'kind': kind, 'name': name,
            'qualified_name': f"{pkg}.{name}", 'package': pkg,
            'signature': m.group(2), 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    return symbols, edges


def index_typescript(filepath, rel_path):
    """Index TypeScript/TSX files for functions, classes, interfaces, types, components."""
    symbols = []
    edges = []
    try:
        text = Path(filepath).read_text(errors='replace')
    except:
        return symbols, edges

    lines = text.split('\n')

    # Imports -> edges
    for m in re.finditer(r"import\s+.*?(?:from|)\s*['\"]([^'\"]+)['\"]", text):
        edges.append({
            'source_qualified': rel_path, 'target_qualified': m.group(1),
            'kind': 'IMPORTS', 'line': 0, 'file': rel_path,
        })

    # Export default function Component
    for m in re.finditer(r'export\s+default\s+function\s+(\w+)\s*(?:<[^>]*>)?\s*\(([^)]*)\)', text):
        name = m.group(1)
        symbols.append({
            'kind': 'component', 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': f"function {name}({m.group(2).strip()})",
            'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # const Component = () => or const Component: React.FC
    for m in re.finditer(r'(?:export\s+(?:default\s+)?)?const\s+(\w+)\s*(?::\s*\w+(?:<[^>]*>)?)?\s*=\s*(?:\([^)]*\)|(\w+))\s*=>', text):
        name = m.group(1)
        kind = 'component' if name[0].isupper() and rel_path.endswith('.tsx') else 'function'
        symbols.append({
            'kind': kind, 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': f"const {name} = ... =>", 'docstring': '',
            'line_start': 0, 'line_end': 0, 'receiver': '', 'parent': '', 'file': rel_path,
        })

    # export function / async function
    for m in re.finditer(r'(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*(?:<[^>]*>)?\s*\(([^)]*)\)(?:\s*:\s*([^{\n]+))?', text):
        name = m.group(1)
        kind = 'component' if name[0].isupper() and rel_path.endswith('.tsx') else 'function'
        ret = m.group(3).strip() if m.group(3) else ''
        sig = f"function {name}({m.group(2).strip()})"
        if ret:
            sig += f": {ret}"
        symbols.append({
            'kind': kind, 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': sig, 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # interface Name { ... }
    for m in re.finditer(r'(?:export\s+)?interface\s+(\w+)(?:<[^>]*>)?(?:\s+extends\s+(\w+))?', text):
        name = m.group(1)
        exports_list = []
        if m.group(2):
            edges.append({
                'source_qualified': f"{rel_path}::{name}",
                'target_qualified': m.group(2), 'kind': 'EXTENDS', 'line': 0, 'file': rel_path,
            })
        symbols.append({
            'kind': 'interface', 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': 'interface{...}', 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'exports': '', 'file': rel_path,
        })

    # type Name = ...
    for m in re.finditer(r'(?:export\s+)?type\s+(\w+)(?:<[^>]*>)?\s*=', text):
        name = m.group(1)
        symbols.append({
            'kind': 'type', 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': 'type', 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # class Name { ... }
    for m in re.finditer(r'(?:export\s+)?class\s+(\w+)(?:<[^>]*>)?(?:\s+extends\s+(\w+))?(?:\s+implements\s+(\w+))?', text):
        name = m.group(1)
        if m.group(2):
            edges.append({
                'source_qualified': f"{rel_path}::{name}",
                'target_qualified': m.group(2), 'kind': 'EXTENDS', 'line': 0, 'file': rel_path,
            })
        symbols.append({
            'kind': 'class', 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': 'class', 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # Hooks: const [state, setter] = useState(...)
    for m in re.finditer(r'const\s+\[([^,\]]+),?\s*[^=\]]*\]\s*=\s*(use\w+)\s*<', text):
        state_name = m.group(1).strip()
        hook_name = m.group(2)
        edges.append({
            'source_qualified': rel_path,
            'target_qualified': f"react::{hook_name}", 'kind': 'USES_HOOK', 'line': 0, 'file': rel_path,
        })

    return symbols, edges


def index_sql(filepath, rel_path):
    """Index SQL files for tables, functions, indexes, triggers."""
    symbols = []
    try:
        text = Path(filepath).read_text(errors='replace')
    except:
        return symbols, []

    # CREATE TABLE
    for m in re.finditer(r'CREATE\s+(?:OR\s+REPLACE\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:public\.)?(\w+)', text, re.IGNORECASE):
        symbols.append({
            'kind': 'table', 'name': m.group(1),
            'qualified_name': f"public.{m.group(1)}", 'package': 'public',
            'signature': 'TABLE', 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # CREATE FUNCTION
    for m in re.finditer(r'CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(?:public\.)?(\w+)\s*\(([^)]*)\)', text, re.IGNORECASE):
        name = m.group(1)
        params = m.group(2).strip()
        ret_match = re.search(r'RETURNS\s+(.+?)(?:\s+(?:LANGUAGE|\$|AS|BEGIN))', text[m.end():], re.IGNORECASE)
        ret = ret_match.group(1).strip() if ret_match else ''
        sig = f"FUNCTION {name}({params})"
        if ret:
            sig += f" RETURNS {ret}"
        symbols.append({
            'kind': 'function', 'name': name,
            'qualified_name': f"public.{name}", 'package': 'public',
            'signature': sig, 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # CREATE INDEX
    for m in re.finditer(r'CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s+ON\s+(?:public\.)?(\w+)', text, re.IGNORECASE):
        symbols.append({
            'kind': 'index', 'name': m.group(1),
            'qualified_name': f"idx.{m.group(1)}", 'package': m.group(2),
            'signature': f"ON {m.group(2)}", 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    # CREATE TRIGGER
    for m in re.finditer(r'CREATE\s+(?:OR\s+REPLACE\s+)?TRIGGER\s+(\w+)', text, re.IGNORECASE):
        symbols.append({
            'kind': 'trigger', 'name': m.group(1),
            'qualified_name': f"trigger.{m.group(1)}", 'package': '',
            'signature': 'TRIGGER', 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    return symbols, []


def index_python(filepath, rel_path):
    """Index Python files for functions, classes, methods."""
    symbols = []
    edges = []
    try:
        text = Path(filepath).read_text(errors='replace')
    except:
        return symbols, edges

    lines = text.split('\n')

    # Imports
    for m in re.finditer(r'^(?:from\s+(\S+)\s+)?import\s+(.+)$', text, re.MULTILINE):
        module = m.group(1) or ''
        imports = m.group(2).strip()
        if module:
            edges.append({
                'source_qualified': rel_path, 'target_qualified': module,
                'kind': 'IMPORTS', 'line': 0, 'file': rel_path,
            })

    # Classes
    for m in re.finditer(r'^class\s+(\w+)(?:\(([^)]+)\))?:', text, re.MULTILINE):
        name = m.group(1)
        parent = m.group(2).strip() if m.group(2) else ''
        symbols.append({
            'kind': 'class', 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': f"class {name}({parent})" if parent else f"class {name}",
            'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': parent, 'file': rel_path,
        })

    # Functions
    for m in re.finditer(r'^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)', text, re.MULTILINE):
        name = m.group(1)
        params = m.group(2).strip()
        kind = 'method' if name == '__init__' else 'function'
        symbols.append({
            'kind': kind, 'name': name,
            'qualified_name': f"{rel_path}::{name}", 'package': '',
            'signature': f"def {name}({params})", 'docstring': '',
            'line_start': 0, 'line_end': 0, 'receiver': '', 'parent': '', 'file': rel_path,
        })

    return symbols, edges


def index_shell(filepath, rel_path):
    """Index shell scripts for functions."""
    symbols = []
    try:
        text = Path(filepath).read_text(errors='replace')
    except:
        return symbols, []

    for m in re.finditer(r'^(?:function\s+)?(\w+)\s*\(\)\s*\{', text, re.MULTILINE):
        symbols.append({
            'kind': 'function', 'name': m.group(1),
            'qualified_name': f"{rel_path}::{m.group(1)}", 'package': '',
            'signature': f"{m.group(1)}()", 'docstring': '',
            'line_start': 0, 'line_end': 0, 'receiver': '', 'parent': '', 'file': rel_path,
        })

    return symbols, []


def index_yaml(filepath, rel_path):
    """Index YAML files for top-level and second-level keys."""
    symbols = []
    try:
        text = Path(filepath).read_text(errors='replace')
    except:
        return symbols, []

    keys = set()
    for m in re.finditer(r'^(\w[\w-]*)\s*:', text, re.MULTILINE):
        keys.add(m.group(1))
    for m in re.finditer(r'^\s{2}(\w[\w-]*)\s*:', text, re.MULTILINE):
        keys.add(m.group(1))

    for key in sorted(keys):
        symbols.append({
            'kind': 'config_key', 'name': key,
            'qualified_name': f"{rel_path}::{key}", 'package': '',
            'signature': 'key', 'docstring': '', 'line_start': 0, 'line_end': 0,
            'receiver': '', 'parent': '', 'file': rel_path,
        })

    return symbols, []


def index_json(filepath, rel_path):
    """Index JSON files for top-level keys and type summary."""
    symbols = []
    try:
        text = Path(filepath).read_text(errors='replace')
        data = json.loads(text)
    except:
        return symbols, []

    if isinstance(data, dict):
        for key in sorted(data.keys()):
            val = data[key]
            val_type = type(val).__name__
            if isinstance(val, list):
                val_type = f"list[{len(val)}]"
            elif isinstance(val, dict):
                val_type = f"dict[{len(val)} keys]"
            symbols.append({
                'kind': 'config_key', 'name': key,
                'qualified_name': f"{rel_path}::{key}", 'package': '',
                'signature': val_type, 'docstring': '', 'line_start': 0, 'line_end': 0,
                'receiver': '', 'parent': '', 'file': rel_path,
            })

    return symbols, []


# ============================================================
# Main indexer
# ============================================================

INDEXERS = {
    'go': index_go_file,
    'typescript': index_typescript,
    'sql': index_sql,
    'python': index_python,
    'shell': index_shell,
    'yaml': index_yaml,
    'json': index_json,
}


def walk_files(repo_root, target_langs=None):
    """Walk repo and yield (filepath, rel_path, language) tuples."""
    for dirpath, dirnames, filenames in os.walk(repo_root):
        # Skip blacklisted directories
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS and not d.startswith('.')]

        for filename in sorted(filenames):
            filepath = os.path.join(dirpath, filename)
            rel_path = os.path.relpath(filepath, repo_root)

            if any(rel_path.endswith(s) for s in SKIP_SUFFIXES):
                continue

            ext = os.path.splitext(filename)[1]
            lang = EXT_MAP.get(ext)
            if not lang:
                continue

            if target_langs and lang not in target_langs:
                continue

            yield filepath, rel_path, lang


def build_index_db(repo_root, target_langs=None):
    """Build the index.db SQLite database."""
    root = Path(repo_root)
    ctx_dir = root / '.context'
    ctx_dir.mkdir(exist_ok=True)
    db_path = ctx_dir / 'index.db'

    # Remove old index
    if db_path.exists():
        db_path.unlink()

    conn = sqlite3.connect(str(db_path))
    c = conn.cursor()

    # Create tables
    c.execute('''CREATE TABLE IF NOT EXISTS symbols (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        kind TEXT NOT NULL,
        name TEXT NOT NULL,
        qualified_name TEXT NOT NULL,
        package TEXT DEFAULT '',
        signature TEXT DEFAULT '',
        docstring TEXT DEFAULT '',
        line_start INTEGER DEFAULT 0,
        line_end INTEGER DEFAULT 0,
        receiver TEXT DEFAULT '',
        parent TEXT DEFAULT '',
        exports TEXT DEFAULT '',
        file TEXT NOT NULL
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS edges (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        source_qualified TEXT NOT NULL,
        target_qualified TEXT NOT NULL,
        kind TEXT NOT NULL,
        line INTEGER DEFAULT 0,
        file TEXT NOT NULL
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS files (
        path TEXT PRIMARY KEY,
        language TEXT NOT NULL,
        symbol_count INTEGER DEFAULT 0,
        edge_count INTEGER DEFAULT 0,
        indexed_at TEXT NOT NULL
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS meta (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL
    )''')

    # Create indexes for fast querying
    c.execute('CREATE INDEX IF NOT EXISTS idx_sym_name ON symbols(name)')
    c.execute('CREATE INDEX IF NOT EXISTS idx_sym_kind ON symbols(kind)')
    c.execute('CREATE INDEX IF NOT EXISTS idx_sym_file ON symbols(file)')
    c.execute('CREATE INDEX IF NOT EXISTS idx_sym_qname ON symbols(qualified_name)')
    c.execute('CREATE INDEX IF NOT EXISTS idx_edge_source ON edges(source_qualified)')
    c.execute('CREATE INDEX IF NOT EXISTS idx_edge_target ON edges(target_qualified)')

    now = datetime.now(timezone.utc).isoformat()
    commit = 'unknown'
    branch = 'unknown'
    try:
        commit = subprocess.run(
            ['git', 'rev-parse', '--short', 'HEAD'],
            capture_output=True, text=True, cwd=str(root)
        ).stdout.strip()
        branch = subprocess.run(
            ['git', 'branch', '--show-current'],
            capture_output=True, text=True, cwd=str(root)
        ).stdout.strip()
    except:
        pass

    # Insert meta
    for key, val in [
        ('indexed_at', now),
        ('commit', commit),
        ('branch', branch),
        ('repo_root', str(root)),
        ('indexer', 'vibemunch.py v1.0'),
    ]:
        c.execute('INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)', (key, val))

    # Index all files
    total_symbols = 0
    total_edges = 0
    total_files = 0
    lang_counts = {}

    for filepath, rel_path, lang in walk_files(repo_root, target_langs):
        indexer = INDEXERS.get(lang)
        if not indexer:
            continue

        try:
            if lang == 'go':
                result = indexer(filepath, rel_path, root)
            else:
                result = indexer(filepath, rel_path)
        except Exception as e:
            print(f"[vibemunch] Error indexing {rel_path}: {e}", file=sys.stderr)
            continue
        if isinstance(result, tuple):
            file_symbols, file_edges = result
        else:
            file_symbols = result
            file_edges = []

        if not file_symbols and not file_edges:
            continue

        # Insert symbols
        for s in file_symbols:
            c.execute(
                'INSERT INTO symbols (kind, name, qualified_name, package, signature, docstring, line_start, line_end, receiver, parent, exports, file) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
                (s['kind'], s['name'], s['qualified_name'], s.get('package', ''),
                 s['signature'], s.get('docstring', ''), s.get('line_start', 0),
                 s.get('line_end', 0), s.get('receiver', ''), s.get('parent', ''),
                 s.get('exports', ''), s['file'])
            )

        # Insert edges
        for e in file_edges:
            c.execute(
                'INSERT INTO edges (source_qualified, target_qualified, kind, line, file) VALUES (?, ?, ?, ?, ?)',
                (e['source_qualified'], e['target_qualified'], e['kind'],
                 e.get('line', 0), e.get('file', rel_path))
            )

        # Track file
        c.execute(
            'INSERT OR REPLACE INTO files (path, language, symbol_count, edge_count, indexed_at) VALUES (?, ?, ?, ?, ?)',
            (rel_path, lang, len(file_symbols), len(file_edges), now)
        )

        total_symbols += len(file_symbols)
        total_edges += len(file_edges)
        total_files += 1
        lang_counts[lang] = lang_counts.get(lang, 0) + 1

    conn.commit()

    # Final meta
    c.execute('INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)', ('total_symbols', str(total_symbols)))
    c.execute('INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)', ('total_edges', str(total_edges)))
    c.execute('INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)', ('total_files', str(total_files)))
    c.execute('INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)', ('languages', json.dumps(lang_counts)))

    conn.commit()
    conn.close()

    db_size = db_path.stat().st_size
    print(f"[vibemunch] Indexed {total_files} files, {total_symbols} symbols, {total_edges} edges")
    print(f"[vibemunch] Languages: {', '.join(f'{k}={v}' for k, v in sorted(lang_counts.items()))}")
    print(f"[vibemunch] Output: {db_path} ({db_size:,} bytes)")
    print(f"[vibemunch] Commit: {commit} | Branch: {branch}")

    return db_path


def main():
    repo_root = sys.argv[1] if len(sys.argv) > 1 else '.'
    target_langs = None

    for i, arg in enumerate(sys.argv):
        if arg == '--index' and i + 1 < len(sys.argv):
            target_langs = set(sys.argv[i + 1].split(','))

    if not Path(repo_root).is_dir():
        print(f"Error: {repo_root} is not a directory", file=sys.stderr)
        sys.exit(1)

    build_index_db(repo_root, target_langs)


if __name__ == '__main__':
    main()
