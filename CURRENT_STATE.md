# VibePilot Current State

**Session 88 - 2026-03-29 21:52**

## Status: BROKEN - Governor not running

### What's Broken
- Governor needs `SUPABASE_SERVICE_KEY` and `VAULT_KEY` to start
- These are in GitHub secrets - I cannot access them
- Without governor running, no plans are processed, no tasks created

### What I Changed (needs rebuild)
- `governor/internal/connectors/runners.go` - Fixed CLIRunner to parse `claude` CLI output format
- The existing `governor.exe` was built BEFORE this fix

### To Fix
1. Set environment variables:
   ```
   SUPABASE_URL=https://qtpdzsinvifkgpxyxlaz.supabase.co
   SUPABASE_SERVICE_KEY=<from GitHub secrets>
   VAULT_KEY=<from GitHub secrets>
   ```

2. Rebuild governor (requires Go):
   ```
   cd C:\Users\MJLOCKBOX\VibePilot\governor
   go build -o governor.exe ./cmd/governor/
   ```

3. Run governor:
   ```
   ./governor.exe
   ```

### Root Cause of Empty plan_path
The CLIRunner expected NDJSON with `type: "text"` and `part.text`, but `claude -p --output-format json` outputs a single JSON object with `type: "result"` and text in `result` field.

### Files Modified This Session
- `governor/internal/connectors/runners.go` - CLIRunner parsing fix (NOT REBUILT)
- `governor/config/connectors.json` - Added claude-code connector
- `governor/config/models.json` - glm-5 uses claude-code connector
