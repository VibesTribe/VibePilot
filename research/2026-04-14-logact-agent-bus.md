# Research: LogAct (Meta) -- Agent Bus Architecture

**Date:** 2026-04-14
**Source:** https://www.youtube.com/watch?v=kEUXyH5Vfjc
**Relevance:** HIGH -- validates and extends VibePilot's existing architecture

## Core Concept
Deconstruct the agent into isolated components communicating via an immutable shared log (Agent Bus).
Each transition requires a commit on the log -- no steps can be skipped.

## 4-Stage Pipeline
1. **Inferring (Driver)**: LLM writes intent to log. Has ZERO execution power.
2. **Voting (Critics)**: Separate agents (rule-based + LLM) vote Yes/No on safety, security, logic.
3. **Deciding (Gatekeeper)**: Deterministic (non-AI) tallies votes. Commits or Aborts.
4. **Executing (Worker)**: Isolated sandbox reads commit, runs code.

## Key Results
- Neutralizes prompt injection (Voters isolated from malicious input)
- Semantic crash recovery (new agent reads log, sees where it failed)
- "Stupidity diagnosis" -- agent read its own slow code, rewrote it 290x faster
- Multi-agent swarms via gossip hub: 41% less tokens, 17% more output

## How VibePilot Already Maps
| LogAct | VibePilot |
|---|---|
| Agent Bus | Supabase (task states, RPCs) |
| Inferring | Planner agent |
| Voting | Supervisor + Council review |
| Deciding | Governor (deterministic Go code) |
| Executing | Task Runner (sandboxed) |
| Crash recovery | Gitree (task branches persist) |
| Gossip hub | Courier swarm (planned) |

## What We Should Adopt
1. **Intent logging** -- before execution, log what the agent INTENDS to do. Not just state transitions.
   Currently we go straight from "plan approved" to "executing." Should add an "intent" record.
2. **Safety Voter as cheap model** -- use a free-tier model to verify intent before execution.
   Different model than the one that generated the plan (cross-validation).
3. **Immutable task log** -- every action append-only, never overwritten. Currently Supabase
   updates rows in-place. Should add a task_events table with append-only log.
4. **Stupidity diagnosis loop** -- agent reads its own failed output, rewrites. Our revision
   loop does this but not as a formal log-read pattern.

## What We Should NOT Adopt
- Full cryptographic signing of transitions (overhead, we're not a bank)
- Separate process for each stage (too heavy for x220, keep it as Go routines)

## Priority: Medium
These are improvements to existing patterns, not new features. Implement after the
basic pipeline is working end-to-end.
