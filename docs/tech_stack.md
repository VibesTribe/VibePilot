# VibePilot Tech Stack Decisions
## Documented: 2026-02-15

---

# CORE DECISIONS

## Backend Language
**Decision: Python**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Python | Existing codebase, Supabase works well, AI-friendly | Dynamic typing | ✅ CHOSEN |
| TypeScript | Type safety, better for AI to write | Rewrite everything | ❌ Rejected |

**Rationale:** We have significant Python code already. The orchestrator, runners, vault manager are all Python. Switching would cost months.

---

## Frontend Framework
**Decision: React + TypeScript + Vite**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| React + TS + Vite | From Vibeflow, AI-friendly, type safety | Learning curve | ✅ CHOSEN |
| Plain JavaScript | Simpler | No type safety | ❌ Rejected |
| Vue | Simpler than React | Less AI training data | ❌ Rejected |
| Svelte | Fast, simple | Less ecosystem | ❌ Rejected |

**Rationale:** Vibeflow already uses React + TypeScript + Vite. Proven pattern. TypeScript is statistically easier for AI to write correctly.

---

## Testing Framework

### Python (Backend)
**Decision: pytest**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| pytest | Standard, zero lock-in, excellent fixtures | None significant | ✅ CHOSEN |
| unittest | Built-in | More verbose | ❌ Rejected |
| nose | Legacy | Deprecated | ❌ Rejected |

### TypeScript (Frontend)
**Decision: Vitest**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Vitest | Fast, modern, Vite-native | Newer | ✅ CHOSEN |
| Jest | Standard | Slower, needs config | ❌ Rejected |

---

## E2E / Browser Automation
**Decision: browser-use (primary), Playwright (fallback)**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| browser-use | AI-native, Python, optimized for AI browsing | Newer project | ✅ CHOSEN |
| Playwright | Mature, reliable | Lower-level, more code | ✅ BACKUP |
| Puppeteer | Google-native | Chrome-only | ❌ Rejected |
| Selenium | Legacy | Slow, brittle | ❌ Rejected |

**Rationale:** browser-use is designed for AI agents. 78K stars. ChatBrowserUse model optimized for browsing. Playwright underneath for reliability.

---

## CI/CD
**Decision: GitHub Actions**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| GitHub Actions | Free tier, built into GitHub, already using | GitHub lock-in | ✅ CHOSEN |
| CircleCI | Feature-rich | Separate service, cost | ❌ Rejected |
| Jenkins | Self-hosted | Heavy, complex | ❌ Rejected |
| Travis CI | Simple | Expensive | ❌ Rejected |

**Rationale:** Already using GitHub. Vibeflow has 20+ workflows proving this works. Free tier sufficient.

---

## Email / Notifications
**Decision: browser-use + Gmail web (primary), Brevo (backup)**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| browser-use + Gmail | Same tool as courier, auth persisted, free | Slower than API | ✅ CHOSEN |
| Gmail API | Fast | OAuth token management | ❌ Rejected |
| SMTP + App Password | Simple | Separate system | ✅ BACKUP |
| Brevo | Already using | External service | ✅ BACKUP |

**Rationale:** Using browser-use for Gmail means one tool does both courier (ChatGPT, Claude) AND email. Auth persists in browser profile. No OAuth complexity.

---

## Database
**Decision: Supabase (PostgreSQL)**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Supabase | Already using, Postgres, built-in auth, real-time | External service | ✅ CHOSEN |
| Self-hosted Postgres | Full control | Maintenance burden | ❌ Rejected |
| MongoDB | Flexible schema | No SQL, harder to reason about | ❌ Rejected |

**Rationale:** Already invested in Supabase. Schema defined. Functions written. Works well.

---

## Hosting
**Decision: Hetzner VPS (target), GCE (current)**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Hetzner | ~€4/mo, reliable | Migration needed | ✅ TARGET |
| GCE | Already set up | $24/2wks = expensive | ⚠️ CURRENT |
| AWS | Feature-rich | Expensive, complex | ❌ Rejected |
| DigitalOcean | Simple | More expensive than Hetzner | ❌ Rejected |

**Rationale:** GCE costs $24/2wks. Hetzner is ~€4/mo. Migration planned after VibePilot functional.

---

## Version Control
**Decision: GitHub**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| GitHub | Already using, Actions, free | Microsoft-owned | ✅ CHOSEN |

**Rationale:** Already invested. Works well.

---

## Dashboard Hosting
**Decision: Vercel (preview), VPS (production)**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Vercel | Free, easy previews | External | ✅ PREVIEW |
| VPS | Full control, with backend | Need to serve static | ✅ PRODUCTION |
| GitHub Pages | Free | Static only | ❌ Rejected |

**Rationale:** Vercel for visual task previews (human review). Production dashboard on same VPS as backend.

---

# MODEL / PLATFORM DECISIONS

## Internal Governance
**Decision: GLM-5 (OpenCode) + Kimi CLI**

These NEVER go to web platforms:
- Consultant
- Planner
- Council
- Supervisor
- Watcher
- System Research

| Role | Primary | Backup |
|------|---------|--------|
| All governance | GLM-5 | Kimi CLI |

---

## Task Execution Priority

| Priority | Platform | Cost | When Used |
|----------|----------|------|-----------|
| 1 | Kimi CLI | Subscription | Parallel tasks, codebase access |
| 2 | OpenCode (GLM-5) | Subscription | Reasoning, fallback |
| 3 | Gemini API | Free tier | Research, simple tasks |
| 4 | DeepSeek API | $2 credit | Coding with cache |
| 5 | OpenRouter | $16 credit | ⚠️ LAST RESORT |

---

## Browser / Courier Model
**Decision: Gemini 2.0 Flash (primary), ChatBrowserUse (backup)**

| Option | Cost | Notes | Decision |
|--------|------|-------|----------|
| Gemini 2.0 Flash | Free tier | Native computer use | ✅ CHOSEN |
| ChatBrowserUse | $0.20/1M in, $2/1M out | Optimized but expensive output | ✅ BACKUP |
| Claude Computer Use | Pay per use | Expensive | ❌ Rejected |

**Rationale:** Gemini free tier with computer use API. ChatBrowserUse backup if Gemini struggles, but output cost is high ($2/1M) so use sparingly.

---

# ARCHITECTURE DECISIONS

## API Style
**Decision: REST + JSON**

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| REST + JSON | Simple, universal | Not real-time | ✅ CHOSEN |
| GraphQL | Flexible | Complex, overkill | ❌ Rejected |
| gRPC | Fast | Complex, overkill | ❌ Rejected |

---

## State Management
**Decision: Supabase (external state)**

| Principle | Implementation |
|-----------|----------------|
| All state in Supabase | Tasks, plans, runs, models, ratings |
| No local state | Stateless services, scalable |
| Session state in DB | Context tracking per session |

---

## Secret Management
**Decision: Vault (encrypted in Supabase)**

| Principle | Implementation |
|-----------|----------------|
| No .env files | Prompt injection risk |
| Encrypted in DB | Fernet encryption |
| Bootstrap keys only | SUPABASE_URL, SUPABASE_KEY, VAULT_KEY |
| All others in vault | Retrieved programmatically |

---

# SUMMARY TABLE

| Layer | Technology | Cost |
|-------|------------|------|
| Backend | Python | Free |
| Frontend | React + TypeScript + Vite | Free |
| Testing (Python) | pytest | Free |
| Testing (TS) | Vitest | Free |
| Browser/E2E | browser-use | Free (self-host) |
| CI/CD | GitHub Actions | Free tier |
| Email | Gmail via browser-use | Free |
| Database | Supabase | Free tier |
| Hosting | Hetzner VPS | ~€4/mo |
| Governance Models | GLM-5, Kimi | Subscriptions |
| Task Models | Kimi, Gemini, DeepSeek | Mix |
| Courier Model | Gemini 2.0 Flash | Free tier |

---

# MIGRATION NOTES

## Current → Target

| Current | Target | Status |
|---------|--------|--------|
| GCE ($24/2wks) | Hetzner (~€4/mo) | Planned |
| No dashboard | Dashboard from Vibeflow | Planned |
| GCE SMTP | Gmail via browser-use | Planned |

---

**Document Version:** 1.0
**Last Updated:** 2026-02-15
**Next Review:** When new options emerge (System Research will flag)
