{
  "prd": {
    "version": "1.0",
    "project_type": "public",
    "title": "Playwright Aria Snapshot Enhancement for VibePilot Couriers",
    "tagline": "Faster, cheaper, more reliable AI-driven browser automation",
    "overview": "Replace screenshot-based vision with structured accessibility trees, add HAR recording for debugging, and adopt safe code manipulation patterns to improve courier performance and maintainability.",
    
    "user_vision": {
      "original_idea": "Adopt Playwright 1.60's ariaSnapshot with boxes option for AI consumption, add HAR recording, use noDefaults CDP connections, and apply str_replace pattern for code modifications.",
      "devices": ["desktop (Linux)"], 
      "primary_use_case": "VibePilot couriers interacting with web AI platforms via Playwright automation",
      "tech_level": "advanced"
    },
    
    "features": {
      "p0_must_have": [
        {
          "name": "Aria Snapshot with Boxes",
          "description": "Couriers use page.ariaSnapshot({ box: true }) to extract accessibility trees with spatial bounding box information for AI decision-making, replacing screenshot-based vision where DOM interaction suffices.",
          "user_value": "Reduces data transfer, faster AI processing, more stable across UI changes, lower cost (no vision model tokens needed)."
        },
        {
          "name": "HAR Recording for Debugging",
          "description": "Courier runs record HTTP archive (HAR) via tracing.startHar()/tracing.stopHar() to enable replay and analysis of failed interactions without re-running.",
          "user_value": "Decreases debugging time, prevents wasted API calls, improves reliability of courier troubleshooting."
        },
        {
          "name": "noDefaults CDP Connections",
          "description": "When attaching to existing Chrome sessions via Chrome DevTools Protocol, use { noDefaults: true } to avoid altering user's browser state (e.g., clearing cookies, resetting settings).",
          "user_value": "Preserves user's logged-in state and browser configuration, preventing unintended side effects during courier operations."
        },
        {
          "name": "str_replace Code Manipulation Pattern",
          "description": "Adopt exact-match/replace pattern for code modifications: fail if not unique, preventing silent errors from partial matches.",
          "user_value": "Increases safety and predictability of automated code changes, reducing bugs in courier self-modification routines."
        }
      ],
      "p1_should_have": [],
      "p2_nice_to_have": []
    },
    
    "tech_stack": {
      "selected": {
        "frontend": "N/A (backend/cli)",
        "backend": "Node.js with Playwright 1.60+",
        "database": "N/A",
        "deployment": "Local Hermes agent on X220"
      },
      "alternatives_considered": [
        {
          "option": "Continue screenshot-based vision",
          "rejected_because": "Expensive (vision model tokens), slow, brittle to UI changes"
        },
        {
          "option": "Pure DOM selectors",
          "rejected_because": "Fails when platforms require vision (canvas, custom controls); less robust than accessibility tree"
        },
        {
          "option": "External HAR tools (e.g., BrowserMob Proxy)",
          "rejected_because": "Adds complexity, requires separate process, not integrated with Playwright tracing"
        }
      ],
      "selection_rationale": "Leverages built-in Playwright 1.60 features already present in the system, zero additional dependencies, aligns with zero-lock-in and production-grade principles."
    },
    
    "competitor_analysis": {
      "existing_apps": [
        {
          "name": "Current VibePilot Courier Approach",
          "features": ["DOM selectors", "Text extraction", "Occasional screenshot vision"],
          "gaps": ["No structured AI-friendly output", "No built-in request tracing", "Risk of disturbing user browser state", "Unsafe code modification patterns"],
          "pricing": "N/A"
        }
      ],
      "differentiation": "Uses accessibility tree as primary AI interface (more stable than vision), integrates HAR recording natively, preserves user environment, and enforces safe code edits."
    },
    
    "architecture": {
      "overview": "Courier modules isolate Playwright interaction logic. Changes are localized to the browser automation layer where page actions are performed.",
      "components": [
        "Courier Base Class (handles Playwright lifecycle)",
        "Platform-Specific Action Implementations",
        "Code Manipulation Utilities"
      ],
      "data_flow": "AI decision → courier invokes page action → action uses ariaSnapshot for state observation → processes accessibility tree → decides next step → executes via Playwright methods → optionally records HAR trace",
      "swap_strategy": {
        "ai_model": "If accessibility tree insufficient for certain platforms, fallback to vision models via config flag (no code change needed)",
        "browser": "If Chrome becomes unsupported, switch to Firefox via Playwright browserType parameter",
        "hosting": "Couriers run locally; if remote execution needed, adapt via SSH/CDP without changing core logic"
      }
    },
    
    "security_requirements": [
      "No storage of HAR files containing sensitive data beyond debugging session",
      "HAR recording must be opt-in or temporary to avoid accidental credential leakage",
      "CDP connections must respect user's existing browser profile and not inject unauthorized extensions"
    ],
    "edge_cases": [
      "Platforms with iframes or shadow DOM: ensure ariaSnapshot traverses correctly",
      "Headless vs headed mode: noDefaults only applicable to headed CDP connections",
      "Empty or malformed accessibility trees: fallback to DOM selector with warning",
      "HAR recording failure: courier should continue and log error, not fail the entire run"
    ],
    "out_of_scope": [
      "Redesigning courier decision-making logic",
      "Changing underlying AI model couplings",
      "Adding new web platform integrations"
    ]
  },
  
  "research_notes": {
    "apps_analyzed": ["Current VibePilot courier implementation"],
    "gaps_identified": ["Lack of AI-friendly state observation", "No integrated request tracing", "Risky browser state mutation", "Error-prone code modifications"],
    "research_depth": "light"
  },
  
  "confidence": 0.95,
  "user_approved": false
}