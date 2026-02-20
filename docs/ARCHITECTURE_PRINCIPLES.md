# VibePilot Architecture: The Non-Negotiable Principles

**Read this. Understand this. Build only this.**

---

## THE CORE RULE

> **EVERYTHING IS ABSTRACT INTERFACE → CONFIG → CONCRETE IMPLEMENTATION**

NOT: Hardcoded → Specific Vendor → Broken when vendor changes

---

## 1. ROLE ≠ MODEL (The Hat ≠ The Head)

### What This Means

```
┌─────────────────────────────────────────────────────────────┐
│                    ROLE (The "Hat")                         │
│  - Council Member                                            │
│  - Supervisor                                                │
│  - Task Runner                                               │
│                                                              │
│  Defined: Behavior, responsibilities, expected output format │
│  NOT Defined: Which model executes it                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ IMPLEMENTS
                              ▼
┌─────────────────────────────────────────────────────────────┐
│              ABSTRACT INTERFACE (Contract)                  │
│  - BaseAgent.execute(task) → AgentResult                    │
│  - BaseRunner.execute(packet) → Result                      │
│  - BaseCouncilMember.review(doc) → Vote                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ CONFIG SELECTS
                              ▼
┌─────────────────────────────────────────────────────────────┐
│           CONCRETE IMPLEMENTATION (The "Engine")            │
│  - KimiRunner (today) → DeepSeekRunner (tomorrow)           │
│  - GLM5Agent (today) → ClaudeAgent (tomorrow)               │
│  - OpenCodeRunner (today) → OpenRouterRunner (tomorrow)     │
└─────────────────────────────────────────────────────────────┘
```

### In Code

**WRONG (Hardcoded):**
```python
def council_review(doc):
    # HARDCODED - breaks if Kimi unavailable
    result = kimi_cli.execute(prompt)  # ← VENDOR LOCK-IN
    return result
```

**RIGHT (Abstract):**
```python
def council_review(doc):
    # ABSTRACT - works with any available model
    runner = runner_pool.get_available()[0]  # ← ABSTRACT
    result = runner.execute(task_packet)     # ← INTERFACE
    return result
```

---

## 2. CONFIG-DRIVEN ARCHITECTURE

### The Config Files Are THE Source of Truth

**NOT:** Code decides which models to use
**YES:** Config declares what's available, code uses what's configured

```yaml
# config/models.yaml - THE TRUTH
models:
  - id: kimi-k2.5
    provider: kimi
    access_type: cli_subscription
    capabilities: [code, vision, long_context]
    routing: internal  # Can handle internal tasks
    
  - id: deepseek-chat
    provider: deepseek
    access_type: api
    api_key_ref: vault.deepseek_key  # Reference, not hardcoded
    capabilities: [code]
    routing: api  # API only, no browser
    
  - id: chatgpt-web
    provider: openai
    access_type: courier  # Web platform
    capabilities: [code, vision]
    routing: web  # Courier delivers
```

### Code Uses What's Configured

```python
# orchestrator.py - USES CONFIG, doesn't decide
class RunnerPool:
    def __init__(self):
        self.runners = load_models_from_config()  # ← CONFIG
        
    def get_runner(self, task_requirements):
        # Picks from AVAILABLE, not hardcoded list
        available = [r for r in self.runners if r.is_available()]
        return self.select_best(available, task_requirements)
```

---

## 3. ABSTRACT INTERFACES (The Contracts)

### Every Component Has an Interface

**BaseRunner (Abstract):**
```python
class BaseRunner(ABC):
    @abstractmethod
    def execute(self, task_packet: Dict) -> Dict:
        """
        Execute a task.
        
        Args:
            task_packet: Contains prompt, constraints, context
            
        Returns:
            {
                "status": "success" | "failed",
                "output": str,
                "errors": [...],
                "metadata": {...}
            }
        """
        pass
    
    @abstractmethod
    def is_available(self) -> bool:
        """Check if this runner can accept tasks."""
        pass
```

**KimiRunner (Concrete):**
```python
class KimiRunner(BaseRunner):
    """Concrete implementation using Kimi CLI."""
    
    def execute(self, task_packet: Dict) -> Dict:
        # Specific: Uses subprocess to call kimi CLI
        # But interface is abstract
        cmd = [self.kimi_path, "--yolo", "--prompt", task_packet["prompt"]]
        result = subprocess.run(cmd, ...)
        return self.format_result(result)
    
    def is_available(self) -> bool:
        # Check if Kimi CLI is installed and working
        return self._check_kimi_exists()
```

**DeepSeekRunner (Concrete):**
```python
class DeepSeekRunner(BaseRunner):
    """Concrete implementation using DeepSeek API."""
    
    def execute(self, task_packet: Dict) -> Dict:
        # Specific: Uses HTTP API
        # Same interface as KimiRunner
        response = requests.post(self.api_url, ...)
        return self.format_result(response)
    
    def is_available(self) -> bool:
        # Check API key and rate limits
        return self._check_api_access()
```

### The Magic

Both runners have the **SAME INTERFACE** but **DIFFERENT IMPLEMENTATIONS**.

Orchestrator doesn't know or care which is which:

```python
# orchestrator.py
runner = runner_pool.get_runner(task)
result = runner.execute(packet)  # ← Works for Kimi, DeepSeek, GLM, whatever
```

---

## 4. NO VENDOR LOCK-IN: THE TEST

### The One-Day Swap Test

For EVERY component, ask:

> "If [VENDOR] disappeared tomorrow, how long to swap?"

| Component | Vendor Today | Swap Time | How |
|-----------|--------------|-----------|-----|
| LLM for Council | Kimi | 5 minutes | Change config, restart |
| Task Execution | GLM-5 | 0 minutes | RunnerPool picks next available |
| Database | Supabase | 1 day | SQL export, connection string change |
| Hosting | GCE | 1 day | Docker containers, env vars |
| Git Provider | GitHub | 2 hours | Remote URL change |

If swap time > 1 day = **NOT MODULAR ENOUGH**

### How We Ensure This

**1. Abstract Interfaces**
- Every external service behind an interface
- Interface defines WHAT, implementation defines HOW

**2. Config-Not-Code**
- URLs, API keys, model names = CONFIG
- Business logic = CODE

**3. Feature Flags for Capabilities**
```python
# NOT: if model == "kimi-k2.5":
# YES: if "vision" in model.capabilities:

if runner.has_capability("vision"):
    # Do vision task
else:
    # Fallback to text-only
```

**4. No Vendor-Specific Code in Business Logic**

WRONG:
```python
def execute_task(task):
    if task["model"] == "kimi":
        return kimi_cli.run(task)
    elif task["model"] == "deepseek":
        return deepseek_api.call(task)
    # ... 20 more elifs
```

RIGHT:
```python
def execute_task(task):
    runner = runner_pool.get_runner_for_task(task)
    return runner.execute(task.packet)
```

---

## 5. CONCRETE EXAMPLES

### Example 1: Council Review (What I Just Built)

**The Interface:**
```python
class CouncilReview:
    def review(self, doc_path: str, lenses: List[str]) -> ReviewResult:
        """
        Review a document through multiple lenses.
        
        Returns votes, concerns, recommendations.
        Model-agnostic.
        """
        pass
```

**The Implementation:**
```python
def route_council_review(self, doc_path, lenses):
    # Get models from POOL (not hardcoded)
    available_models = self._get_council_models()
    
    # Execute in parallel
    reviews = {}
    for model_id in available_models:
        # Each model executes through RUNNER interface
        runner = self.runner_pool.get_runner(model_id)
        result = runner.execute(task_packet)
        reviews[model_id] = result
    
    # Aggregate (model-agnostic)
    return self._aggregate_reviews(reviews)
```

**Swap Models:** Change `config/models.yaml`, restart. Done.

### Example 2: Task Execution

**The Interface:**
```python
class TaskExecutor:
    def execute(self, task: Task) -> ExecutionResult:
        pass
```

**The Config:**
```yaml
runners:
  - name: cli-runners
    type: internal
    models: [kimi-k2.5, opencode]
    priority: 1
    
  - name: api-runners
    type: external
    models: [deepseek-chat, gemini-pro]
    priority: 2
    cost_limit: 0.01  # per 1k tokens
```

**The Code:**
```python
def dispatch_task(task):
    # Pool decides based on config + availability
    runner = runner_pool.get_best_for_task(task)
    
    # Interface is abstract
    result = runner.execute(task.to_packet())
    
    # Store result (model-agnostic)
    task_store.save_result(task.id, result)
```

**Swap Execution Backend:** Change config, restart. Done.

### Example 3: Database (Supabase Today, Postgres Tomorrow)

**The Interface:**
```python
class TaskStore(ABC):
    @abstractmethod
    def get_task(self, task_id: str) -> Task:
        pass
    
    @abstractmethod
    def save_task(self, task: Task) -> bool:
        pass
```

**Supabase Implementation (Today):**
```python
class SupabaseTaskStore(TaskStore):
    def get_task(self, task_id: str) -> Task:
        result = self.supabase.table("tasks").select("*").eq("id", task_id).execute()
        return Task.from_dict(result.data[0])
```

**Postgres Implementation (Tomorrow):**
```python
class PostgresTaskStore(TaskStore):
    def get_task(self, task_id: str) -> Task:
        with self.conn.cursor() as cur:
            cur.execute("SELECT * FROM tasks WHERE id = %s", (task_id,))
            return Task.from_dict(cur.fetchone())
```

**Swap:** Change config `task_store: postgres`, restart. Done.

---

## 6. WHAT GLM-5 WAS DOING WRONG

### The Problem: Adding Hardcoded Supabase RPC

GLM-5 wanted to add:
```sql
-- vibes_submit_idea RPC
CREATE FUNCTION vibes_submit_idea(p_idea TEXT, p_user_id UUID)
RETURNS TABLE (...)  -- Hardcoded to Supabase
```

This is **WRONG** because:
1. Hardcoded to Supabase (vendor lock-in)
2. Hardcoded RPC name in code
3. If we swap to Postgres/MySQL = broken
4. Schema change required for new entry method

### The Right Way: Use Existing Abstraction

**Entry already exists:** `orchestrator.process_idea()`

**Vibes panel calls:**
```javascript
// Frontend (VibesChatPanel.tsx)
const submitIdea = async (ideaText) => {
    // Call existing Python function via API wrapper
    const response = await fetch('/api/submit-idea', {
        method: 'POST',
        body: JSON.stringify({ idea: ideaText })
    });
    return response.json();
};
```

**API wrapper (optional, thin):**
```python
# api/routes.py - THIN wrapper, not new logic
@app.post("/api/submit-idea")
def api_submit_idea(request: IdeaRequest):
    # Just calls existing orchestrator method
    result = orchestrator.process_idea(request.idea)
    return result
```

**Benefits:**
- No Supabase schema changes
- Works with any backend (REST, gRPC, etc.)
- Orchestrator handles all logic
- Swappable transport layer

---

## 7. THE CHECKLIST

Before committing ANY code:

- [ ] **No hardcoded vendor names** ("kimi", "openai", etc.)
- [ ] **Abstract interface** exists for the component
- [ ] **Config file** can change implementation
- [ ] **Swap test**: Can I swap vendors in < 1 day?
- [ ] **No vendor-specific types** in function signatures
- [ ] **Feature detection** not version detection
- [ ] **Fallback paths** exist for every external dependency

---

## 8. SUMMARY

```
┌─────────────────────────────────────────────────────────────┐
│                    VIBEPILOT ARCHITECTURE                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ROLES (Hats)          ← Abstract responsibilities          │
│  ↓                                                           │
│  INTERFACES (Contracts) ← Abstract method signatures        │
│  ↓                                                           │
│  CONFIG (Selection)     ← Declares what's available         │
│  ↓                                                           │
│  IMPLEMENTATIONS        ← Concrete vendor code              │
│                                                              │
│  KEY RULE: Code only knows INTERFACES                       │
│  Vendor specifics live in IMPLEMENTATIONS + CONFIG          │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

**This is not optional. This is not "nice to have". This is VibePilot.**

Every violation is technical debt. Every shortcut is a future rewrite.

**Build it right, or don't build it.**
