# PIF Multi-Model Council Review — June 30, 2026

## Models Consulted: Gemini 2.5 Flash, ChatGPT (GPT), Claude Sonnet 5, DeepSeek

---

## GEMINI (4,421 chars)

### Key Points:
1. **State Leak Problem**: Shared runtime memory buffers between projects. Fix: namespace-scoped runtimes/sandboxes per project.
2. **Secret Injection Bottleneck**: Agent could log env vars. Fix: proxy-based secret accessor (agent never sees the value).
3. **Skill Compatibility**: Swapping agents needs abstraction adapter layer (JSON-RPC for tools).
4. **Inter-project comms**: Event-driven bus with contract registry. Projects don't know each other.
5. **Scaling**: Manifest-to-deployment pipeline. vibepilot.toml → containerized stack. Add ResourceConstraints block.
6. **Agent agnosticism**: Capability-based routing (requires_reasoning_high) instead of declaring specific runtime.
7. **Content-addressable storage**: Deduplicate shared knowledge files.
8. **Self-healing audit log**: Journal directory for replay/rollback.
9. **Export validation**: export.sh should validate self-containment (no hardcoded paths).

---

## CHATGPT (5,244 chars) — Rated framework 9/10

### What's Strong:
- Project (not AI runtime) as unit of isolation — prevents context bleed, dependency drift
- Runtime abstraction prevents vendor lock-in
- Swappable infrastructure (database type in manifest)
- Shared metadata only dramatically reduces cross-contamination

### Weaknesses:
1. **Separate assets from operational data**: Backups can back up backups. Split project/ (src, docs, knowledge, memories) from runtime/ (logs, cache, backups).
2. **Memories need namespaces**: planner/, architect/, reviewer/, tester/, human/, execution/ — otherwise one giant noisy store.
3. **Knowledge vs research distinction**: Knowledge = stable facts (API docs, architecture). Research = temporary findings (benchmarks, comparisons). Research graduates into knowledge.
4. **Need import.sh / restore.sh**: Portability must be symmetric.
5. **Manifest versioning**: manifest_version, framework_min, framework_max fields.
6. **Execution-profile layer**: Between project and runtime — cost limits, latency preference, reasoning depth, retry policy, approval requirements.
7. **Memory retention policies**: Execution history ("Task 183 failed because...") is operational telemetry, not project knowledge. Needs pruning.

---

## CLAUDE (most critical, ~4,500 chars)

### Disagreements with Gemini/ChatGPT:
1. **Event bus contradicts isolation**: Any inter-project bus is a deliberate hole. Should be orchestrator-mediated only, opt-in, default-deny, logged as exception. Question whether projects need to talk at all.
2. **CAS across projects undermines isolation**: Shared store = side channel (timing, storage growth). Scope CAS within a project only.
3. **"Self-healing" audit log is a red flag**: Logs must be append-only. "Self-healing" implies rewriting entries = security anti-pattern.
4. **ChatGPT's 9/10 is not signal**: No rubric, no baseline. Ignore self-assigned scores.

### Gaps Neither Model Raised:
1. **NETWORK/EGRESS ISOLATION ABSENT**: Nothing addresses what a project's agent can reach over the network. Need per-project egress allowlisting (default: no internet). More important than sandbox isolation — process isolation without network isolation lets agents exfiltrate data.
2. **SECRETS GOVERNANCE STOPS AT READ**: Proxy accessor controls reading. Doesn't stop secrets ending up in research/, memories/, then shipped via export.sh. Need scrub/redaction pass on export.
3. **RUNTIME SWAP NOT SAFE WITHOUT ADAPTER**: Each runtime has own caching/temp behavior. Need compatibility contract each runtime must satisfy.
4. **NO CONCURRENCY/LOCKING MODEL**: What if orchestrator + human + two agents write simultaneously? Need transactional guarantees.
5. **NO COST/TOKEN ISOLATION**: Should attribute token spend per project. Can't bill/audit/transfer cost history without it.
6. **NO ACTOR PROVENANCE**: Every audit entry needs who/what acted (Hermes vs Claude vs human vs orchestrator).
7. **EXPORT INTEGRITY**: Need signing/checksums on export archive. Manifest hash check before restore runs.

**Priority ranking**: Fix network isolation and export-time secret scrubbing FIRST — those are where "fully isolated" and "transferable" both quietly fail at once.

---

## DEEPSEEK (7,661 chars — most technical)

### Disagreements:
1. **Event bus doesn't contradict isolation IF**: It's a command queue, not a router. Orchestrator is dumb mailman. Projects publish signed commands. Default deny. No IP/port knowledge.
2. **CAS is fine IF**: Content-defined chunking + per-project encryption keys. Different plaintext = different hash. Side channel eliminated.
3. **"Self-healing" means reconciliation, not mutation**: Immutable Merkle DAG. Healing = replay from genesis, compare state hash, trigger restore if mismatched.

### Missing Implementation Layers:
1. **Frozen Runtime + Semantic Adapter**: Don't swap binaries. Container runs frozen runtime version. Swap prompt/system-card and tool schemas at boot.
2. **Concurrency: Optimistic Locking**: base_revision in manifest. Stale revision = MERGE_REQUIRED with diff. Forces agents to be good citizens.
3. **Network Egress via SOCKS5 Sidecar**: Per-project proxy with allowlist in vibepilot.toml. iptables rules bundled in export.sh.

### Critical Changes:
1. **Export = Asymmetric Re-encryption**: Don't redact secrets — re-encrypt with target machine's public key. PII scrub via local LLM pre-flight.
2. **Staged Manifest**: [manifest.v1] immutable schema + [execution.prod]/[execution.dev] profiles with memory retention policies.
3. **Cost Isolation via Proxy**: Sidecar injects X-Project-ID header on API calls. Budget threshold = 429 + system prompt injection to halt agent.

### The Golden Rule:
"Treat the AI agent as a remote-controlled robot that only reads/writes files and makes HTTP calls through a guarded proxy. Do not give the agent control over the framework's runtime."

---

## CONSENSUS (All 4 agree)
- Manifest versioning needed from day one
- Need restore.sh not just export.sh
- Execution profiles / capability-based routing better than hardcoded agent names
- Memory retention/pruning policies needed

## MAJOR DISAGREEMENTS
- **Inter-project communication**: Gemini wants event bus. Claude says it contradicts isolation. DeepSeek says command queue with default-deny is fine. No consensus.
- **CAS/dedup**: Gemini wants it. Claude says it's a side channel. DeepSeek says encrypt-then-hash solves it. No consensus.
- **Audit log "self-healing"**: Gemini wants it. Claude says it's a red flag. DeepSeek says Merkle DAG reconciliation. No consensus.

## UNIQUE INSIGHTS (only one model raised)
- **Claude**: Network/egress isolation completely absent (CRITICAL)
- **Claude**: Secrets governance stops at read, not export (CRITICAL)
- **Claude**: Actor provenance in audit trail
- **ChatGPT**: Separate immutable assets from operational data
- **ChatGPT**: Knowledge vs research lifecycle distinction
- **DeepSeek**: Asymmetric re-encryption on export instead of redaction
- **DeepSeek**: SOCKS5 sidecar for network isolation
- **DeepSeek**: Cost isolation via proxy-injected headers
- **DeepSeek**: "Frozen runtime + semantic adapter" pattern
- **Gemini**: Capability-based routing instead of runtime names
