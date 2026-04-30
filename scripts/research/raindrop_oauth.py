#!/usr/bin/env python3
"""
Raindrop Research with OAuth

Uses official Raindrop API with OAuth authentication.

Usage:
    # First time setup - get access token
    python scripts/research/raindrop_oauth.py --auth
    
    # Research collection
    python scripts/research/raindrop_oauth.py --collection 59987361 --days 7
    
    # List your collections
    python scripts/research/raindrop_oauth.py --list-collections
"""

import os
import sys
import json
import argparse
import requests
import webbrowser
import http.server
import socketserver
from datetime import datetime, timedelta
from pathlib import Path
from urllib.parse import urlencode, parse_qs, urlparse
from typing import List, Dict, Optional

sys.path.insert(0, str(Path(__file__).parent.parent.parent))

# Raindrop credentials
CLIENT_ID = "6995298e703bf1e11e73890a"
CLIENT_SECRET = "4b698b0b-6ce6-469d-aa4f-aff29a75f0ec"
RAINDROP_API = "https://api.raindrop.io/rest/v1"
REDIRECT_URI = "http://localhost:8080/callback"

# Token storage
TOKEN_FILE = Path(__file__).parent / '.raindrop_token.json'


class RaindropOAuthServer(http.server.BaseHTTPRequestHandler):
    """Handle OAuth callback."""
    auth_code = None
    
    def do_GET(self):
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query)
        
        if 'code' in query:
            RaindropOAuthServer.auth_code = query['code'][0]
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            self.wfile.write(b"""
                <html><body style="font-family: Arial; text-align: center; padding: 50px;">
                <h1>Authorized!</h1>
                <p>You can close this window.</p>
                </body></html>
            """)
        else:
            self.send_response(400)
            self.end_headers()
    
    def log_message(self, format, *args):
        pass


class RaindropResearcher:
    """Research Raindrop bookmarks via OAuth API."""
    
    def __init__(self):
        self.access_token = None
        self.refresh_token = None
        self._load_token()
    
    def _load_token(self):
        """Load token from file."""
        if TOKEN_FILE.exists():
            with open(TOKEN_FILE) as f:
                data = json.load(f)
                self.access_token = data.get('access_token')
                self.refresh_token = data.get('refresh_token')
    
    def _save_token(self, access_token: str, refresh_token: str = None):
        """Save token to file."""
        data = {
            'access_token': access_token,
            'refresh_token': refresh_token,
            'saved_at': datetime.now().isoformat()
        }
        with open(TOKEN_FILE, 'w') as f:
            json.dump(data, f)
        self.access_token = access_token
        self.refresh_token = refresh_token
    
    def authenticate(self):
        """Run OAuth flow to get access token."""
        # Generate auth URL
        auth_url = f"https://raindrop.io/oauth/authorize?" + urlencode({
            'client_id': CLIENT_ID,
            'redirect_uri': REDIRECT_URI,
            'response_type': 'code'
        })
        
        print(f"Opening browser to authorize...")
        print(f"If browser doesn't open, visit: {auth_url}")
        webbrowser.open(auth_url)
        
        # Start server to receive callback
        print("\nWaiting for authorization (timeout: 2 minutes)...")
        RaindropOAuthServer.auth_code = None
        
        with socketserver.TCPServer(("", 8080), RaindropOAuthServer) as httpd:
            httpd.timeout = 120
            httpd.handle_request()
        
        if not RaindropOAuthServer.auth_code:
            print("X No authorization code received")
            return False
        
        print("OK Authorization code received")
        
        # Exchange code for token
        token_data = {
            'grant_type': 'authorization_code',
            'client_id': CLIENT_ID,
            'client_secret': CLIENT_SECRET,
            'code': RaindropOAuthServer.auth_code,
            'redirect_uri': REDIRECT_URI
        }
        
        resp = requests.post(f"{RAINDROP_API}/oauth/access_token", data=token_data)
        
        if resp.ok:
            data = resp.json()
            self._save_token(data['access_token'], data.get('refresh_token'))
            print("✓ Access token saved")
            return True
        else:
            print(f"✗ Token exchange failed: {resp.text}")
            return False
    
    def _api_get(self, endpoint: str, params: dict = None) -> Optional[dict]:
        """Make authenticated API GET request."""
        if not self.access_token:
            print("Not authenticated. Run with --auth first.")
            return None
        
        headers = {'Authorization': f'Bearer {self.access_token}'}
        url = f"{RAINDROP_API}/{endpoint}"
        
        resp = requests.get(url, headers=headers, params=params)
        
        if resp.status_code == 401:
            print("Token expired. Run with --auth to re-authenticate.")
            return None
        
        resp.raise_for_status()
        return resp.json()
    
    def get_collections(self) -> List[Dict]:
        """Get user's collections."""
        data = self._api_get('collections')
        return data.get('items', []) if data else []
    
    def get_bookmarks(self, collection_id: int, page: int = 0) -> List[Dict]:
        """Get bookmarks from collection."""
        data = self._api_get(f'raindrops/{collection_id}', {'page': page, 'perpage': 50})
        return data.get('items', []) if data else []
    
    def research_bookmark(self, bookmark: Dict) -> Dict:
        """Research bookmark in VibePilot context."""
        title = bookmark.get('title', 'Untitled')
        url = bookmark.get('link', '')
        excerpt = bookmark.get('excerpt', '')
        tags = bookmark.get('tags', [])
        created = bookmark.get('created')
        
        print(f"Researching: {title[:50]}...")
        
        # Analyze relevance
        relevance = self._analyze_relevance(title, excerpt, tags)
        
        return {
            'title': title,
            'url': url,
            'excerpt': excerpt,
            'tags': tags,
            'created': created,
            'relevance': relevance
        }
    
    def _analyze_relevance(self, title: str, excerpt: str, tags: List[str]) -> Dict:
        """Analyze VibePilot relevance."""
        keywords = {
            'models_platforms': ['llm', 'model', 'ai', 'gemini', 'claude', 'chatgpt', 'kimi', 'deepseek', 'api', 'free tier'],
            'architecture': ['agent', 'automation', 'orchestration', 'pipeline', 'workflow', 'multi-agent'],
            'courier_browser': ['browser', 'automation', 'playwright', 'scraping', 'courier'],
            'cost_optimization': ['pricing', 'cost', 'rate limit', 'token', 'efficiency'],
            'infrastructure': ['supabase', 'github', 'vercel', 'deployment']
        }
        
        text = f"{title} {excerpt} {' '.join(tags)}".lower()
        
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
            'all_scores': scores,
            'matches': list(set(all_matches))[:10]
        }
    
    def generate_report(self, collection_name: str, bookmarks: List[Dict], days: int) -> str:
        """Generate markdown report."""
        date_str = datetime.now().strftime('%Y-%m-%d')
        
        # Sort by relevance
        sorted_bm = sorted(bookmarks, key=lambda x: x['relevance']['score'], reverse=True)
        
        high = sum(1 for b in sorted_bm if b['relevance']['score'] >= 6)
        med = sum(1 for b in sorted_bm if 3 <= b['relevance']['score'] < 6)
        low = sum(1 for b in sorted_bm if b['relevance']['score'] < 3)
        
        report = f"""# Raindrop Research: {collection_name}
**Date:** {date_str} | **Period:** Last {days} days | **Bookmarks:** {len(bookmarks)}

## Summary
| Priority | Count |
|----------|-------|
| 🔴 High (6+) | {high} |
| 🟡 Medium (3-5) | {med} |
| 🟢 Low (<3) | {low} |

---

"""
        
        for i, b in enumerate(sorted_bm, 1):
            rel = b['relevance']
            score = rel['score']
            badge = "HIGH" if score >= 6 else "MED" if score >= 3 else "LOW"
            
            report += f"""### {i}. {b['title']}
**{badge} Score:** {score}/10 | **Category:** {rel['primary_category'].replace('_', ' ').title()}

**URL:** {b['url']}

**Relevance:** {', '.join(rel['matches']) or 'General'}

> {b['excerpt'][:200] if b['excerpt'] else 'No excerpt'}

---

"""
        
        report += "*Report generated by VibePilot Research Agent*"
        return report


def main():
    parser = argparse.ArgumentParser(description='Raindrop Research with OAuth')
    parser.add_argument('--auth', action='store_true', help='Authenticate with Raindrop')
    parser.add_argument('--list-collections', action='store_true', help='List your collections')
    parser.add_argument('--collection', type=int, help='Collection ID to research')
    parser.add_argument('--days', type=int, default=7, help='Days to look back')
    parser.add_argument('--output', help='Output file path')
    
    args = parser.parse_args()
    
    researcher = RaindropResearcher()
    
    if args.auth:
        if researcher.authenticate():
            print("\n✓ Authentication complete")
        else:
            print("\n✗ Authentication failed")
            sys.exit(1)
    
    elif args.list_collections:
        collections = researcher.get_collections()
        print(f"\nYour collections ({len(collections)}):")
        for c in collections:
            print(f"  - {c.get('title')} (ID: {c.get('_id')}, Count: {c.get('count', 0)})")
    
    elif args.collection:
        print(f"Fetching bookmarks from collection {args.collection}...")
        bookmarks = researcher.get_bookmarks(args.collection)
        
        if not bookmarks:
            print("No bookmarks found.")
            sys.exit(1)
        
        print(f"Found {len(bookmarks)} bookmarks")
        
        # Get collection name
        collections = researcher.get_collections()
        coll_name = next((c['title'] for c in collections if c['_id'] == args.collection), f"Collection {args.collection}")
        
        # Research each
        researched = [researcher.research_bookmark(b) for b in bookmarks]
        
        # Generate report
        report = researcher.generate_report(coll_name, researched, args.days)
        
        # Save
        output_dir = Path('docs/research')
        output_dir.mkdir(parents=True, exist_ok=True)
        
        if args.output:
            output_file = Path(args.output)
        else:
            date_str = datetime.now().strftime('%Y%m%d')
            output_file = output_dir / f"raindrop-{args.collection}-{date_str}.md"
        
        with open(output_file, 'w') as f:
            f.write(report)
        
        print(f"\nOK Report saved: {output_file}")
        print(f"  High: {sum(1 for b in researched if b['relevance']['score'] >= 6)}")
        print(f"  Medium: {sum(1 for b in researched if 3 <= b['relevance']['score'] < 6)}")
        print(f"  Low: {sum(1 for b in researched if b['relevance']['score'] < 3)}")
    
    else:
        print("Use --auth to authenticate, --list-collections to see collections,")
        print("or --collection <id> to research bookmarks.")


if __name__ == '__main__':
    main()
