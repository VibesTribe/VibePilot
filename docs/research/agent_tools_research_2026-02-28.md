# Research: Agent Tools and Frameworks

**Date:** 2026-02-28
**Session:** 37

---

## SUMMARY

| Project | Stars | Type | Key Insight |
|---------|-------|------|-------------|
| **agent-browser** | 16,694 | Browser automation | Fast Rust CLI for AI browser control |
| **SkillRL** | 583 | Learning framework | Skills from trajectories, 10-20% token compression |
| **pi.dev** | N/A | Coding agent | Minimal, extendable, TypeScript packages |
| **SpecWeave** | N/A | Spec-driven framework | Multi-agent teams, persistent memory, autonomous execution |

---

## 1. agent-browser (Vercel Labs)

**What it is:** Headless browser automation CLI for AI agents. Fast Rust binary.

**Key Features:**
- Accessibility tree with refs (best for AI)
- Screenshot with element annotations
- Click, fill, type, scroll, drag, upload
- Traditional CSS selectors OR semantic refs
- Sub-millisecond parsing overhead

**Example Usage:**
```bash
agent-browser open example.com
agent-browser snapshot        # Get accessibility tree with @refs
agent-browser click @e2       # Click by ref
agent-browser fill @e3 "text" # Fill by ref
agent-browser screenshot --annotate
```

**What VibePilot Can Learn:**
- Accessibility tree > raw HTML for AI understanding
- Ref-based interaction is cleaner than CSS selectors
- Screenshot annotation helps visual debugging
- Rust binary for performance (we use Go, similar philosophy)

**Applicability to VibePilot:**
- Courier could use agent-browser for web platform execution
- Accessibility tree would help AI understand web pages
- Screenshot annotations could go to visual tester

**Concerns:**
- Adds Chromium dependency
- Node.js/Rust stack (we're Go)
- Could be called as external CLI from Courier

---

## 2. SkillRL

**What it is:** Framework for LLM agents to learn reusable skills from past experiences.

**Key Concepts:**

1. **Experience-based Skill Distillation**
   - Successful trajectories → strategic patterns
   - Failed trajectories → lessons learned

2. **Hierarchical SkillBank**
   - General Skills: Universal strategic guidance
   - Task-Specific Skills: Category-level heuristics

3. **Recursive Skill Evolution**
   - Skill library co-evolves with agent policy
   - Analyzes validation failures

4. **Context Efficiency**
   - 10-20% token compression vs raw trajectory storage
   - More reasoning utility

**What VibePilot Can Learn:**
- Our `planner_learned_rules` table is similar to SkillBank
- We should distill failures into lessons (we do this partially)
- Hierarchical skills: general vs task-specific
- Recursive evolution: skills improve as agents improve

**Already Doing:**
- `planner_learned_rules` = skill library
- `failure_records` = failure analysis
- `problem_solutions` = what worked

**Could Improve:**
- Compress failure patterns into concise lessons
- Hierarchical organization (general vs task-specific)
- Recursive skill evolution (update skills as agents improve)

---

## 3. pi.dev

**What it is:** Minimal terminal coding harness. Extendable via TypeScript.

**Philosophy:**
- "There are many coding agents, but this one is mine"
- Minimal defaults, extendable
- Four modes: interactive, print/JSON, RPC, SDK

**Extension Points:**
- TypeScript extensions
- Skills (reusable capabilities)
- Prompt templates
- Themes
- Packages via npm or git

**What VibePilot Can Learn:**
- Skills as packages (our agent prompts are similar)
- Multiple modes (interactive, RPC, SDK)
- Shareable via git (we already do this)

**Already Doing:**
- Agent prompts as .md files in git
- Configurable routing and agents

**Could Improve:**
- Package system for skills/prompts
- RPC mode for external integration
- SDK for programmatic use

---

## 4. SpecWeave

**What it is:** Spec-driven framework for AI coding agents with persistent memory.

**Key Features:**

1. **Multi-Agent Teams**
   - PM, Architect, QA, Security, DevOps agents
   - Powered by Claude Opus 4.6

2. **Persistent Memory**
   - AI learns from corrections
   - Full context across sessions
   - "Fix once — remembered permanently"

3. **Autonomous Execution**
   - `/sw:auto` runs for hours without intervention
   - Implements, tests, fixes, documents

4. **Quality Gates**
   - "Code Grill" reviews every change
   - Tests, docs, acceptance criteria

5. **Living Documentation**
   - Specs, ADRs, runbooks sync automatically

6. **Bidirectional Sync**
   - GitHub Issues, JIRA, Azure DevOps

**Three Commands:**
```
/sw:increment → Define feature (spec, plan, tasks)
/sw:auto      → Execute autonomously
/sw:done      → Validate, sync, deploy
```

**What VibePilot Can Learn:**

| SpecWeave Feature | VibePilot Equivalent | Gap |
|-------------------|---------------------|-----|
| Multi-agent teams | Planner, Supervisor, Council | ✅ Have this |
| Persistent memory | planner_learned_rules | ⚠️ Partial |
| Autonomous execution | Event-driven flow | ✅ Have this |
| Quality gates | Supervisor review | ✅ Have this |
| Living docs | Git commits | ⚠️ Could auto-update docs |
| Bidirectional sync | None | ❌ Missing |

**SpecWeave's Key Insight:**

"Fix once — remembered permanently"

This is what our learning system should do. Every correction becomes a permanent skill.

---

## ACTIONABLE INSIGHTS FOR VIBEPILOT

### High Priority

| From | What to Implement | Why |
|------|-------------------|-----|
| **SkillRL** | Compress failure patterns into concise lessons | 10-20% token efficiency |
| **SpecWeave** | "Fix once, remember forever" | Learning persistence |
| **agent-browser** | Accessibility tree for web interaction | Better AI understanding |

### Medium Priority

| From | What to Implement | Why |
|------|-------------------|-----|
| **SpecWeave** | Living documentation auto-update | Docs never drift |
| **pi.dev** | Skills as packages | Shareable capabilities |
| **SpecWeave** | Bidirectional sync (GitHub/JIRA) | External tool integration |

### Low Priority

| From | What to Implement | Why |
|------|-------------------|-----|
| **pi.dev** | SDK/RPC modes | External integration |
| **agent-browser** | Rust binary for browser automation | Performance |

---

## SPECIFIC RECOMMENDATIONS

### 1. Skill Compression (from SkillRL)

Current: `planner_learned_rules` stores full text
Better: Compress into concise patterns

```
Before (200 tokens):
"The task failed because the model truncated output when given more than 50K context. 
This happened 3 times with Gemini Flash on planning tasks. Consider splitting tasks 
or using a model with larger context window."

After (30 tokens):
"Truncation on >50K context → split task or use larger context model"
```

### 2. Accessibility Tree for Web (from agent-browser)

When Courier interacts with web platforms, use accessibility tree instead of raw HTML:
- Cleaner for AI to understand
- Ref-based interaction
- Screenshot annotations for debugging

### 3. Living Documentation (from SpecWeave)

Auto-update docs after tasks complete:
- Update README if APIs changed
- Update CLAUDE.md if architecture changed
- Update relevant docs based on task type

### 4. Bidirectional Sync (from SpecWeave)

- GitHub Issues created from tasks
- Task status updates GitHub Issues
- JIRA integration for enterprise

---

## QUESTIONS FOR HUMAN

1. **Skill compression:** Should we compress learned rules into concise patterns?

2. **agent-browser:** Should Courier use agent-browser for web automation?

3. **Living docs:** Should we auto-update documentation?

4. **Bidirectional sync:** Is GitHub/JIRA sync needed?

---

## CONCLUSION

VibePilot already has much of what these systems offer:
- Multi-agent teams ✅
- Learning from failures ✅
- Event-driven autonomous execution ✅
- Quality gates (Supervisor) ✅

Key improvements from research:
1. **Compress skills** (SkillRL) - 10-20% token efficiency
2. **Accessibility tree** (agent-browser) - Better web interaction
3. **Living docs** (SpecWeave) - Auto-update documentation
4. **Fix once, remember forever** (SpecWeave) - Permanent learning

No major architectural changes needed. These are enhancements to existing systems.
