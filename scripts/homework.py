#!/usr/bin/env python3
"""Pre-modification homework gate for VibePilot.

Runs the 5-step orientation protocol and writes a timestamped homework log.
Agents MUST run this before modifying any VibePilot infrastructure code.

Usage:
    python3 scripts/homework.py "description of planned change"

Output:
    Writes .homework/<timestamp>.md with blast radius, callers, callees,
    relevant decisions, rules, and contradictions.

Exit 0 = homework complete, safe to proceed.
Exit 1 = homework found issues that need attention before proceeding.
"""

import json
import os
import re
import sys
from datetime import datetime, timezone

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_DIR = os.path.join(SCRIPT_DIR, '..')
HOMEWORK_DIR = os.path.join(REPO_DIR, '.homework')

import psycopg


def get_conn():
    return psycopg.connect("dbname=vibepilot")


def run_homework(task_description: str) -> dict:
    """Execute the 5-step homework protocol and return structured results."""
    pg = get_conn()
    results = {
        'task': task_description,
        'timestamp': datetime.now(timezone.utc).isoformat(),
        'steps': {},
        'warnings': [],
        'safe_to_proceed': True,
    }

    try:
        # Step 1: kb_context_pack
        with pg.cursor() as cur:
            cur.execute("SELECT section, content FROM kb_context_pack(%s, p_limit=>15)", (task_description,))
            rows = cur.fetchall()
            context = {}
            for section, content in rows:
                try:
                    context[section] = json.loads(content) if content else []
                except (json.JSONDecodeError, TypeError):
                    context[section] = content or []
            results['steps']['context_pack'] = {
                'sections': list(context.keys()),
                'symbols_count': len(context.get('symbols', []) or []),
                'docs_count': len(context.get('docs', []) or []),
                'decisions_count': len(context.get('decisions', []) or []),
                'rules_count': len(context.get('non_negotiable_rules', []) or []),
            }
            # Check for blocking rules
            blocking_rules = []
            informational_rules = []
            for rule in context.get('non_negotiable_rules', []) or []:
                if isinstance(rule, dict):
                    rule_text = rule.get('rule', rule.get('title', rule.get('name', str(rule))))
                elif isinstance(rule, str):
                    rule_text = rule
                else:
                    continue
                # Informational rules that agents should see but don't block
                info_keywords = ['push to github', 'commit and push', 'human boundary', 'never guess',
                                 'never hardcode', 'never hunt', 'never take shortcuts', 'roles are defined']
                if any(kw in rule_text.lower() for kw in info_keywords):
                    informational_rules.append(rule_text)
                else:
                    blocking_rules.append(rule_text)

            if blocking_rules:
                results['warnings'].extend([f"BLOCKING RULE: {r}" for r in blocking_rules])
        # Step 1b: Also query decisions directly (kb_context_pack omits status field)
        with pg.cursor() as cur:
            cur.execute("""
                SELECT name, title, status, COALESCE(NULLIF(substring(summary, 1, 500), ''), substring(content, 1, 500), '') as desc
                FROM kb_knowledge_items
                WHERE item_type = 'decision' AND status IS NOT NULL
                ORDER BY name
            """)
            dec_cols = [desc[0] for desc in cur.description]
            all_decisions = [dict(zip(dec_cols, row)) for row in cur.fetchall()]

        # Check ALL decisions for potential conflicts with the planned change
        import re as _re
        stop_words = {'the', 'a', 'an', 'and', 'or', 'to', 'for', 'in', 'of', 'is', 'it',
                      'with', 'using', 'from', 'by', 'on', 'at', 'be', 'has', 'that', 'this',
                      'we', 'our', 'can', 'will', 'should', 'would', 'do', 'not', 'but', 'all',
                      'dec', 'code', 'github', 'management'}
        plan_words = set(_re.findall(r'[a-z]+', task_description.lower())) - stop_words

        relevant_decisions = []
        for dec in all_decisions:
            title = dec.get('title', '') or ''
            desc = dec.get('desc', '') or ''
            dec_text = (title + ' ' + desc).lower()
            status = dec.get('status', '')

            dec_words = set(_re.findall(r'[a-z]+', dec_text)) - stop_words
            overlap = plan_words & dec_words

            if len(overlap) >= 2:
                relevant_decisions.append({
                    'title': title,
                    'status': status,
                    'overlap': list(overlap)[:5],
                })
                if status == 'rejected':
                    results['warnings'].append(
                        f"REJECTED DECISION '{title}' overlaps with plan (shared: {', '.join(list(overlap)[:5])}). Verify you're not re-implementing a rejected approach."
                    )
                elif status == 'adopted':
                    results['warnings'].append(
                        f"APPROVED DECISION '{title}' is relevant to your plan (shared: {', '.join(list(overlap)[:5])}). Ensure your change is consistent with this decision."
                    )

        results['steps']['decisions_check'] = {
            'total_decisions': len(all_decisions),
            'relevant_count': len(relevant_decisions),
            'relevant': relevant_decisions[:10],
        }

        # Step 2: Extract key terms for blast radius
        # Use the top symbols from context_pack
        symbols = context.get('symbols', [])
        top_symbols = []
        for sym in symbols[:5]:
            if isinstance(sym, dict):
                qname = sym.get('qualified_name', sym.get('name', ''))
                if qname:
                    top_symbols.append(qname)

        # Step 3: Blast radius for top symbols
        blast_results = []
        for qname in top_symbols[:3]:  # Top 3 symbols only to keep tokens low
            with pg.cursor() as cur:
                cur.execute("SELECT * FROM kb_get_blast_radius(%s)", (qname,))
                cols = [desc[0] for desc in cur.description]
                rows = [dict(zip(cols, row)) for row in cur.fetchall()]
                if rows:
                    blast_results.append({
                        'symbol': qname,
                        'reach_count': len(rows),
                        'top_callers': [r.get('caller', r.get('source', str(r))) for r in rows[:5]],
                    })
        results['steps']['blast_radius'] = blast_results
        if blast_results:
            max_reach = max(b['reach_count'] for b in blast_results)
            if max_reach > 20:
                results['warnings'].append(
                    f"HIGH BLAST RADIUS: {max_reach} symbols affected. Review impact before proceeding."
                )

        # Step 4: Callers and callees for the primary symbol
        if top_symbols:
            primary = top_symbols[0]
            with pg.cursor() as cur:
                cur.execute("SELECT * FROM kb_get_callers(%s, result_limit=>20)", (primary,))
                cols = [desc[0] for desc in cur.description]
                callers = [dict(zip(cols, row)) for row in cur.fetchall()]
            with pg.cursor() as cur:
                cur.execute("SELECT * FROM kb_get_callees(%s, result_limit=>20)", (primary,))
                cols = [desc[0] for desc in cur.description]
                callees = [dict(zip(cols, row)) for row in cur.fetchall()]
            results['steps']['call_graph'] = {
                'primary_symbol': primary,
                'callers_count': len(callers),
                'callees_count': len(callees),
                'top_callers': [c.get('caller', c.get('source_qualified', str(c))) for c in callers[:5]],
                'top_callees': [c.get('callee', c.get('target_qualified', str(c))) for c in callees[:5]],
            }

        # Step 5: Check for open contradictions
        with pg.cursor() as cur:
            cur.execute("SELECT * FROM kb_get_open_contradictions()")
            cols = [desc[0] for desc in cur.description]
            contradictions = [dict(zip(cols, row)) for row in cur.fetchall()]
        results['steps']['contradictions'] = {
            'open_count': len(contradictions),
            'items': contradictions[:5],
        }
        if contradictions:
            results['warnings'].append(
                f"{len(contradictions)} OPEN CONTRADICTIONS detected. Resolve before making changes."
            )
            results['safe_to_proceed'] = False

        # Final safety: only block on real issues
        real_issues = [w for w in results['warnings'] if w.startswith(('BLOCKING RULE', 'OPEN CONTRADICTION', 'HIGH BLAST RADIUS', 'REJECTED DECISION'))]
        if real_issues:
            results['safe_to_proceed'] = False

    finally:
        pg.close()

    return results


def write_homework_log(results: dict) -> str:
    """Write a human-readable homework log to .homework/ directory."""
    os.makedirs(HOMEWORK_DIR, exist_ok=True)

    ts = datetime.now(timezone.utc).strftime('%Y%m%d-%H%M%S')
    # Create a safe filename from the task description
    safe_name = results['task'][:40].replace(' ', '-').replace('/', '-').lower()
    safe_name = ''.join(c for c in safe_name if c.isalnum() or c in '-_')
    filename = f"{ts}-{safe_name}.md"
    filepath = os.path.join(HOMEWORK_DIR, filename)

    lines = [
        f"# Homework Log: {results['task']}",
        f"",
        f"**Time**: {results['timestamp']}",
        f"**Safe to proceed**: {'YES' if results['safe_to_proceed'] else 'NO -- review warnings'}",
        f"",
    ]

    if results['warnings']:
        lines.append("## Warnings")
        for w in results['warnings']:
            lines.append(f"- {w}")
        lines.append("")

    cp = results['steps'].get('context_pack', {})
    lines.extend([
        "## Context Pack Summary",
        f"- Symbols: {cp.get('symbols_count', 0)}",
        f"- Docs: {cp.get('docs_count', 0)}",
        f"- Decisions: {cp.get('decisions_count', 0)}",
        f"- Rules: {cp.get('rules_count', 0)}",
        "",
    ])

    blast = results['steps'].get('blast_radius', [])
    if blast:
        lines.append("## Blast Radius")
        for b in blast:
            lines.append(f"- **{b['symbol']}**: {b['reach_count']} symbols affected")
            for c in b.get('top_callers', [])[:3]:
                lines.append(f"  - <- {c}")
        lines.append("")

    cg = results['steps'].get('call_graph', {})
    if cg:
        lines.extend([
            "## Call Graph",
            f"- Primary: {cg.get('primary_symbol', 'N/A')}",
            f"- Callers (upstream): {cg.get('callers_count', 0)}",
            f"- Callees (downstream): {cg.get('callees_count', 0)}",
            "",
        ])
        for c in cg.get('top_callers', [])[:3]:
            lines.append(f"  - <- {c}")
        for c in cg.get('top_callees', [])[:3]:
            lines.append(f"  - -> {c}")
        lines.append("")

    contra = results['steps'].get('contradictions', {})
    lines.extend([
        "## Contradictions",
        f"- Open: {contra.get('open_count', 0)}",
        "",
    ])

    lines.extend([
        "---",
        f"*Generated by homework.py*",
    ])

    with open(filepath, 'w') as f:
        f.write('\n'.join(lines))

    return filepath


if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: python3 scripts/homework.py \"description of planned change\"")
        print("  Runs the 5-step orientation protocol before modifying VibePilot.")
        sys.exit(1)

    task = ' '.join(sys.argv[1:])
    print(f"Running homework for: {task}", flush=True)
    print("=" * 60, flush=True)

    results = run_homework(task)

    # Print summary
    cp = results['steps'].get('context_pack', {})
    print(f"  Context: {cp.get('symbols_count', 0)} symbols, {cp.get('docs_count', 0)} docs, {cp.get('decisions_count', 0)} decisions, {cp.get('rules_count', 0)} rules", flush=True)

    blast = results['steps'].get('blast_radius', [])
    if blast:
        for b in blast:
            print(f"  Blast: {b['symbol']} -> {b['reach_count']} affected", flush=True)

    cg = results['steps'].get('call_graph', {})
    if cg:
        print(f"  Calls: {cg.get('callers_count', 0)} callers, {cg.get('callees_count', 0)} callees of {cg.get('primary_symbol', 'N/A')}", flush=True)

    contra = results['steps'].get('contradictions', {})
    print(f"  Contradictions: {contra.get('open_count', 0)} open", flush=True)

    if results['warnings']:
        print(f"\n  WARNINGS ({len(results['warnings'])}):", flush=True)
        for w in results['warnings']:
            print(f"    - {w}", flush=True)

    # Write log
    log_path = write_homework_log(results)
    print(f"\n  Log: {log_path}", flush=True)
    print(f"  Safe to proceed: {'YES' if results['safe_to_proceed'] else 'NO -- review warnings above'}", flush=True)

    sys.exit(0 if results['safe_to_proceed'] else 1)
