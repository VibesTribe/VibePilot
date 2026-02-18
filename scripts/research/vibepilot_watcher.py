#!/usr/bin/env python3
"""
VibePilot Raindrop Watcher

Monitors the VIBEPILOT collection for new bookmarks.
- Checks for new items since last run
- Researches new bookmarks in VibePilot context
- Generates reports
- Can run as cron job for automated monitoring

Usage:
    # Check once (for cron jobs)
    python scripts/research/vibepilot_watcher.py
    
    # Force full research of all bookmarks
    python scripts/research/vibepilot_watcher.py --full
    
    # Check specific collection
    python scripts/research/vibepilot_watcher.py --collection-id 67118576
"""

import os
import sys
import json
import argparse
from datetime import datetime, timedelta
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from dotenv import load_dotenv
load_dotenv()

# Configuration
VIBEPILOT_COLLECTION_ID = 67118576  # Your VIBEPILOT folder
STATE_FILE = Path(__file__).parent / '.vibepilot_state.json'


def load_state():
    """Load last check state."""
    if STATE_FILE.exists():
        with open(STATE_FILE) as f:
            return json.load(f)
    return {'last_check': None, 'processed_ids': []}


def save_state(state):
    """Save check state."""
    with open(STATE_FILE, 'w') as f:
        json.dump(state, f, indent=2)


def get_raindrop_token():
    """Get access token from file."""
    token_file = Path(__file__).parent / '.raindrop_token.json'
    if token_file.exists():
        with open(token_file) as f:
            data = json.load(f)
            return data.get('access_token')
    return None


def get_new_bookmarks(collection_id, since_date=None):
    """Fetch bookmarks from collection, optionally filtering by date."""
    import requests
    
    token = get_raindrop_token()
    if not token:
        print("Error: No Raindrop token found. Run raindrop_get_token.py first.")
        return []
    
    headers = {'Authorization': f'Bearer {token}'}
    bookmarks = []
    page = 0
    
    while True:
        resp = requests.get(
            f'https://api.raindrop.io/rest/v1/raindrops/{collection_id}',
            headers=headers,
            params={'page': page, 'perpage': 50}
        )
        
        if not resp.ok:
            print(f"Error fetching bookmarks: {resp.status_code}")
            break
        
        data = resp.json()
        items = data.get('items', [])
        
        if not items:
            break
        
        for item in items:
            created = item.get('created')
            if since_date and created:
                item_date = datetime.fromisoformat(created.replace('Z', '+00:00'))
                if item_date >= since_date:
                    bookmarks.append(item)
            else:
                bookmarks.append(item)
        
        if len(items) < 50:
            break
        page += 1
    
    return bookmarks


def analyze_relevance(bookmark):
    """Quick relevance analysis for VibePilot context."""
    title = bookmark.get('title', '').lower()
    excerpt = bookmark.get('excerpt', '').lower()
    tags = [t.lower() for t in bookmark.get('tags', [])]
    text = f"{title} {excerpt} {' '.join(tags)}"
    
    # VibePilot keywords
    keywords = {
        'models_platforms': ['llm', 'model', 'ai', 'gemini', 'claude', 'chatgpt', 'kimi', 'deepseek', 'api', 'free tier'],
        'architecture': ['agent', 'automation', 'orchestration', 'pipeline', 'workflow', 'multi-agent', 'vibepilot'],
        'courier_browser': ['browser', 'automation', 'playwright', 'scraping', 'courier', 'openclaw'],
        'cost_optimization': ['pricing', 'cost', 'rate limit', 'token', 'efficiency', 'free'],
        'infrastructure': ['supabase', 'github', 'vercel', 'deployment', 'database']
    }
    
    scores = {}
    all_matches = []
    
    for cat, words in keywords.items():
        matches = [w for w in words if w in text]
        scores[cat] = len(matches)
        all_matches.extend(matches)
    
    total_score = min(sum(scores.values()) / 1.5, 10)
    primary = max(scores, key=scores.get) if scores else 'general'
    
    return {
        'score': round(total_score, 1),
        'primary_category': primary,
        'matches': list(set(all_matches))[:10]
    }


def generate_quick_report(bookmarks):
    """Generate a quick markdown report."""
    date_str = datetime.now().strftime('%Y-%m-%d %H:%M')
    
    if not bookmarks:
        return f"""# VibePilot Raindrop Digest
**Date:** {date_str}

No new bookmarks in VIBEPILOT collection.

---
*Next check in ~12 hours*
"""
    
    # Sort by relevance
    analyzed = []
    for b in bookmarks:
        rel = analyze_relevance(b)
        analyzed.append({**b, 'relevance': rel})
    
    analyzed.sort(key=lambda x: x['relevance']['score'], reverse=True)
    
    high = [b for b in analyzed if b['relevance']['score'] >= 6]
    med = [b for b in analyzed if 3 <= b['relevance']['score'] < 6]
    low = [b for b in analyzed if b['relevance']['score'] < 3]
    
    report = f"""# VibePilot Raindrop Digest
**Date:** {date_str}  
**New Bookmarks:** {len(bookmarks)}

## Summary
| Priority | Count |
|----------|-------|
| High (6+) | {len(high)} |
| Medium (3-5) | {len(med)} |
| Low (<3) | {len(low)} |

## New Bookmarks

"""
    
    for b in analyzed:
        rel = b['relevance']
        badge = "HIGH" if rel['score'] >= 6 else "MED" if rel['score'] >= 3 else "LOW"
        
        report += f"""### {b['title']}
**{badge}** (Score: {rel['score']}) | {rel['primary_category'].replace('_', ' ').title()}

**URL:** {b['link']}

**Why relevant:** {', '.join(rel['matches']) or 'General interest'}

> {b.get('excerpt', 'No excerpt')[:150]}...

---

"""
    
    report += "*Digest ends - next check in ~12 hours*"
    return report


def main():
    parser = argparse.ArgumentParser(description='VibePilot Raindrop Watcher')
    parser.add_argument('--full', action='store_true', help='Research all bookmarks, not just new')
    parser.add_argument('--collection-id', type=int, default=VIBEPILOT_COLLECTION_ID, help='Collection ID to watch')
    parser.add_argument('--output', help='Output file path')
    
    args = parser.parse_args()
    
    state = load_state()
    
    # Determine date filter
    if args.full:
        since_date = None
        print("Mode: Full collection research")
    else:
        if state['last_check']:
            since_date = datetime.fromisoformat(state['last_check'])
            print(f"Checking for bookmarks since: {since_date}")
        else:
            since_date = datetime.now() - timedelta(days=7)
            print(f"First run - checking last 7 days")
    
    # Fetch bookmarks
    print(f"Fetching from collection {args.collection_id}...")
    bookmarks = get_new_bookmarks(args.collection_id, since_date)
    
    print(f"Found {len(bookmarks)} bookmarks to process")
    
    # Generate report
    report = generate_quick_report(bookmarks)
    
    # Save report
    output_dir = Path('docs/research')
    output_dir.mkdir(parents=True, exist_ok=True)
    
    if args.output:
        output_file = Path(args.output)
    else:
        date_str = datetime.now().strftime('%Y%m%d_%H%M')
        output_file = output_dir / f"vibepilot-digest-{date_str}.md"
    
    with open(output_file, 'w') as f:
        f.write(report)
    
    print(f"\nReport saved: {output_file}")
    
    # Update state
    state['last_check'] = datetime.now().isoformat()
    state['processed_ids'] = state.get('processed_ids', []) + [b['_id'] for b in bookmarks]
    save_state(state)
    
    # Summary
    if bookmarks:
        scores = [analyze_relevance(b)['score'] for b in bookmarks]
        print(f"\nProcessed {len(bookmarks)} bookmarks:")
        print(f"  High (6+): {sum(1 for s in scores if s >= 6)}")
        print(f"  Medium (3-5): {sum(1 for s in scores if 3 <= s < 6)}")
        print(f"  Low (<3): {sum(1 for s in scores if s < 3)}")
    else:
        print("\nNo new bookmarks to process.")


if __name__ == '__main__':
    main()
