# VibePilot Task Lifecycle & Git Architecture

## Current Task Flow (what the code actually does)

```
Dashboard creates task → status: available
        ↓
handleTaskAvailable
  - Claims task (claim_task RPC)
  - Creates branch: git checkout -b task/{slice}/{number} IN THE SAME DIRECTORY
  - Routes to model (connector selection)
  - Executes via agent session
  - Commits output to branch (commitOutput)
  - Transitions → review
        ↓
handleTaskReview
  - Supervisor agent reviews output
  - Decision: approved → testing
               fail → available (branch deleted)
               needs_revision → available (branch deleted)
               council_review → council_review
               reroute → available (branch deleted)
        ↓
handleTaskTesting
  - Tester agent runs tests on the branch
  - Decision: pass → complete, then auto-merge to TEST_MODULES/{slice}
               fail → available (branch deleted)
               unclear → awaiting_human
        ↓
Auto-merge (after test pass)
  - Merges task branch → TEST_MODULES/{slice} (e.g. TEST_MODULES/general)
  - Deletes task branch on success
  - Unlocks dependent tasks
  - On merge fail → merge_pending (will retry)
```

## The Problem (why worktrees are needed)

Currently ALL tasks share ONE working directory (`~/vibepilot/`).
`CreateBranch()` does `git checkout -b` in that single directory.

When multiple agents run in parallel:
- Agent A checks out task/T001 → starts working
- Agent B checks out task/T002 → Agent A's branch is gone
- Agent A commits → commits to task/T002's branch by mistake
- "Which branch am I on" confusion

## The Gitree Strategy (designed but NOT yet wired)

From Gemini design session: Agent-Orchestrated Parallelism with worktrees.

### What worktree.go implements (13 functions, DEAD CODE -- nothing calls them):

| Function | Purpose | Where it SHOULD be called |
|---|---|---|
| `CreateWorktree(taskID, branch)` | Creates isolated checkout at `~/VibePilot-work/{taskID}` on `task/{id}-{slug}` branch | In `handleTaskAvailable`, INSTEAD of `CreateBranch()` |
| `BootstrapWorktree(path)` | Symlinks config, prompts, .context into worktree so agent has full context | Right after `CreateWorktree()` |
| `GetWorktreePath(taskID)` | Returns path for a task's worktree | When routing agent to its working dir |
| `ListWorktrees()` | Lists all active worktrees | Dashboard visibility |
| `PruneWorktrees()` | Removes stale worktrees | Periodic cleanup |
| `CleanAllWorktrees()` | Removes ALL worktrees | Governor shutdown |
| `ShadowMerge(source, target)` | Test-merges source into target WITHOUT committing. Returns conflict info | BEFORE real merge in `handleTaskTesting` |
| `shadowMergeFallback()` | Simpler merge check when ShadowMerge fails | Fallback |
| `RemoveWorktree(taskID)` | Removes a single worktree | After merge or on task failure |
| `TaskBranchName(taskID, slug)` | Generates branch name `task/{id8}-{slug}` | Branch naming |
| `isValidWorktree()` | Validates a worktree path | Internal |
| `parseWorktreeList()` | Parses `git worktree list` output | Internal |

### What SHOULD happen (the target flow):

```
handleTaskAvailable:
  1. CreateWorktree(taskID, branchName) → ~/VibePilot-work/{taskID}
  2. BootstrapWorktree(path) → symlinks config, prompts, .context
  3. Agent executes IN the worktree (not ~/vibepilot/)
  4. Agent commits to branch inside worktree
  5. Transition → review

handleTaskReview:
  - Same as now, but reads from worktree path
  - On fail: RemoveWorktree(taskID) instead of just DeleteBranch

handleTaskTesting:
  - ShadowMerge(taskBranch, targetBranch) FIRST to check for conflicts
  - If shadow merge clean: real merge + RemoveWorktree(taskID)
  - If shadow merge conflicts: merge_pending, human intervention
```

### BootstrapWorktree symlinks:
- config/*.json → governor config
- prompts/*.md → agent role templates  
- .context/ → knowledge layer
- .hermes.md → enforcement rules
- .governor_env → credentials (NOT symlinked, copied for security)

## Module Merge Flow (NOT yet implemented)

The TEST_MODULES/{slice} branches are a staging area. The full vision:
- Tasks merge to TEST_MODULES/{slice} after testing passes
- When all tasks for a module are complete → module QA
- Module branch merges to main after QA passes

This is partially in place: `getTargetBranch()` returns `TEST_MODULES/{slice}`
and the testing handler auto-merges there. But there's no module-level QA or
module-to-main merge logic yet.

## Config

```json
// system.json
"worktrees": {
  "enabled": false,           // ← currently off
  "base_path": "/home/vibes/VibePilot-work"
}
```

## What Needs Wiring (the gap)

1. **handleTaskAvailable**: Replace `CreateBranch()` with `CreateWorktree()` + `BootstrapWorktree()`
2. **handleTaskAvailable**: Pass worktree path (not ~/vibepilot/) to agent session
3. **handleTaskReview**: Read from worktree path, cleanup on fail
4. **handleTaskTesting**: Use `ShadowMerge()` before real merge
5. **handleTaskTesting**: Use `RemoveWorktree()` after merge (not just DeleteBranch)
6. **Flip `enabled: true`** in system.json
7. **Governor shutdown**: CleanAllWorktrees already wired in main.go
