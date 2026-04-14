#!/usr/bin/env python3
"""
tier0.py - Extract Tier 0 rules from knowledge.db.

Outputs the ~500-token "impossible to miss" rules block for boot.md.
These are the rules that MUST be in every agent's context, not behind a query.

Usage: python3 .context/tools/tier0.py [repo_root]
"""

import sqlite3
import sys
from pathlib import Path

REPO_ROOT = Path(sys.argv[1]) if len(sys.argv) > 1 else Path(__file__).resolve().parent.parent.parent
DB_PATH = REPO_ROOT / ".context" / "knowledge.db"

def build_tier0():
    if not DB_PATH.exists():
        print("# Tier 0 not available - run build-knowledge-db.py first")
        return

    conn = sqlite3.connect(str(DB_PATH))
    c = conn.cursor()

    lines = []
    lines.append("# TIER 0: NON-NEGOTIABLE RULES — READ FIRST OR FAIL")
    lines.append("Every session, every agent, every task. These are not suggestions.")
    lines.append("")

    # --- SECTION 1: The 5 Absolute Rules ---
    lines.append("### Absolute Rules")
    lines.append("These apply to EVERY task, EVERY agent, EVERY session:")

    # Curated list of the rules that actually prevent the behavioral problems
    tier0_titles = [
        "NEVER Hardcode Anything",
        "NO Type 1 Errors",
        "The Dashboard is SACRED",
        "CRITICAL: Dashboard is READ-ONLY",
        "CRITICAL: No Webhooks",
        "Never apply migrations directly",
    ]

    for title in tier0_titles:
        c.execute("SELECT content FROM rules WHERE title = ?", (title,))
        row = c.fetchone()
        if row:
            content = row[0]
            # Strip markdown headers and emoji
            content = content.replace("⛔", "")
            # Extract just the first meaningful paragraph (skip header lines)
            paragraphs = [p.strip() for p in content.split("\n\n") if p.strip()]
            meat = ""
            for p in paragraphs:
                clean = p.lstrip("#").strip()
                # Skip if paragraph is essentially just the title repeated
                clean_no_markdown = clean.replace("**", "").replace("*", "").replace("`", "").strip()
                title_clean = title.replace("**", "").replace("*", "").strip()
                if clean_no_markdown.lower().startswith(title_clean.lower()):
                    # Strip the title prefix from content
                    clean = clean_no_markdown[len(title_clean):].strip().lstrip(":").strip()
                    if len(clean) < 20:
                        continue
                if len(clean) > 20 and not p.startswith("```") and not p.startswith("|"):
                    meat = clean
                    break
            if not meat:
                # Fallback: use second paragraph if first is just the title
                for p in paragraphs[1:]:
                    clean = p.lstrip("#").strip()
                    if len(clean) > 15 and not p.startswith("```"):
                        meat = clean
                        break
            if meat:
                lines.append(f"- **{title}**: {meat[:150]}")
            else:
                lines.append(f"- **{title}**")

    # --- SECTION 2: Philosophy in 4 bullets ---
    lines.append("")
    lines.append("### Core Philosophy (this is WHY we build this way)")

    philosophy_titles = [
        ("Zero Vendor Lock-In", "Can we replace [X] in one day with zero code changes?"),
        ("Exit Ready", "Pack up, hand over to anyone, survive anywhere"),
        ("If It Can't Be Undone, It Can't Be Done", "Every change reversible, rollback plan before implementation"),
        ("THE CORE RULE", "Interface -> Config -> Concrete Implementation. NOT: Hardcoded -> Vendor -> Broken"),
    ]

    for title, short in philosophy_titles:
        c.execute("SELECT content FROM rules WHERE title = ?", (title,))
        row = c.fetchone()
        if row:
            content = row[0].replace("⛔", "")
            paragraphs = [p.strip() for p in content.split("\n\n") if p.strip() and len(p.strip()) > 30]
            # Skip the title paragraph and "What it means:" prefix lines
            meat = ""
            for p in paragraphs:
                clean = p.lstrip("#").strip()
                clean = clean.replace("**What it means:**", "").strip()
                clean = clean.replace("**How we ensure it:**", "").strip()
                if len(clean) > 20 and clean.lower() != title.lower() and not clean.startswith("```") and not clean.startswith("|") and not clean.startswith(">"):
                    meat = clean[:150]
                    break
            if meat:
                lines.append(f"- **{title}**: {meat}")
            else:
                lines.append(f"- **{title}**: {short}")

    # --- SECTION 3: Before You Code checklist ---
    lines.append("")
    lines.append("### Before You Code (mandatory checklist)")
    lines.append("- [ ] Read relevant docs in knowledge.db FIRST, not after")
    lines.append("- [ ] Search knowledge.db for existing rules/patterns before inventing new ones")
    lines.append("- [ ] No hardcoding - use config JSON in governor/config/")
    lines.append("- [ ] Credentials: vault ONLY (sqlite3 .context/knowledge.db \"SELECT content FROM rules WHERE title LIKE '%Supabase%'\")")
    lines.append("- [ ] No .env files, no sudo systemctl, no journalctl -u governor")
    lines.append("- [ ] If dashboard looks wrong, fix the Go code, not the dashboard")

    # --- SECTION 4: What NOT to do ---
    lines.append("")
    lines.append("### Time Wasters (don't do these)")
    lines.append("- Looking for .env files (don't exist)")
    lines.append("- Using sudo systemctl (it's a user service: systemctl --user)")
    lines.append("- Wrong log command (use: journalctl --user -u vibepilot-governor)")
    lines.append("- Guessing instead of checking what exists first")
    lines.append("- Touching dashboard code (problem is in Go, not dashboard)")
    lines.append("- Using webhooks (we use Supabase Live realtime)")

    # --- SECTION 5: Human boundaries ---
    lines.append("")
    lines.append("### Human Role")
    lines.append("- Human reviews code, merges code, maintains the system")
    lines.append("- Human does NOT debug agent code or write agent code directly")
    lines.append("- Respect human time: batch questions, not one-at-a-time")

    # --- SECTION 6: Where to find more ---
    lines.append("")
    lines.append("### Going Deeper")
    lines.append("When you need more than Tier 0:")
    lines.append("- Architecture rules: sqlite3 .context/knowledge.db \"SELECT title,content FROM rules WHERE priority='high'\"")
    lines.append("- All prompts: sqlite3 .context/knowledge.db \"SELECT name,role FROM prompts\"")
    lines.append("- Search docs: sqlite3 .context/knowledge.db \"SELECT title,file_path FROM docs WHERE title LIKE '%<topic>%'\"")
    lines.append("- Full code map: cat .context/map.md")

    conn.close()

    output = "\n".join(lines)
    print(output)

    # Rough token estimate
    word_count = len(output.split())
    est_tokens = int(word_count * 1.3)
    print(f"\n# Tier 0 estimated: ~{est_tokens} tokens ({word_count} words)", file=sys.stderr)


if __name__ == "__main__":
    build_tier0()
