# TIER 0: NON-NEGOTIABLE RULES
# Hand-crafted. These exist because agents keep making these same mistakes.
# Edit this file directly, not auto-generated.
# Located at: .context/tools/tier0-static.md
# build.sh copies this into boot.md verbatim.

## Why These Rules Exist

Every rule below exists because an agent has repeatedly caused real problems
by violating it. These are not theoretical -- each one cost significant time
and cleanup work.

## Core Principles (the WHY)

- **Modular and agnostic** -- swap any component in a day. No vendor dependency.
- **Config-driven** -- everything configurable via JSON in config/. Nothing baked in.
- **Reversible** -- every change can be undone. No one-way doors.
- **Recoverable** -- GitHub + Supabase = full rebuild anywhere, any device.

## Absolute Rules (the DO NOT)

1. **NEVER hardcode anything.** Every hardcoded value eventually requires undoing a mess.
   Models, connectors, routing, agents -- all in config JSON files.

2. **NEVER hunt for .env files.** Credentials live in Supabase vault. Period.
   Agents have burned 30+ minutes searching for .env files that don't exist,
   then another 30 figuring out vault access. Use vault. First time. Every time.

3. **NEVER guess -- check first.** Guessing creates cleanup work.
   Read the existing code, query knowledge.db, check what's there before inventing.

4. **NEVER apply migrations directly.** Always go through GitHub first.

5. **NEVER modify the dashboard to fix a display issue.**
   Dashboard is the source of truth. If it looks wrong, the Go code is wrong.
   Fix the backend, not the display.

## Operational Rules (the DO)

6. **Governor is a systemd user service.**
   Use: systemctl --user (not sudo systemctl)
   Logs: journalctl --user -u vibepilot-governor (not journalctl -u governor)
   Getting stuck on this wastes entire sessions.

7. **Roles are defined in config, not guessed.**
   Look them up. Human does 2 things. Council does 2 things. Supervisor does 3 things.
   Check config/agents.json and system.json for role definitions.

8. **Read before you code.**
   Query knowledge.db for existing rules and patterns before starting any task.
   sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
   5 minutes of reading saves hours of rewriting.

## Human Boundaries

- Human reviews code and merges code. That's it.
- Human does NOT debug agent code, write agent code, or figure out architecture.
- Respect human time: batch questions together, don't ask one at a time.

## Going Deeper

When you need more than these basics:
- Architecture details: sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
- All prompts: sqlite3 .context/knowledge.db "SELECT name,role FROM prompts"
- Search docs: sqlite3 .context/knowledge.db "SELECT title,file_path FROM docs WHERE title LIKE '%<topic>%'"
- Full code map: cat .context/map.md
