#!/usr/bin/env python3
"""Get Raindrop OAuth token - run this first, then visit the URL."""

import http.server
import socketserver
import json
from urllib.parse import urlparse, parse_qs
from pathlib import Path
import requests

CLIENT_ID = "6995298e703bf1e11e73890a"
CLIENT_SECRET = "4b698b0b-6ce6-469d-aa4f-aff29a75f0ec"
TOKEN_FILE = Path(__file__).parent / '.raindrop_token.json'

class Handler(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        query = parse_qs(parsed.query)
        
        if 'code' in query:
            code = query['code'][0]
            print(f"\n[OK] Got authorization code: {code[:20]}...")
            
            # Exchange for token
            print("[...] Exchanging for access token...")
            resp = requests.post(
                'https://api.raindrop.io/rest/v1/oauth/access_token',
                data={
                    'grant_type': 'authorization_code',
                    'client_id': CLIENT_ID,
                    'client_secret': CLIENT_SECRET,
                    'code': code,
                    'redirect_uri': 'http://localhost:8080/callback'
                }
            )
            
            if resp.ok:
                data = resp.json()
                token_data = {
                    'access_token': data['access_token'],
                    'refresh_token': data.get('refresh_token'),
                    'expires': data.get('expires', 'never'),
                    'saved_at': json.dumps({})
                }
                
                with open(TOKEN_FILE, 'w') as f:
                    json.dump(token_data, f, indent=2)
                
                print(f"[OK] Token saved to {TOKEN_FILE}")
                print(f"[OK] Access token: {data['access_token'][:30]}...")
                
                # Success response
                self.send_response(200)
                self.send_header('Content-type', 'text/html')
                self.end_headers()
                self.wfile.write(b"""
                    <html><body style="font-family: Arial; text-align: center; padding: 50px;">
                    <h1>SUCCESS!</h1>
                    <p>Authorization complete. Token saved.</p>
                    <p>You can close this window and return to the terminal.</p>
                    </body></html>
                """)
            else:
                print(f"[ERROR] Token exchange failed: {resp.text}")
                self.send_response(400)
                self.end_headers()
                self.wfile.write(f"Error: {resp.text}".encode())
        else:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b"No code received")
    
    def log_message(self, format, *args):
        pass

print("="*60)
print("Raindrop OAuth Token Generator")
print("="*60)
print()
print("Step 1: Visit this URL in your browser:")
print()
print("https://raindrop.io/oauth/authorize?client_id=6995298e703bf1e11e73890a&redirect_uri=http%3A%2F%2Flocalhost%3A8080%2Fcallback&response_type=code")
print()
print("Step 2: Authorize the app")
print("Step 3: Token will be saved automatically")
print()
print("Waiting for authorization...")
print("(Server running on http://localhost:8080)")
print()

with socketserver.TCPServer(("", 8080), Handler) as httpd:
    httpd.serve_forever()
