#!/usr/bin/env python3
"""
Simple Raindrop Research - Public Collections

Fetches public Raindrop collections via share URLs.
No authentication required for public collections.

Usage:
    python scripts/research/raindrop_simple.py --url <public_collection_url> --days 7
"""

import os
import sys
import json
import argparse
import requests
import re
from datetime import datetime, timedelta
from typing import List, Dict, Optional
from pathlib import Path
from urllib.parse import unquote

sys.path.insert(0, str(Path(__file__).parent.parent.parent))


class SimpleRaindropResearcher:
    """Research public Raindrop collections without API auth."""
    
    def __init__(self):
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36'
        })
    
    def get_collection_from_url(self, share_url: str) -> List[Dict]:
        """Fetch bookmarks from a public Raindrop collection URL."""
        try:
            print(f"Fetching collection: {share_url}")
            
            response = self.session.get(share_url, timeout=30)
            response.raise_for_status()
            
            html = response.text
            
            # Extract bookmark data from embedded JSON
            # Raindrop embeds collection data in the page
            bookmarks = []
            
            # Look for bookmark items in the HTML
            # Pattern: data in script tags or specific HTML structure
            
            # Try to find JSON data
            json_match = re.search(r'window\.__INITIAL_STATE__\s*=\s*({.+?});', html, re.DOTALL)
            if json_match:
                try:
                    data = json.loads(json_match.group(1))
                    # Extract bookmarks from the state
                    bookmarks = self._extract_from_state(data)
                except:
                    pass
            
            # Fallback: Parse HTML for bookmark links
            if not bookmarks:
                bookmarks = self._extract_from_html(html, share_url)
            
            return bookmarks
            
        except Exception as e:
            print(f"Error fetching collection: {e}")
            return []
    
    def _extract_from_state(self, data: dict) -> List[Dict]:
        """Extract bookmarks from initial state JSON."""
        bookmarks = []
        
        # Raindrop structure varies, try common patterns
        if 'bookmarks' in data:
            items = data['bookmarks']
        elif 'raindrops' in data:
            items = data['raindrops']
        else:
            return []
        
        for item in items:
            bookmark = {
                '_id': item.get('_id', item.get('id')),
                'title': item.get('title', 'Untitled'),
                'link': item.get('link', ''),
                'excerpt': item.get('excerpt', ''),
                'tags': item.get('tags', []),
                'created': item.get('created'),
                'cover': item.get('cover', '')
            }
            bookmarks.append(bookmark)
        
        return bookmarks
    
    def _extract_from_html(self, html: str, base_url: str) -> List[Dict]:
        """Fallback: Extract bookmarks from HTML structure."""
        bookmarks = []
        
        # Look for bookmark containers
        # Raindrop uses specific class names and structures
        
        # Pattern for bookmark items
        import re
        
        # Find all bookmark links
        link_pattern = r'<a[^>]*href="([^"]+)"[^>]*class="[^"]*bookmark[^"]*"[^>]*>'
        title_pattern = r'<div[^>]*class="[^"]*title[^"]*"[^>]*>(.*?)</div>'
        
        links = re.findall(link_pattern, html, re.IGNORECASE)
        titles = re.findall(title_pattern, html, re.IGNORECASE | re.DOTALL)
        
        for i, link in enumerate(links[:20]):  # Limit to first 20
            title = titles[i] if i < len(titles) else 'Untitled'
            # Clean up title (remove HTML tags)
            title = re.sub(r'<[^>]+>', '', title).strip()
            
            bookmarks.append({
                '_id': f'html_{i}',
                'title': title,
                'link': link if link.startswith('http') else f"{base_url}{link}",
                'excerpt': '',
                'tags': [],
                'created': None
            })
        
        return bookmarks
    
    def research_bookmark(self, bookmark: Dict) -> Dict:
        """Research a bookmark in context of VibePilot."""
        url = bookmark.get('link', '')
        title = bookmark.get('title', 'Untitled')
        excerpt = bookmark.get('excerpt', '')
        tags = bookmark.get('tags', [])
        
        print(f"\nResearching: {title[:60]}...")
        print(f"  URL: {url[:80]}...")
        
        # Fetch content for deeper analysis
        content = self._fetch_content(url)
        
        # Analyze relevance
        relevance = self._analyze_relevance(title, excerpt, content, tags)
        
        return {
            'title': title,
            'url': url,
            'excerpt': excerpt[:500] if excerpt else '',
            'tags': tags,
            'created': bookmark.get('created'),
            'relevance': relevance,
            'content_snippet': content[:1500] if content else ''
        }
    
    def _fetch_content(self, url: str) -> str:
        """Fetch and extract readable content from URL."""
        try:
            # Skip certain URLs
            if any(skip in url for skip in ['youtube.com', 'twitter.com', 'x.com']):
                return ""
            
            resp = self.session.get(url, timeout=20)
            resp.raise_for_status()
            
            html = resp.text
            
            # Extract text content (simple approach)
            import re
            # Remove scripts and styles
            html = re.sub(r'<script[^>]*>.*?</script>', '', html, flags=re.DOTALL | re.IGNORECASE)
            html = re.sub(r'<style[^>]*>.*?</style>', '', html, flags=re.DOTALL | re.IGNORECASE)
            
            # Extract text
            text = re.sub(r'<[^>]+>', ' ', html)
            text = re.sub(r'\s+', ' ', text).strip()
            
            return text[:5000]  # First 5000 chars
            
        except Exception as e:
            print(f"  Could not fetch: {e}")
            return ""
    
    def _analyze_relevance(self, title: str, excerpt: str, content: str, tags: List[str]) -> Dict:
        """Analyze bookmark relevance to VibePilot."""
        vp_keywords = {
            'models_platforms': ['llm', 'model', 'ai', 'gemini', 'claude', 'chatgpt', 'kimi', 'deepseek', 'glm', 'gpt', 'api', 'free tier'],
            'architecture': ['agent', 'automation', 'orchestration', 'pipeline', 'workflow', 'multi-agent', 'system'],
            'courier_browser': ['browser', 'automation', 'playwright', 'selenium', 'scraping', 'courier'],
            'cost_optimization': ['pricing', 'cost', 'rate limit', 'token', 'efficiency', 'free', 'cheap'],
            'infrastructure': ['supabase', 'github', 'vercel', 'deployment', 'database', 'storage'],
            'research_tools': ['bookmark', 'raindrop', 'research', 'digest', 'newsletter', 'tool']
        }
        
        text = f"{title} {excerpt} {' '.join(tags)} {content[:2000]}".lower()
        
        scores = {}
        total_matches = []
        
        for category, keywords in vp_keywords.items():
            matches = [k for k in keywords if k in text]
            scores[category] = len(matches)
            total_matches.extend(matches)
        
        # Calculate overall score (0-10)
        max_category_score = max(scores.values()) if scores else 0
        total_score = min(sum(scores.values()) / 2, 10)
        
        # Determine primary category
        primary_category = max(scores, key=scores.get) if scores else 'general'
        
        return {
            'score': round(total_score, 1),
            'max_category_score': max_category_score,
            'primary_category': primary_category,
            'all_categories': {k: v for k, v in scores.items() if v > 0},
            'matched_keywords': list(set(total_matches))[:15]
        }
    
    def generate_report(self, collection_name: str, bookmarks: List[Dict], days: int) -> str:
        """Generate markdown report."""
        date_str = datetime.now().strftime('%Y-%m-%d')
        
        # Sort by relevance
        sorted_bookmarks = sorted(bookmarks, key=lambda x: x['relevance']['score'], reverse=True)
        
        # Count by priority
        high = sum(1 for b in sorted_bookmarks if b['relevance']['score'] >= 6)
        medium = sum(1 for b in sorted_bookmarks if 3 <= b['relevance']['score'] < 6)
        low = sum(1 for b in sorted_bookmarks if b['relevance']['score'] < 3)
        
        report = f"""# Raindrop Research Report: {collection_name}
**Date:** {date_str}  
**Period:** Last {days} days  
**Bookmarks Reviewed:** {len(bookmarks)}

---

## Summary

| Metric | Count |
|--------|-------|
| Total Bookmarks | {len(bookmarks)} |
| 🔴 High Priority (6+) | {high} |
| 🟡 Medium Priority (3-5) | {medium} |
| 🟢 Low Priority (<3) | {low} |

### Top Categories
"""
        
        # Aggregate categories
        all_cats = {}
        for b in sorted_bookmarks:
            cat = b['relevance']['primary_category']
            all_cats[cat] = all_cats.get(cat, 0) + 1
        
        for cat, count in sorted(all_cats.items(), key=lambda x: -x[1])[:5]:
            report += f"- {cat.replace('_', ' ').title()}: {count}\n"
        
        report += "\n---\n\n## Detailed Findings (by relevance)\n\n"
        
        for i, b in enumerate(sorted_bookmarks, 1):
            rel = b['relevance']
            score = rel['score']
            
            if score >= 6:
                badge = "🔴 HIGH"
            elif score >= 3:
                badge = "🟡 MEDIUM"
            else:
                badge = "🟢 LOW"
            
            report += f"""### {i}. {b['title']}
**Priority:** {badge} (Score: {score}/10)  
**Category:** {rel['primary_category'].replace('_', ' ').title()}  
**URL:** {b['url']}

**Relevance Analysis:**
- Categories: {', '.join(f"{k}({v})" for k, v in rel['all_categories'].items()) or 'None'}
- Matched keywords: {', '.join(rel['matched_keywords']) or 'None'}

**Excerpt:**
> {b['excerpt'][:300] if b['excerpt'] else 'No excerpt available'}

---

"""
        
        report += """## Recommendations

1. **High Priority Items** - Review for immediate applicability
2. **Tag processed bookmarks** in Raindrop with `researched-YYYY-MM-DD`
3. **Create tasks** for findings with score 7+

---

*Report generated by VibePilot Raindrop Research Agent*  
*Source: research-considerations branch*
"""
        
        return report


def main():
    parser = argparse.ArgumentParser(description='Research public Raindrop collections')
    parser.add_argument('--url', required=True, help='Public collection URL')
    parser.add_argument('--name', help='Collection name (for report title)')
    parser.add_argument('--days', type=int, default=7, help='Days to look back')
    parser.add_argument('--output', help='Output file path')
    
    args = parser.parse_args()
    
    researcher = SimpleRaindropResearcher()
    
    # Get collection name from URL if not provided
    collection_name = args.name or "Raindrop Collection"
    
    # Fetch bookmarks
    print(f"Fetching bookmarks from: {args.url}")
    bookmarks_data = researcher.get_collection_from_url(args.url)
    
    if not bookmarks_data:
        print("No bookmarks found or collection is not public.")
        print("\nTo make a collection public:")
        print("  1. Go to raindrop.io")
        print("  2. Open your collection")
        print("  3. Click 'Share' → 'Make Public'")
        print("  4. Copy the public URL")
        sys.exit(1)
    
    print(f"Found {len(bookmarks_data)} bookmarks")
    
    # Research each
    researched = []
    for bookmark in bookmarks_data:
        findings = researcher.research_bookmark(bookmark)
        researched.append(findings)
    
    # Generate report
    report = researcher.generate_report(collection_name, researched, args.days)
    
    # Save
    output_dir = Path('docs/research')
    output_dir.mkdir(parents=True, exist_ok=True)
    
    if args.output:
        output_file = Path(args.output)
    else:
        date_str = datetime.now().strftime('%Y%m%d')
        safe_name = collection_name.lower().replace(' ', '-')
        output_file = output_dir / f"raindrop-{safe_name}-{date_str}.md"
    
    with open(output_file, 'w') as f:
        f.write(report)
    
    print(f"\n✓ Report saved to: {output_file}")
    
    # Summary
    print(f"\nSummary:")
    print(f"  High (6+): {sum(1 for b in researched if b['relevance']['score'] >= 6)}")
    print(f"  Medium (3-5): {sum(1 for b in researched if 3 <= b['relevance']['score'] < 6)}")
    print(f"  Low (<3): {sum(1 for b in researched if b['relevance']['score'] < 3)}")


if __name__ == '__main__':
    main()
