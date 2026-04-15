#!/usr/bin/env python3
"""
build-knowledge-db.py - Build knowledge.db from all VibePilot sources.

Creates a single SQLite database with 4 tables:
  rules    - All rules/principles, deduplicated, priority-ranked
  prompts  - All prompt templates with role, summary, key instructions
  configs  - All config files with purpose and key fields
  docs     - All documentation sections, searchable

This replaces the jDocMunch tarball with something actually queryable.

Usage: python3 .context/tools/build-knowledge-db.py [repo_root]
"""

import json
import os
import re
import sqlite3
import sys
from pathlib import Path

REPO_ROOT = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).resolve().parent.parent.parent
DB_PATH = REPO_ROOT / ".context" / "knowledge.db"

def md_headings(text):
    """Extract headings with their line numbers."""
    results = []
    for i, line in enumerate(text.split("\n"), 1):
        m = re.match(r'^(#{1,6})\s+(.+)', line)
        if m:
            results.append((len(m.group(1)), m.group(2).strip(), i))
    return results

def section_text(text, start_line, end_line=None):
    """Extract text between line numbers."""
    lines = text.split("\n")
    if end_line:
        return "\n".join(lines[start_line-1:end_line-1]).strip()
    return "\n".join(lines[start_line-1:]).strip()

def extract_between_heading(text, heading_line, headings):
    """Get text from heading_line to next heading of same or lower level."""
    h_level, _, h_lineno = None, None, heading_line
    for level, title, lineno in headings:
        if lineno == heading_line:
            h_level = level
            break
    if h_level is None:
        return ""
    for level, title, lineno in headings:
        if lineno > heading_line and level <= h_level:
            return section_text(text, heading_line, lineno).strip()
    return section_text(text, heading_line).strip()

# ============================================================
# RULES EXTRACTION
# ============================================================

def extract_rules():
    """Extract all rules from all sources, deduplicate, prioritize.
    
    Primary source: .context/tools/tier0-static.md (hand-crafted, curated)
    Supplementary: coding rules, guardrails from other docs (no role/principle duplication)
    """
    rules = []
    
    # ============================================================
    # PRIMARY SOURCE: tier0-static.md -- hand-crafted, no garbage
    # ============================================================
    tier0_path = REPO_ROOT / ".context" / "tools" / "tier0-static.md"
    if tier0_path.exists():
        tier0 = tier0_path.read_text()
        
        # Parse the structured sections
        current_section = ""
        for line in tier0.split("\n"):
            # Detect sections
            if line.startswith("## Core Principles"):
                current_section = "principle"
                continue
            elif line.startswith("## Absolute Rules"):
                current_section = "absolute"
                continue
            elif line.startswith("## Operational Rules"):
                current_section = "operational"
                continue
            elif line.startswith("## Human Boundaries"):
                current_section = "human"
                continue
            elif line.startswith("## ") or line.startswith("# "):
                current_section = ""
                continue
            
            # Skip non-content lines
            if not line.strip() or line.startswith("#") or line.startswith("Every rule"):
                continue
            
            # Parse numbered rules: "1. **TITLE.** Description"
            num_match = re.match(r'^(\d+)\.\s+\*\*(.+?)\*\*[.:]\s*(.*)', line)
            if not num_match:
                # Try without trailing punctuation after **
                num_match = re.match(r'^(\d+)\.\s+\*\*(.+?)\*\*\s*(.*)', line)
            if num_match:
                num, title, desc = num_match.groups()
                # Gather continuation lines (indented under the numbered rule)
                # desc might be empty, that's fine
                priority = "critical" if current_section == "absolute" else "high"
                category = current_section
                rules.append({
                    "priority": priority,
                    "category": category,
                    "title": title.strip(),
                    "content": desc.strip() if desc else title.strip(),
                    "source": ".context/tools/tier0-static.md",
                    "source_line": 0,
                })
                continue
            
            # Parse bullet principles: "- **Title** -- description"
            bullet_match = re.match(r'^-\s+\*\*(.+?)\*\*\s*--\s*(.*)', line)
            if bullet_match and current_section == "principle":
                title, desc = bullet_match.groups()
                rules.append({
                    "priority": "high",
                    "category": "principle",
                    "title": title.strip(),
                    "content": desc.strip(),
                    "source": ".context/tools/tier0-static.md",
                    "source_line": 0,
                })
                continue
            
            # Human boundary bullets
            bullet_match2 = re.match(r'^-\s+(.*)', line)
            if bullet_match2 and current_section == "human":
                text = bullet_match2.group(1).strip()
                if len(text) > 15:
                    rules.append({
                        "priority": "high",
                        "category": "human",
                        "title": f"Human boundary: {text[:60]}",
                        "content": text,
                        "source": ".context/tools/tier0-static.md",
                        "source_line": 0,
                    })
                    continue
    
    # ============================================================
    # SUPPLEMENTARY: Coding rules from ARCHITECTURE.md
    # (no principles/roles -- those come from tier0 only)
    # ============================================================
    arch_path = REPO_ROOT / "ARCHITECTURE.md"
    if arch_path.exists():
        arch = arch_path.read_text()
        headings_arch = md_headings(arch)
        
        coding_sections = [
            ("Coding Rules", "coding"),
            ("JSONB Rules", "database"),
        ]
        for title_keyword, category in coding_sections:
            for level, title, lineno in headings_arch:
                if title_keyword.lower() in title.lower():
                    content = extract_between_heading(arch, lineno, headings_arch)
                    if content:
                        rules.append({
                            "priority": "medium",
                            "category": category,
                            "title": title,
                            "content": content[:500],
                            "source": "ARCHITECTURE.md",
                            "source_line": lineno,
                        })
    
    # ============================================================
    # SUPPLEMENTARY: Guardrails from guardrails.md
    # ============================================================
    gl_path = REPO_ROOT / ".context" / "guardrails.md"
    if gl_path.exists():
        gl = gl_path.read_text()
        headings_gl = md_headings(gl)
        
        for level, title, lineno in headings_gl:
            if level <= 3 and any(kw in title.lower() for kw in ["gate", "checklist", "protocol"]):
                content = extract_between_heading(gl, lineno, headings_gl)
                if content and len(content) > 20:
                    rules.append({
                        "priority": "medium",
                        "category": "guardrail",
                        "title": title,
                        "content": content[:500],
                        "source": ".context/guardrails.md",
                        "source_line": lineno,
                    })
    
    # Deduplicate by title (exact match, case-insensitive)
    seen_titles = set()
    deduped = []
    for r in rules:
        key = r["title"].lower().strip()
        if key not in seen_titles:
            seen_titles.add(key)
            deduped.append(r)
    
    return deduped


# ============================================================
# PROMPTS EXTRACTION
# ============================================================

def extract_prompts():
    """Extract all prompt templates from all prompt directories."""
    prompts = []
    prompt_dirs = [
        REPO_ROOT / "prompts",
        REPO_ROOT / "config" / "prompts",
    ]
    
    for pdir in prompt_dirs:
        if not pdir.exists():
            continue
        for f in sorted(pdir.glob("*.md")):
            if f.name == "README.md":
                continue
            text = f.read_text()
            headings = md_headings(text)
            
            # Extract role from first heading or filename
            role = f.stem.replace("_", " ").replace("-", " ").title()
            if headings:
                role = headings[0][1]
            
            # Extract key instructions (first few headings or key phrases)
            key_points = []
            for line in text.split("\n"):
                stripped = line.strip()
                if stripped.startswith("- ") and len(stripped) > 10:
                    key_points.append(stripped.lstrip("- "))
                    if len(key_points) >= 8:
                        break
            
            # Determine role category
            rel_path = str(f.relative_to(REPO_ROOT))
            location = "config/prompts" if "config" in rel_path else "prompts"
            
            prompts.append({
                "name": f.stem,
                "role": role[:100],
                "location": location,
                "file_path": rel_path,
                "size_bytes": f.stat().st_size,
                "key_instructions": "\n".join(key_points)[:500],
                "summary": text[:300].replace("\n", " ").strip(),
            })
    
    # Also check pipeline configs
    pipeline_dir = REPO_ROOT / "governor" / "config" / "pipelines"
    if pipeline_dir.exists():
        for f in sorted(pipeline_dir.glob("*.yaml")):
            text = f.read_text()
            prompts.append({
                "name": f.stem,
                "role": f"Pipeline: {f.stem}",
                "location": "governor/config/pipelines",
                "file_path": str(f.relative_to(REPO_ROOT)),
                "size_bytes": f.stat().st_size,
                "key_instructions": text[:500],
                "summary": text[:200].replace("\n", " ").strip(),
            })
    
    return prompts


# ============================================================
# CONFIGS EXTRACTION
# ============================================================

def extract_configs():
    """Extract all config files with purpose and key fields."""
    configs = []
    config_dirs = [
        REPO_ROOT / "config",
        REPO_ROOT / "governor" / "config",
    ]
    
    for cdir in config_dirs:
        if not cdir.exists():
            continue
        for f in sorted(cdir.glob("*.json")):
            try:
                data = json.loads(f.read_text())
            except json.JSONDecodeError:
                continue
            
            rel_path = str(f.relative_to(REPO_ROOT))
            
            # Determine purpose from structure
            purpose = ""
            key_fields = []
            
            if isinstance(data, dict):
                key_fields = list(data.keys())[:10]
                # Try to extract description
                purpose = data.get("description", data.get("name", data.get("title", "")))
                if isinstance(purpose, dict):
                    purpose = str(purpose)[:100]
            
            if isinstance(data, list):
                key_fields = [f"[array of {len(data)} items]"]
                if data and isinstance(data[0], dict):
                    key_fields += list(data[0].keys())[:8]
                purpose = f"Array of {len(data)} entries"
            
            configs.append({
                "name": f.stem,
                "file_path": rel_path,
                "purpose": str(purpose)[:200] if purpose else "(no description)",
                "key_fields": ", ".join(str(k) for k in key_fields),
                "entry_count": len(data) if isinstance(data, list) else len(data.keys()),
                "size_bytes": f.stat().st_size,
            })
    
    # Deduplicate (config/ and governor/config/ may have same-named files)
    seen = set()
    deduped = []
    for c in configs:
        if c["name"] not in seen:
            seen.add(c["name"])
            deduped.append(c)
    
    return deduped


# ============================================================
# DOCS EXTRACTION
# ============================================================

def extract_docs():
    """Extract all documentation sections from all markdown files."""
    sections = []
    
    # Find all markdown files
    md_files = []
    for pattern in ["*.md", "docs/**/*.md", "config/**/*.md", "contracts/**/*.md", 
                     "agents/**/*.md", "research/**/*.md", "governor/**/*.md"]:
        md_files.extend(REPO_ROOT.glob(pattern))
    
    # Remove duplicates
    md_files = sorted(set(f for f in md_files if ".git" not in str(f)))
    
    for f in md_files:
        rel_path = str(f.relative_to(REPO_ROOT))
        text = f.read_text(errors="replace")
        headings = md_headings(text)
        
        if not headings:
            # File has no headings, index as single section
            sections.append({
                "title": f.stem,
                "file_path": rel_path,
                "level": 0,
                "line_start": 1,
                "line_end": text.count("\n") + 1,
                "summary": text[:200].replace("\n", " ").strip(),
                "content": text[:2000],
            })
            continue
        
        for i, (level, title, lineno) in enumerate(headings):
            # Get end line
            if i + 1 < len(headings):
                end_line = headings[i+1][2] - 1
            else:
                end_line = text.count("\n") + 1
            
            content = section_text(text, lineno, end_line)
            
            # Only index sections with actual content
            if len(content.strip()) > 20:
                summary = content.replace("\n", " ").strip()[:200]
                sections.append({
                    "title": title,
                    "file_path": rel_path,
                    "level": level,
                    "line_start": lineno,
                    "line_end": end_line,
                    "summary": summary,
                    "content": content[:2000],
                })
    
    return sections


# ============================================================
# BUILD THE DATABASE
# ============================================================

def build_db():
    """Build the knowledge.db SQLite database."""
    if DB_PATH.exists():
        DB_PATH.unlink()
    
    conn = sqlite3.connect(str(DB_PATH))
    c = conn.cursor()
    
    # Create tables
    c.execute("""CREATE TABLE rules (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        priority TEXT NOT NULL,    -- critical, high, medium
        category TEXT NOT NULL,    -- hardcode, architecture, dashboard, vault, etc
        title TEXT NOT NULL,
        content TEXT NOT NULL,
        source TEXT NOT NULL,
        source_line INTEGER
    )""")
    
    c.execute("""CREATE TABLE prompts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        role TEXT NOT NULL,
        location TEXT NOT NULL,
        file_path TEXT NOT NULL UNIQUE,
        size_bytes INTEGER,
        key_instructions TEXT,
        summary TEXT
    )""")
    
    c.execute("""CREATE TABLE configs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        file_path TEXT NOT NULL,
        purpose TEXT,
        key_fields TEXT,
        entry_count INTEGER,
        size_bytes INTEGER
    )""")
    
    c.execute("""CREATE TABLE docs (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        file_path TEXT NOT NULL,
        level INTEGER,
        line_start INTEGER,
        line_end INTEGER,
        summary TEXT,
        content TEXT
    )""")
    
    # Create indexes
    c.execute("CREATE INDEX idx_rules_priority ON rules(priority)")
    c.execute("CREATE INDEX idx_rules_category ON rules(category)")
    c.execute("CREATE INDEX idx_prompts_name ON prompts(name)")
    c.execute("CREATE INDEX idx_configs_name ON configs(name)")
    c.execute("CREATE INDEX idx_docs_file ON docs(file_path)")
    c.execute("CREATE INDEX idx_docs_title ON docs(title)")
    
    # Insert rules
    print("[knowledge] Extracting rules...")
    rules = extract_rules()
    for r in rules:
        c.execute("INSERT INTO rules (priority, category, title, content, source, source_line) VALUES (?, ?, ?, ?, ?, ?)",
                  (r["priority"], r["category"], r["title"], r["content"], r["source"], r["source_line"]))
    print(f"[knowledge] Rules: {len(rules)} entries")
    
    # Insert prompts
    print("[knowledge] Extracting prompts...")
    prompts = extract_prompts()
    for p in prompts:
        c.execute("INSERT INTO prompts (name, role, location, file_path, size_bytes, key_instructions, summary) VALUES (?, ?, ?, ?, ?, ?, ?)",
                  (p["name"], p["role"], p["location"], p["file_path"], p["size_bytes"], p["key_instructions"], p["summary"]))
    print(f"[knowledge] Prompts: {len(prompts)} entries")
    
    # Insert configs
    print("[knowledge] Extracting configs...")
    configs = extract_configs()
    for cf in configs:
        c.execute("INSERT INTO configs (name, file_path, purpose, key_fields, entry_count, size_bytes) VALUES (?, ?, ?, ?, ?, ?)",
                  (cf["name"], cf["file_path"], cf["purpose"], cf["key_fields"], cf["entry_count"], cf["size_bytes"]))
    print(f"[knowledge] Configs: {len(configs)} entries")
    
    # Insert docs
    print("[knowledge] Extracting docs...")
    docs = extract_docs()
    for d in docs:
        c.execute("INSERT INTO docs (title, file_path, level, line_start, line_end, summary, content) VALUES (?, ?, ?, ?, ?, ?, ?)",
                  (d["title"], d["file_path"], d["level"], d["line_start"], d["line_end"], d["summary"], d["content"]))
    print(f"[knowledge] Docs: {len(docs)} sections")
    
    # Metadata
    c.execute("""CREATE TABLE meta (
        key TEXT PRIMARY KEY,
        value TEXT
    )""")
    
    import subprocess
    commit = subprocess.run(["git", "-C", str(REPO_ROOT), "rev-parse", "--short", "HEAD"], 
                           capture_output=True, text=True).stdout.strip()
    
    from datetime import datetime, timezone
    c.execute("INSERT INTO meta VALUES ('generated', ?)", (datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),))
    c.execute("INSERT INTO meta VALUES ('commit', ?)", (commit,))
    c.execute("INSERT INTO meta VALUES ('rules_count', ?)", (str(len(rules)),))
    c.execute("INSERT INTO meta VALUES ('prompts_count', ?)", (str(len(prompts)),))
    c.execute("INSERT INTO meta VALUES ('configs_count', ?)", (str(len(configs)),))
    c.execute("INSERT INTO meta VALUES ('docs_count', ?)", (str(len(docs)),))
    
    conn.commit()
    
    db_size = DB_PATH.stat().st_size
    print(f"\n[knowledge] Built {DB_PATH}")
    print(f"[knowledge] Size: {db_size / 1024:.0f} KB")
    print(f"[knowledge] Total: {len(rules)} rules, {len(prompts)} prompts, {len(configs)} configs, {len(docs)} doc sections")
    
    conn.close()


if __name__ == "__main__":
    build_db()
