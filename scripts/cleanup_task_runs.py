#!/usr/bin/env python3
"""
VibePilot Task Run Cleanup Script

Fixes bad token data in task_runs table:
- Sets tokens_used = tokens_in + tokens_out where real data exists
- Keeps historical runs but corrects the values

Run: python scripts/cleanup_task_runs.py
"""

import os
import sys
from dotenv import load_dotenv
from supabase import create_client

load_dotenv()

SUPABASE_URL = os.getenv("SUPABASE_URL")
SUPABASE_KEY = os.getenv("SUPABASE_KEY")

if not SUPABASE_URL or not SUPABASE_KEY:
    print("ERROR: Missing SUPABASE_URL or SUPABASE_KEY")
    sys.exit(1)

db = create_client(SUPABASE_URL, SUPABASE_KEY)


def main():
    print("=" * 60)
    print("VIBEPILOT TASK RUN CLEANUP")
    print("=" * 60)

    print("\n1. Analyzing task_runs...")
    runs = (
        db.table("task_runs")
        .select("id, tokens_used, tokens_in, tokens_out, model_id")
        .execute()
    )

    if not runs.data:
        print("No task_runs found.")
        return

    print(f"   Total runs: {len(runs.data)}")

    needs_fix = []
    suspicious = []

    for run in runs.data:
        tokens_used = run.get("tokens_used") or 0
        tokens_in = run.get("tokens_in") or 0
        tokens_out = run.get("tokens_out") or 0
        calculated = tokens_in + tokens_out

        if tokens_in > 0 or tokens_out > 0:
            if tokens_used != calculated:
                needs_fix.append(
                    {
                        "id": run["id"],
                        "model": run.get("model_id", "unknown"),
                        "old": tokens_used,
                        "new": calculated,
                    }
                )

        if tokens_used > 1000 and tokens_in == 0 and tokens_out == 0:
            suspicious.append(
                {
                    "id": run["id"],
                    "model": run.get("model_id", "unknown"),
                    "tokens_used": tokens_used,
                }
            )

    print(f"\n2. Runs needing fix (tokens_used != in+out): {len(needs_fix)}")
    for r in needs_fix[:10]:
        print(f"   {r['id'][:8]}... ({r['model']}): {r['old']} -> {r['new']}")
    if len(needs_fix) > 10:
        print(f"   ... and {len(needs_fix) - 10} more")

    print(f"\n3. Suspicious runs (tokens_used > 1000, but in=out=0): {len(suspicious)}")
    for r in suspicious:
        print(
            f"   {r['id'][:8]}... ({r['model']}): {r['tokens_used']} tokens - NO in/out data"
        )

    if suspicious:
        print("\n   These runs have hardcoded test values with no real token data.")
        print("   Options:")
        print("   a) Set tokens_used = 0 (assume test data)")
        print("   b) Delete these runs entirely")
        print("   c) Skip for now")
        choice = input("\n   Choice (a/b/c): ").strip().lower()

        if choice == "a":
            for r in suspicious:
                db.table("task_runs").update({"tokens_used": 0}).eq(
                    "id", r["id"]
                ).execute()
            print(f"   Set tokens_used=0 for {len(suspicious)} suspicious runs")
        elif choice == "b":
            for r in suspicious:
                db.table("task_runs").delete().eq("id", r["id"]).execute()
            print(f"   Deleted {len(suspicious)} suspicious runs")
        else:
            print("   Skipping suspicious runs")

    if needs_fix:
        print(f"\n4. Fixing {len(needs_fix)} runs with real token data...")
        confirm = (
            input(f"   Update tokens_used for these {len(needs_fix)} runs? (y/n): ")
            .strip()
            .lower()
        )

        if confirm == "y":
            for r in needs_fix:
                db.table("task_runs").update({"tokens_used": r["new"]}).eq(
                    "id", r["id"]
                ).execute()
            print(f"   Updated {len(needs_fix)} runs")
        else:
            print("   Skipped")

    print("\n5. Verifying final totals...")
    runs = (
        db.table("task_runs")
        .select("tokens_used, tokens_in, tokens_out, model_id")
        .execute()
    )

    total_used = 0
    total_in = 0
    total_out = 0
    by_model = {}

    for run in runs.data or []:
        tu = run.get("tokens_used") or 0
        ti = run.get("tokens_in") or 0
        to = run.get("tokens_out") or 0
        m = run.get("model_id", "unknown")

        total_used += tu
        total_in += ti
        total_out += to

        if m not in by_model:
            by_model[m] = {"used": 0, "in": 0, "out": 0}
        by_model[m]["used"] += tu
        by_model[m]["in"] += ti
        by_model[m]["out"] += to

    print(f"\n   Total tokens_used: {total_used:,}")
    print(f"   Total tokens_in:   {total_in:,}")
    print(f"   Total tokens_out:  {total_out:,}")
    print(f"   Calculated total:  {total_in + total_out:,}")

    print("\n   By model:")
    for m, d in sorted(by_model.items(), key=lambda x: -x[1]["used"]):
        calc = d["in"] + d["out"]
        match = (
            "OK" if d["used"] == calc else f"MISMATCH (used={d['used']}, calc={calc})"
        )
        print(f"   {m}: in={d['in']}, out={d['out']} | {match}")

    print("\n" + "=" * 60)
    print("CLEANUP COMPLETE")
    print("=" * 60)


if __name__ == "__main__":
    main()
