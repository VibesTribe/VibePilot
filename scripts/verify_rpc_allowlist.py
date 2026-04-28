#!/usr/bin/env python3
"""Verify RPC allowlist coverage: Go callers vs allowlist vs DB functions.
Run from VibePilot root: python3 scripts/verify_rpc_allowlist.py
"""
import subprocess
import re
import sys

def run(cmd):
    r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=30)
    return r.stdout

def get_go_calls():
    """Find all RPC names called from Go code."""
    output = run("grep -rn 'database\\.RPC\\|database\\.CallRPC\\|database\\.CallRPCInto\\|\\.RPC(ctx,' governor/ --include='*.go' | grep -v '_test' | grep -v 'func.*RPC'")
    calls = set()
    for line in output.split('\n'):
        match = re.search(r'RPC\([^,]+,\s*"([^"]+)"', line)
        if match:
            calls.add(match.group(1))
    return calls

def get_allowlist():
    """Extract RPC allowlist from rpc.go."""
    content = run("cat governor/internal/db/rpc.go")
    return set(re.findall(r'"([a-z_]+)":\s*true', content))

def get_db_functions():
    """Get all public functions from PostgreSQL."""
    output = run('psql -d vibepilot -t -c "SELECT proname FROM pg_proc WHERE pronamespace=(SELECT oid FROM pg_namespace WHERE nspname=\'public\') ORDER BY proname"')
    return {f.strip() for f in output.strip().split('\n') if f.strip()}

def main():
    go_calls = get_go_calls()
    allowlist = get_allowlist()
    db_functions = get_db_functions()

    print(f"Go callers: {len(go_calls)} | Allowlist: {len(allowlist)} | DB functions: {len(db_functions)}")
    print()

    # Critical: Go calls not in allowlist (SILENT FAILURES at runtime)
    missing = go_calls - allowlist
    if missing:
        print(f"CRITICAL: {len(missing)} RPCs called from Go but NOT in allowlist (will silently fail):")
        for n in sorted(missing):
            print(f"  !!! {n}")
        print()
    else:
        print("OK: All Go RPC calls are in the allowlist.")
        print()

    # Critical: Go calls not in DB (will error at runtime)
    missing_db = go_calls - db_functions
    if missing_db:
        print(f"CRITICAL: {len(missing_db)} RPCs called from Go but NOT in database:")
        for n in sorted(missing_db):
            print(f"  !!! {n}")
        print()
    else:
        print("OK: All Go RPC calls exist in the database.")
        print()

    # Warning: In allowlist but not in DB (ghost entries)
    ghosts = allowlist - db_functions
    if ghosts:
        print(f"WARN: {len(ghosts)} allowlist entries have no matching DB function (ghost entries):")
        for n in sorted(ghosts):
            print(f"  ghost: {n}")
        print()

    # Info: In allowlist but never called from Go
    unused = allowlist - go_calls
    if unused:
        print(f"INFO: {len(unused)} allowlist entries never called from Go (may be dashboard/future use):")
        for n in sorted(unused):
            print(f"  unused: {n}")

    # Return non-zero if critical issues found
    return 1 if (missing or missing_db) else 0

if __name__ == '__main__':
    sys.exit(main())
