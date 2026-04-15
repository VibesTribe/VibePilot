#!/usr/bin/env python3
"""Get Raindrop OAuth token - uses correct API endpoint and JSON format."""

import requests
import json
from pathlib import Path

CLIENT_ID = "6995298e703bf1e11e73890a"
CLIENT_SECRET = "4b698b0b-6ce6-469d-aa4f-aff29a75f0ec"
TOKEN_FILE = Path(__file__).parent / '.raindrop_token.json'

print("="*60)
print("Raindrop Token Exchange")
print("="*60)
print()
print("IMPORTANT: The authorization code expires in ~60 seconds!")
print("Paste the code immediately after getting it from the browser.")
print()

code = input("Authorization code from URL: ").strip()

if not code:
    print("Error: No code provided")
    exit(1)

print("\n[...] Exchanging code for token...")

# Use the correct v1 endpoint with JSON format
resp = requests.post(
    "https://api.raindrop.io/v1/oauth/access_token",
    headers={"Content-Type": "application/json"},
    json={
        "code": code,
        "client_id": CLIENT_ID,
        "redirect_uri": "https://vibestribe.github.io/vibeflow/",
        "client_secret": CLIENT_SECRET,
        "grant_type": "authorization_code"
    }
)

print(f"\nStatus: {resp.status_code}")

if resp.ok:
    data = resp.json()
    
    # Save token
    with open(TOKEN_FILE, 'w') as f:
        json.dump(data, f, indent=2)
    
    print(f"\n[OK] Token saved to {TOKEN_FILE}")
    print(f"Access token: {data['access_token'][:30]}...")
    print(f"Expires in: {data.get('expires_in', 'unknown')} seconds")
    print(f"\nNext step:")
    print("  python scripts/research/raindrop_oauth.py --collection 59987361 --days 7")
else:
    print(f"\n[ERROR] Failed: {resp.text}")
    print("\nCommon causes:")
    print("  - Code expired (get a fresh one)")
    print("  - Code already used (get a fresh one)")
    print("  - Wrong redirect_uri in app settings")
