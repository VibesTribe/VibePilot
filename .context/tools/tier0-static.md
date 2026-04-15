# TIER 0: NON-NEGOTIABLE RULES
# Hand-crafted. These exist because agents keep making these same mistakes.
# Edit this file directly. Not auto-generated.
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

5. **ALWAYS push to GitHub.** Local-only work gets lost. Commit and push.
   This has caused more lost work than anything else.

6. **NEVER take shortcuts or "do it later."** Every shortcut becomes a bigger problem later.
   "I'll just hardcode this for now" = days of cleanup later.
   "I'll skip the config and bake it in" = can't swap without rewrite.
   "I'll just put this here temporarily" = permanent technical debt.
   "I'll wire it later" = the wiring never happens and the next agent is blind.
   If you can't do it properly right now, stop and say so. Don't half-do it.
   The right amount of work is the full amount. Not a TODO comment that becomes someone else's crisis.

7. **Go SERVES VibePilot's design. It does NOT invent new processes.**
   Go is the plumbing that makes VibePilot's actual processes run.
   It writes to Supabase in the format the dashboard expects.
   Do NOT rewrite Go with hallucinated processes that aren't in the spec.
   Do NOT modify Supabase schema to accommodate hallucinated Go code.
   Supabase is the contract. Dashboard shows what VibePilot state IS.
   Go conforms to both, not the other way around.

## Operational Rules (the DO)

8. **Governor is a systemd user service.**
   Use: systemctl --user (not sudo systemctl)
   Logs: journalctl --user -u vibepilot-governor (not journalctl -u governor)
   Getting stuck on this wastes entire sessions.

9. **Roles are defined in config, not guessed.** Look them up, don't invent.

   **Supervisor (3 responsibilities):**
   - Reviews task plans against PRD for alignment + plan quality. Calls Council for complex plans.
   - Reviews task output against task prompt + expected output + quality/security. Approved -> testing -> auto-merge if pass.
   - Approves basic researcher suggestions (new platform/model). Complex architecture -> Council -> Human.

   **Council (2 responsibilities):**
   - Reviews complex task plans escalated by Supervisor.
   - Reviews complex architecture changes escalated by Supervisor.

   **Human (Vibes) -- NOT technical. Does NOT code, merge, debug, or review code:**
   - Reviews visual UI/UX after visual testing. Requests changes or approves.
   - Receives Council-reviewed architecture suggestions. Asks questions. Final yes/no.

   **Merge system -- fully automated, zero human involvement:**
   - Supervisor approves output -> testing phase -> auto-merge on pass
   - Task branch created, merged to module branch, task branch deleted
   - Module branch merged after further Supervisor + Tester review
   - Merge problems are solved by agents, not human

   **Maintenance -- ONLY agent with git write access. No approval = no change.**

10. **Read before you code.**
   Query knowledge.db for existing rules and patterns before starting any task.
   sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
   5 minutes of reading saves hours of rewriting.

## Human Boundaries

- Human is NOT technical. Never assume technical knowledge.
- Human sees visual outputs and makes aesthetic/UX decisions.
- Human makes strategic architecture decisions after Council review.
- Human does NOT code, merge, debug, review code, or fix merge conflicts.
- Respect human time: batch questions together, don't ask one at a time.

## Going Deeper

When you need more than these basics:
- Architecture details: sqlite3 .context/knowledge.db "SELECT title,content FROM rules WHERE priority='high'"
- All prompts: sqlite3 .context/knowledge.db "SELECT name,role FROM prompts"
- Search docs: sqlite3 .context/knowledge.db "SELECT title,file_path FROM docs WHERE title LIKE '%<topic>%'"
- Full code map: cat .context/map.md
- Context layer explained: docs/CONTEXT_KNOWLEDGE_LAYER.md

## Post-Task Update Discipline (NON-OPTIONAL)

After completing any significant task (new feature, bug fix, config change, research, doc update),
you MUST update the relevant docs to prevent the next agent from flying blind:

1. **CURRENT_STATE.md** -- Update the section(s) affected by your work.
   If you changed anything real, the state doc must reflect it.

2. **WYNTK (VIBEPILOT_WHAT_YOU_NEED_TO_KNOW.md)** -- If your change affects:
   - Architecture (new packages, moved files, new config files)
   - File paths (anything moved or renamed)
   - How something works (new procedures, changed commands)
   - Technical constraints (new hardware info, changed limits)
   Then update the relevant section. Don't rewrite the whole file -- patch what changed.

3. **tier0-static.md** -- If you discovered a new rule that agents keep violating,
   add it here. This is the single source of truth for rules.

4. **This TODO list** -- Mark items done, add new items discovered during work.

5. **Commit and push** -- Local-only work gets lost. Always push.

The .context/ layer (boot.md, knowledge.db, map.md) auto-rebuilds on commit.
But the SOURCE docs it reads from (tier0-static.md, CURRENT_STATE.md, WYNTK, etc.)
only get updated if you update them. The auto-build is useless if sources are stale.

**Why this matters:** Agents swap frequently. Each new agent starts from these docs.
A gap here means the next agent repeats mistakes, wastes time, or breaks things.
The prevention system only works if we maintain the prevention system.
