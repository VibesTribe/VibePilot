#!/usr/bin/env python3
"""
Raindrop Authentication Helper

Generates authentication URL for Raindrop OAuth flow.
Run this once to get an access token, then store it in the vault.
"""

import os
import sys
import webbrowser
import http.server
import socketserver
import urllib.parse
from pathlib import Path

# Add project root to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

RAINDROP_CLIENT_ID = "6995298e703bf1e11e73890a"
RAINDROP_CLIENT_SECRET = "4b698b0b-6ce6-469d-aa4f-aff29a75f0ec"
RAINDROP_AUTH_URL = "https://raindrop.io/oauth/authorize"
RAINDROP_TOKEN_URL = "https://raindrop.io/oauth/access_token"
REDIRECT_URI = "http://localhost:8080/callback"


class CallbackHandler(http.server.SimpleHTTPRequestHandler):
    """Handle OAuth callback."""
    
    def do_GET(self):
        """Handle GET request to callback URL."""
        parsed = urllib.parse.urlparse(self.path)
        query = urllib.parse.parse_qs(parsed.query)
        
        if 'code' in query:
            auth_code = query['code'][0]
            self.server.auth_code = auth_code
            
            self.send_response(200)
            self.send_header('Content-type', 'text/html')
            self.end_headers()
            self.wfile.write(b"""
                <html>
                <body style="font-family: Arial, sans-serif; text-align: center; padding: 50px;">
                    <h1>✓ Authentication Successful</h1>
                    <p>You can close this window and return to the terminal.</p>
                </body>
                </html>
            """)
        else:
            self.send_response(400)
            self.end_headers()
            self.wfile.write(b"Error: No authorization code received")
    
    def log_message(self, format, *args):
        """Suppress log messages."""
        pass


def get_auth_url():
    """Generate authorization URL."""
    params = {
        'client_id': RAINDROP_CLIENT_ID,
        'redirect_uri': REDIRECT_URI,
        'response_type': 'code'
    }
    return f"{RAINDROP_AUTH_URL}?{urllib.parse.urlencode(params)}"


def main():
    print("=" * 60)
    print("Raindrop.io Authentication for VibePilot")
    print("=" * 60)
    print()
    
    # Option 1: Try public collections first (simpler)
    print("Option 1: Public Collections (Recommended for now)")
    print("-" * 60)
    print("Make your Raindrop collections public:")
    print("  1. Go to raindrop.io")
    print("  2. Open your 'vibeflow' and/or 'vibepilot' collections")
    print("  3. Click Share → Make Public")
    print("  4. Note the collection IDs from the URLs")
    print()
    print("Then run: python scripts/research/raindrop_researcher.py --collection <name>")
    print()
    
    # Option 2: OAuth flow
    print("Option 2: OAuth Authentication (For private collections)")
    print("-" * 60)
    print("This will open a browser to authorize VibePilot.")
    response = input("Proceed with OAuth? (y/N): ")
    
    if response.lower() != 'y':
        print("\nExiting. Use Option 1 (public collections) for now.")
        return
    
    auth_url = get_auth_url()
    
    print(f"\nOpening browser to: {auth_url}")
    print("Waiting for authorization...")
    
    webbrowser.open(auth_url)
    
    # Start temporary server to receive callback
    with socketserver.TCPServer(("", 8080), CallbackHandler) as httpd:
        httpd.auth_code = None
        httpd.timeout = 120  # 2 minute timeout
        
        try:
            httpd.handle_request()
        except KeyboardInterrupt:
            print("\nCancelled by user")
            return
    
    if httpd.auth_code:
        print(f"\n✓ Authorization code received")
        print(f"\nNext steps:")
        print(f"  1. Exchange this code for an access token")
        print(f"  2. Store the token in Supabase vault as 'RAINDROP_ACCESS_TOKEN'")
        print(f"\nAuthorization code: {httpd.auth_code}")
        
        # TODO: Exchange code for token
        print("\nNote: Token exchange not yet implemented.")
        print("Use public collections for now, or implement token exchange.")
    else:
        print("\n✗ No authorization code received")


if __name__ == '__main__':
    main()
