# VibePilot Architecture Diagram

## System Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              VIBEPILOT v1.2                                  │
│                    "Sovereign AI Execution Engine"                           │
└─────────────────────────────────────────────────────────────────────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│              │     │              │     │              │     │              │
│     IDEA     │────▶│   CONSULTANT │────▶│    PLANNER   │────▶│   COUNCIL    │
│   (Human)    │     │  (PRD Gen)   │     │ (Atomic Plan)│     │  (Review)    │
│              │     │              │     │              │     │              │
└──────────────┘     └──────────────┘     └──────────────┘     └──────┬───────┘
                                                                       │
                                                           ┌───────────┴───────────┐
                                                           │                       │
                                                           │ APPROVED?             │
                                                           │                       │
                                                           └───────────┬───────────┘
                                                                       │
                              ┌────────────────────────────────────────┴────────────────────────────────────────┐
                              │                                                                                 │
                              ▼                                                                                 │
                    ┌─────────────────┐                                                                        │
                    │    DIRECTOR     │                                                                        │
                    │    (Vibes)      │                                                                        │
                    │                 │                                                                        │
                    │ • Task Queue    │                                                                        │
                    │ • Model Select  │                                                                        │
                    │ • Budget Check  │                                                                        │
                    │ • Retry Policy  │                                                                        │
                    └────────┬────────┘                                                                        │
                             │                                                                                  │
              ┌──────────────┴──────────────┬──────────────────────────┐                       │
              │                             │                          │                        │
              ▼                             ▼                          ▼                        │
    ┌──────────────────┐         ┌──────────────────┐       ┌──────────────────┐               │
    │   CLI RUNNER     │         │  VISION RUNNER   │       │  WEB COURIER     │               │
    │   (Primary)      │         │   (Secondary)    │       │   (Tertiary)     │               │
    │                  │         │                  │       │                  │               │
    │ • OpenCode CLI   │         │ • Playwright     │       │ • Human Courier  │               │
    │ • Kimi CLI       │         │ • GLM-4V         │       │ • Carries packet │               │
    │ • Ephemeral      │         │ • DOM Fingerprint│       │   to web AI      │               │
    │                  │         │                  │       │ • Returns result │               │
    └────────┬─────────┘         └────────┬─────────┘       │   + chat URL     │               │
             │                            │                  └────────┬─────────┘               │
             │                            │                           │                         │
             └──────────────┬─────────────┴───────────────────────────┘                         │
                            │                                                                │
                            ▼                                                                │
                  ┌─────────────────┐                                                        │
                  │   EXECUTION     │                                                        │
                  │    RESULT       │                                                        │
                  └────────┬────────┘                                                        │
                           │                                                                 │
                           ▼                                                                 │
              ┌───────────────────────┐                                                     │
              │      SUPERVISOR       │                                                     │
              │                       │                                                     │
              │ • Review Output       │                                                     │
              │ • Check Tests         │─────────────────────────────────────────────────────┐│
              │ • Validate PRD Align  │                                                     ││
              │ • Council if Needed   │                                                     ││
              └───────────┬───────────┘                                                     ││
                          │                                                                  ││
              ┌───────────┴───────────┐                                                     ││
              │                       │                                                     ││
              ▼                       ▼                                                     ││
        ┌───────────┐          ┌───────────┐                                               ││
        │  PASSED   │          │  FAILED   │                                               ││
        └─────┬─────┘          └─────┬─────┘                                               ││
              │                      │                                                     ││
              │                      ▼                                                     ││
              │            ┌──────────────────┐                                            ││
              │            │ FAILURE ANALYSIS │                                            ││
              │            │                  │                                            ││
              │            │ • Classify Error │                                            ││
              │            │ • Check Attempts │                                            ││
              │            │ • Reassign/Split │                                            ││
              │            │ • Escalate (3x)  │                                            ││
              │            └────────┬─────────┘                                            ││
              │                     │                                                      ││
              │                     └──────────────────────┐                               ││
              │                                            │                               ││
              ▼                                            ▼                               ││
    ┌─────────────────┐                          ┌─────────────────┐                      ││
    │  MAINTENANCE    │                          │   REASSIGNED    │                      ││
    │     MERGE       │                          │   (Loop Back)   │                      ││
    │                 │                          └─────────────────┘                      ││
    │ • Feature Branch│                                                                   ││
    │ • Tests Pass    │                                                                   ││
    │ • PR Created    │                                                                   ││
    │ • Merge to Main │                                                                   ││
    │ • Update ROI    │                                                                   ││
    └────────┬────────┘                                                                   ││
             │                                                                            ││
             ▼                                                                            ││
    ┌─────────────────┐                                                                   ││
    │    COMPLETE     │                                                                   ││
    │                 │                                                                   ││
    │ • Task Merged   │                                                                   ││
    │ • Branch Deleted│                                                                   ││
    │ • ROI Logged    │                                                                   ││
    │ • Dependencies  │◀──────────────────────────────────────────────────────────────────┘│
    │   Unlocked      │                                                                    │
    └─────────────────┘                                                                    │
                                                                                           │
                                                                                           │
┌──────────────────────────────────────────────────────────────────────────────────────────┘
│
│
│   ══════════════════════════════════════════════════════════════════════════════════════
│   STATE LAYER (SUPABASE) - Source of Truth
│   ══════════════════════════════════════════════════════════════════════════════════════
│
│   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   │   tasks     │  │task_packets │  │   models    │  │ task_runs   │
│   │             │  │             │  │             │  │             │
│   │ • status    │  │ • prompt    │  │ • platform  │  │ • courier   │
│   │ • result    │  │ • tech_spec │  │ • limits    │  │ • chat_url  │
│   │ • attempts  │  │ • version   │  │ • usage     │  │ • result    │
│   │ • deps      │  │             │  │ • status    │  │ • tokens    │
│   └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘
│
└───────────────────────────────────────────────────────────────────────────────────────────


══════════════════════════════════════════════════════════════════════════════════════════════
COUNCIL GOVERNANCE (Triggered by Supervisor)
══════════════════════════════════════════════════════════════════════════════════════════════

         ┌─────────────────────────────────────────────────────────────┐
         │                    COUNCIL REVIEW                           │
         │                                                             │
         │  Triggered For:                                             │
         │  • New Plans (before execution)                             │
         │  • System Updates / Architecture Changes                    │
         │  • New Feature Proposals                                    │
         │  • New Tool/Model Integration                               │
         │                                                             │
         └───────────────────────────┬─────────────────────────────────┘
                                     │
                  ┌──────────────────┼──────────────────┐
                  │                  │                  │
                  ▼                  ▼                  ▼
         ┌───────────────┐  ┌───────────────┐  ┌───────────────┐
         │  STRUCTURAL   │  │SPECIFICATION  │  │  FEASIBILITY  │
         │   VALIDATOR   │  │   PRECISION   │  │   ANALYST     │
         │               │  │   REVIEWER    │  │               │
         │ • Tech stack  │  │ • PRD align   │  │ • Can it be   │
         │ • Docker      │  │ • Gaps        │  │   built?      │
         │ • Multi-stage │  │ • Conflicts   │  │ • Resources   │
         │ • Non-root    │  │ • Edge cases  │  │ • Timeline    │
         │               │  │               │  │ • Risk        │
         └───────┬───────┘  └───────┬───────┘  └───────┬───────┘
                 │                  │                  │
                 └──────────────────┼──────────────────┘
                                    │
                                    ▼
                         ┌───────────────────┐
                         │  COUNCIL RESULT   │
                         │                   │
                         │  • APPROVED       │
                         │  • REVISION NEEDED│
                         │  • BLOCKED        │
                         └───────────────────┘

    NOTE: Each council member is a DIFFERENT MODEL (prevents blind spots)
          No agent-to-agent chat (token efficiency)
          Input: System Summary + PRD + Plan + Role Prompt


══════════════════════════════════════════════════════════════════════════════════════════════
EXECUTION HANDS (Only 2, No Third Without Council)
══════════════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────┐    ┌─────────────────────────────────────┐
│         CLI RUNNER (Primary)        │    │       VISION RUNNER (Secondary)     │
│                                     │    │                                     │
│  Environment:                       │    │  Environment:                       │
│  • GitHub Actions (ephemeral)       │    │  • GitHub Actions                   │
│  • OpenCode CLI (GLM subscription)  │    │  • Playwright                       │
│  • Optional: Kimi CLI               │    │  • GLM-4V reasoning                 │
│                                     │    │                                     │
│  Lifecycle:                         │    │  Used Only When:                    │
│  1. Spin up runner                  │    │  • Platform has no API              │
│  2. Install CLI dynamically         │    │  • Web interaction required         │
│  3. Execute instruction packet      │    │                                     │
│  4. Validate exit code              │    │  Safety:                            │
│  5. Run lint/tests                  │    │  • DOM fingerprint check            │
│  6. Commit to feature branch        │    │  • Exit on UI mismatch              │
│  7. Push PR                         │    │  • Retry classification             │
│  8. Destroy runner                  │    │                                     │
│                                     │    │  Repeated Failures → Escalate       │
└─────────────────────────────────────┘    └─────────────────────────────────────┘


══════════════════════════════════════════════════════════════════════════════════════════════
RETRY & FAILURE CLASSIFICATION
══════════════════════════════════════════════════════════════════════════════════════════════

    ERROR TYPE              RETRY POLICY           ACTION
    ─────────────────────────────────────────────────────────────
    MODEL_ERROR         →   Retry up to 3         → Same model or switch
    NETWORK_ERROR       →   Retry up to 3         → Exponential backoff
    PLATFORM_ERROR      →   Retry 2, then         → Escalate to Council
    LOGIC_ERROR         →   Escalate immediately  → Human review
    CLI_ERROR           →   Retry up to 2         → Check CLI health
    TIMEOUT (>30 min)   →   Kill + classify       → Reassign or escalate


══════════════════════════════════════════════════════════════════════════════════════════════
BUDGET ENFORCEMENT
══════════════════════════════════════════════════════════════════════════════════════════════

    ┌─────────────────────────────────────────────────────────┐
    │                    DIRECTOR ENFORCES                     │
    │                                                         │
    │  • Monthly token ceiling                                │
    │  • Per-model usage cap                                  │
    │  • Auto-throttle when 80% threshold exceeded            │
    │  • Model ranking by cost-per-success (ROI)              │
    │                                                         │
    │  If monthly ceiling reached:                            │
    │  → Pause new non-critical tasks                         │
    │  → Log event                                            │
    │  → Alert human                                          │
    └─────────────────────────────────────────────────────────┘


══════════════════════════════════════════════════════════════════════════════════════════════
SWAPPABILITY (All Can Be Replaced)
══════════════════════════════════════════════════════════════════════════════════════════════

    COMPONENT          REPLACEMENT RULES
    ──────────────────────────────────────────────────────────
    GLM               → Pass Council review
    Kimi              → No execution lifecycle change
    LiteLLM           → No governance removal
    Supabase          → No persistent compute
    GitHub Actions    → State must migrate cleanly
    Angie/Nginx       → Gateway only, no logic

    ARCHITECTURE ASSUMPTION: All vendors will fail eventually


══════════════════════════════════════════════════════════════════════════════════════════════
FORBIDDEN ACTIONS
══════════════════════════════════════════════════════════════════════════════════════════════

    ❌ Silent architecture changes
    ❌ Self-modifying orchestration logic
    ❌ Infinite retry loops
    ❌ Direct execution without PRD
    ❌ Manual hotfix edits outside pipeline
    ❌ Model-driven schema mutation
    ❌ Agent-to-agent chat (token waste)
    ❌ Same model reviewing its own work


══════════════════════════════════════════════════════════════════════════════════════════════
DEFINITION OF DONE (Per Task)
══════════════════════════════════════════════════════════════════════════════════════════════

    □ Code compiles
    □ Tests pass
    □ PR created
    □ Supervisor validation logged
    □ ROI metrics recorded
    □ Confidence score stored
    □ Branch merged
    □ Branch deleted
    □ Dependencies unlocked
