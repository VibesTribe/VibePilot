#!/usr/bin/env python3
"""
Raindrop Research Integration for VibePilot

Fetches bookmarks from Raindrop collections, researches each in context
of VibePilot, and outputs findings for review.

Usage:
    python scripts/research/raindrop_researcher.py --collection vibeflow --days 7
    python scripts/research/raindrop_researcher.py --collection vibepilot --watch  # Check for new items
"""

import os
import sys
import json
import argparse
import requests
from datetime import datetime, timedelta
from typing import List, Dict, Optional
from urllib.parse import urlencode
from pathlib import Path

# Add project root to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from dotenv import load_dotenv
load_dotenv()

# Raindrop API Configuration
RAINDROP_CLIENT_ID = "6995298e703bf1e11e73890a"
RAINDROP_CLIENT_SECRET = "4b698b0b-6ce6-469d-aa4f-aff29a75f0ec"
RAINDROP_API_BASE = "https://api.raindrop.io/rest/v1"


class RaindropResearcher:
    """Researches Raindrop bookmarks in context of VibePilot."""
    
    def __init__(self):
        self.access_token = None
        self.token_expires = None
        self._authenticate()
    
    def _authenticate(self):
        """Authenticate with Raindrop API using client credentials."""
        auth_url = f"{RAINDROP_API_BASE}/auth/token"
        
        # Raindrop supports public collection access without auth
        # For private collections, OAuth2 flow is required
        print("Setting up Raindrop access...")
        
        # Try to get existing token or create public access
        self.access_token = os.getenv('RAINDROP_ACCESS_TOKEN')
        
        if not self.access_token:
            print("Note: No access token found. Using public collection access.")
            print("  - Make collections public in Raindrop (Share → Make Public)")
            print("  - For private collections, run: python scripts/research/raindrop_auth.py")
    
    def get_collection(self, title: str) -> Optional[Dict]:
        """Find collection by title."""
        try:
            headers = {}
            if self.access_token:
                headers['Authorization'] = f'Bearer {self.access_token}'
            
            response = requests.get(
                f"{RAINDROP_API_BASE}/collections",
                headers=headers
            )
            response.raise_for_status()
            
            collections = response.json().get('items', [])
            
            for col in collections:
                if col.get('title', '').lower() == title.lower():
                    return col
            
            return None
            
        except Exception as e:
            print(f"Error fetching collections: {e}")
            return None
    
    def get_bookmarks(self, collection_id: int, days: int = 7) -> List[Dict]:
        """Fetch bookmarks from collection, filtered by date."""
        try:
            headers = {}
            if self.access_token:
                headers['Authorization'] = f'Bearer {self.access_token}'
            
            cutoff_date = datetime.now() - timedelta(days=days)
            bookmarks = []
            page = 0
            
            while True:
                params = {'page': page, 'perpage': 50}
                
                response = requests.get(
                    f"{RAINDROP_API_BASE}/raindrops/{collection_id}",
                    headers=headers,
                    params=params
                )
                response.raise_for_status()
                
                data = response.json()
                items = data.get('items', [])
                
                if not items:
                    break
                
                for item in items:
                    created = item.get('created')
                    if created:
                        item_date = datetime.fromisoformat(created.replace('Z', '+00:00'))
                        if item_date >= cutoff_date:
                            bookmarks.append(item)
                
                # If we got less than perpage, we're done
                if len(items) < 50:
                    break
                
                page += 1
            
            return bookmarks
            
        except Exception as e:
            print(f"Error fetching bookmarks: {e}")
            return []
    
    def research_bookmark(self, bookmark: Dict) -> Dict:
        """
        Research a bookmark in context of VibePilot.
        Returns structured findings.
        """
        url = bookmark.get('link', '')
        title = bookmark.get('title', 'Untitled')
        excerpt = bookmark.get('excerpt', '')
        tags = bookmark.get('tags', [])
        
        print(f"\nResearching: {title}")
        print(f"URL: {url}")
        
        # Fetch content
        content = self._fetch_url(url)
        
        # Analyze in context of VibePilot
        findings = {
            'title': title,
            'url': url,
            'raindrop_id': bookmark.get('_id'),
            'tags': tags,
            'excerpt': excerpt,
            'date_added': bookmark.get('created'),
            'content_preview': content[:2000] if content else None,
            'vibepilot_relevance': self._analyze_relevance(title, excerpt, content, tags),
            'recommendations': []
        }
        
        return findings
    
    def _fetch_url(self, url: str) -> str:
        """Fetch content from URL."""
        try:
            response = requests.get(url, timeout=30, headers={
                'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
            })
            response.raise_for_status()
            return response.text
        except Exception as e:
            print(f"  Could not fetch content: {e}")
            return ""
    
    def _analyze_relevance(self, title: str, excerpt: str, content: str, tags: List[str]) -> Dict:
        """Analyze bookmark relevance to VibePilot."""
        # Keywords that indicate VibePilot relevance
        vp_keywords = [
            'ai', 'llm', 'model', 'agent', 'automation', 'orchestration',
            'pipeline', 'workflow', 'multi-agent', 'browser automation',
            'courier', 'free tier', 'rate limit', 'api', 'cli',
            'raindrop', 'bookmark', 'research', 'digest',
            'supabase', 'github', 'mcp', 'ide',
            'gemini', 'claude', 'chatgpt', 'kimi', 'deepseek', 'glm',
            'vercel', 'deployment', 'dashboard'
        ]
        
        text = f"{title} {excerpt} {' '.join(tags)}".lower()
        
        matches = []
        relevance_score = 0
        
        for keyword in vp_keywords:
            if keyword in text:
                matches.append(keyword)
                relevance_score += 1
        
        # Determine category
        categories = []
        if any(k in text for k in ['gemini', 'claude', 'chatgpt', 'kimi', 'deepseek', 'glm', 'llm', 'model']):
            categories.append('models_platforms')
        if any(k in text for k in ['agent', 'automation', 'orchestration', 'pipeline', 'workflow', 'multi-agent']):
            categories.append('architecture')
        if any(k in text for k in ['browser', 'courier', 'automation']):
            categories.append('courier_browser')
        if any(k in text for k in ['free tier', 'rate limit', 'api', 'pricing']):
            categories.append('cost_optimization')
        if any(k in text for k in ['supabase', 'github', 'vercel', 'deployment']):
            categories.append('infrastructure')
        
        return {
            'score': min(relevance_score, 10),
            'keyword_matches': matches[:10],  # Top 10 matches
            'categories': categories,
            'recommendation': self._generate_recommendation(relevance_score, categories, title)
        }
    
    def _generate_recommendation(self, score: int, categories: List[str], title: str) -> str:
        """Generate recommendation based on analysis."""
        if score >= 7:
            return f"HIGH RELEVANCE: Potential {categories[0] if categories else 'system'} improvement. Deep research recommended."
        elif score >= 4:
            return f"MEDIUM RELEVANCE: Related to {', '.join(categories[:2])}. Worth reviewing."
        elif score >= 2:
            return "LOW RELEVANCE: Tangentially related. Quick scan sufficient."
        else:
            return "MINIMAL RELEVANCE: Unlikely to impact VibePilot."
    
    def generate_report(self, collection_name: str, bookmarks: List[Dict], days: int) -> str:
        """Generate research report in markdown format."""
        date_str = datetime.now().strftime('%Y-%m-%d')
        
        report = f"""# Raindrop Research Report: {collection_name}
**Date:** {date_str}  
**Period:** Last {days} days  
**Bookmarks Reviewed:** {len(bookmarks)}

---

## Summary

| Metric | Value |
|--------|-------|
| Total Bookmarks | {len(bookmarks)} |
| High Relevance (7+) | {sum(1 for b in bookmarks if b['vibepilot_relevance']['score'] >= 7)} |
| Medium Relevance (4-6) | {sum(1 for b in bookmarks if 4 <= b['vibepilot_relevance']['score'] < 7)} |
| Low Relevance (<4) | {sum(1 for b in bookmarks if b['vibepilot_relevance']['score'] < 4)} |

### Categories Found
"""
        
        # Aggregate categories
        all_categories = {}
        for b in bookmarks:
            for cat in b['vibepilot_relevance']['categories']:
                all_categories[cat] = all_categories.get(cat, 0) + 1
        
        for cat, count in sorted(all_categories.items(), key=lambda x: -x[1]):
            report += f"- {cat.replace('_', ' ').title()}: {count}\n"
        
        report += "\n---\n\n## Detailed Findings\n\n"
        
        # Sort by relevance score
        sorted_bookmarks = sorted(bookmarks, key=lambda x: x['vibepilot_relevance']['score'], reverse=True)
        
        for i, b in enumerate(sorted_bookmarks, 1):
            rel = b['vibepilot_relevance']
            score = rel['score']
            
            # Priority badge
            if score >= 7:
                priority = "🔴 HIGH"
            elif score >= 4:
                priority = "🟡 MEDIUM"
            else:
                priority = "🟢 LOW"
            
            report += f"""### {i}. {b['title']}
**Priority:** {priority} (Score: {score}/10)  
**URL:** {b['url']}  
**Categories:** {', '.join(rel['categories']) or 'None'}  
**Tags:** {', '.join(b['tags']) or 'None'}

**Relevance:** {rel['recommendation']}

**Matched Keywords:** {', '.join(rel['keyword_matches']) or 'None'}

**Excerpt:**
> {b['excerpt'][:300] if b['excerpt'] else 'No excerpt available'}{"..." if b['excerpt'] and len(b['excerpt']) > 300 else ""}

---

"""
        
        report += """## Recommendations

Based on this research pass:

1. **Review HIGH priority items first** - These have direct VibePilot relevance
2. **Tag processed bookmarks** in Raindrop with `researched-YYYY-MM-DD` to avoid re-processing
3. **Create research tasks** for high-value findings (score 7+)

---

*Report generated by VibePilot Raindrop Research Agent*  
*Commit to research-considerations branch for Council review*
"""
        
        return report


def main():
    parser = argparse.ArgumentParser(description='Research Raindrop bookmarks for VibePilot')
    parser.add_argument('--collection', default='vibeflow', help='Collection name to research')
    parser.add_argument('--days', type=int, default=7, help='Number of days to look back')
    parser.add_argument('--output', help='Output file path')
    parser.add_argument('--watch', action='store_true', help='Check for new items and exit if none')
    
    args = parser.parse_args()
    
    researcher = RaindropResearcher()
    
    # Get collection
    print(f"Looking for collection: {args.collection}")
    collection = researcher.get_collection(args.collection)
    
    if not collection:
        print(f"Collection '{args.collection}' not found.")
        print("Available collections:")
        
        headers = {}
        if researcher.access_token:
            headers['Authorization'] = f'Bearer {researcher.access_token}'
        
        try:
            response = requests.get(
                f"{RAINDROP_API_BASE}/collections",
                headers=headers
            )
            if response.ok:
                cols = response.json().get('items', [])
                for col in cols:
                    print(f"  - {col.get('title')} (ID: {col.get('_id')})")
        except:
            pass
        
        sys.exit(1)
    
    print(f"Found collection: {collection.get('title')} (ID: {collection.get('_id')})")
    print(f"Collection count: {collection.get('count', 'unknown')}")
    
    # Get bookmarks
    print(f"\nFetching bookmarks from last {args.days} days...")
    bookmarks_data = researcher.get_bookmarks(collection.get('_id'), days=args.days)
    
    if not bookmarks_data:
        print("No bookmarks found in the specified time period.")
        if args.watch:
            sys.exit(0)  # Normal exit for watch mode
        sys.exit(1)
    
    print(f"Found {len(bookmarks_data)} bookmarks to research")
    
    # Research each bookmark
    researched = []
    for bookmark in bookmarks_data:
        findings = researcher.research_bookmark(bookmark)
        researched.append(findings)
    
    # Generate report
    report = researcher.generate_report(args.collection, researched, args.days)
    
    # Output
    if args.output:
        with open(args.output, 'w') as f:
            f.write(report)
        print(f"\nReport saved to: {args.output}")
    else:
        # Generate filename
        date_str = datetime.now().strftime('%Y%m%d')
        output_dir = Path('docs/research')
        output_dir.mkdir(parents=True, exist_ok=True)
        output_file = output_dir / f"raindrop-{args.collection}-{date_str}.md"
        
        with open(output_file, 'w') as f:
            f.write(report)
        print(f"\nReport saved to: {output_file}")
    
    # Summary
    high = sum(1 for b in researched if b['vibepilot_relevance']['score'] >= 7)
    print(f"\nResearch complete!")
    print(f"  High relevance: {high}")
    print(f"  Medium relevance: {sum(1 for b in researched if 4 <= b['vibepilot_relevance']['score'] < 7)}")
    print(f"  Low relevance: {sum(1 for b in researched if b['vibepilot_relevance']['score'] < 4)}")


if __name__ == '__main__':
    main()
