#!/usr/bin/env python3
"""
VibePilot Free Model Sync
- Fetches current free models from OpenRouter API
- Applies ban list (models known to cause issues for our use case)
- Applies preview/volatility flags
- Outputs a routing.json-compatible priority list
- Writes a diff report for the knowledgebase
"""

import json
import sys
import os
from datetime import datetime, timezone

OPENROUTER_API = "https://openrouter.ai/api/v1/models"

# Models we KNOW cause problems for agentic coding tasks
BAN_LIST = {
    "nvidia/nemotron-3-super-120b-a12b:free": "Mamba vibe-loop hangs on agentic tasks",
    "nvidia/nemotron-nano-9b-v2:free": "Poor instruction following, repetitive output",
    "nvidia/nemotron-nano-12b-v2-vl:free": "Poor instruction following, repetitive output",
    "nvidia/nemotron-3-nano-30b-a3b:free": "Nemotron family - inconsistent quality",
    "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free": "Nemotron family - inconsistent quality",
    "meta-llama/llama-3.2-3b-instruct:free": "Too small for agentic tasks, hallucination prone",
    "meta-llama/llama-3.3-70b-instruct:free": "Llama 3 instruct variants - poor tool use",
    "liquid/lfm-2.5-1.2b-instruct:free": "Too small for agentic tasks",
    "liquid/lfm-2.5-1.2b-thinking:free": "Too small for agentic tasks",
    "baidu/qianfan-ocr-fast:free": "OCR-only, not general purpose",
    "google/gemma-3n-e2b-it:free": "Too small (2B), agentic tasks fail",
    "google/gemma-3n-e4b-it:free": "Too small (4B), agentic tasks fail",
}

# Models that are "preview" -- likely to expire or change in 30-90 days
PREVIEW_FLAG = {
    "google/lyria-3-pro-preview": "Vision multimodal preview - HIGH expiration risk",
    "google/lyria-3-clip-preview": "Vision clip preview - HIGH expiration risk",
    "tencent/hy3-preview:free": "Preview label - MEDIUM expiration risk",
}

# Priority ordering for VibePilot agentic tasks (best first)
# Models not in this list get appended alphabetically after
PRIORITY_ORDER = [
    # Tier 1: Best free models for agentic coding
    "qwen/qwen3-coder:free",           # 480B MoE, best free coder
    "qwen/qwen3-next-80b-a3b-instruct:free",  # Strong generalist
    "google/gemma-4-31b-it:free",      # Good instruction following
    "google/gemma-4-26b-a4b-it:free",  # Efficient MoE
    "minimax/minimax-m2.5:free",       # Strong coding, 197K context
    # Tier 2: Solid alternatives
    "google/gemma-3-27b-it:free",      # Good mid-size
    "nousresearch/hermes-3-llama-3.1-405b:free",  # Large model
    "openai/gpt-oss-120b:free",        # OpenAI free offering
    "openai/gpt-oss-20b:free",         # Smaller OpenAI
    "openrouter/free",                  # Router - distributes across free
    "inclusionai/ling-2.6-1t:free",    # 1T parameter MoE, tools
    # Tier 3: Niche/specialized
    "poolside/laguna-m.1:free",        # Code-focused
    "poolside/laguna-xs.2:free",       # Smaller code model
    "cognitivecomputations/dolphin-mistral-24b-venice-edition:free",  # Uncensored
    "google/gemma-3-12b-it:free",      # Mid-size
    "google/gemma-3-4b-it:free",       # Small but capable
    "z-ai/glm-4.5-air:free",           # Zhipu GLM
]


def fetch_free_models():
    """Fetch free models from OpenRouter API."""
    import urllib.request
    req = urllib.request.Request(OPENROUTER_API)
    req.add_header("User-Agent", "VibePilot-ModelSync/1.0")
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = json.loads(resp.read())
    
    all_models = data.get("data", [])
    free = []
    for m in all_models:
        pricing = m.get("pricing", {})
        prompt_price = pricing.get("prompt", "1")
        completion_price = pricing.get("completion", "1")
        if prompt_price == "0" and completion_price == "0":
            free.append({
                "id": m["id"],
                "name": m.get("name", m["id"]),
                "context_length": m.get("context_length", 0),
                "capabilities": [],
            })
            # Extract capabilities
            arch = m.get("architecture", {})
            if arch.get("modality") == "multimodal" or m.get("supports_vision"):
                free[-1]["capabilities"].append("vision")
            # Check for tool support from top_provider
            tp = m.get("top_provider", {})
            if tp.get("tool_use"):
                free[-1]["capabilities"].append("tools")
    
    return free


def build_report(free_models):
    """Build a sync report."""
    now = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    
    banned = []
    preview = []
    approved = []
    
    for m in free_models:
        mid = m["id"]
        entry = {
            "id": mid,
            "name": m["name"],
            "context": m["context_length"],
            "caps": m["capabilities"],
        }
        
        if mid in BAN_LIST:
            entry["reason"] = BAN_LIST[mid]
            banned.append(entry)
        elif mid in PREVIEW_FLAG:
            entry["risk"] = PREVIEW_FLAG[mid]
            preview.append(entry)
            approved.append(entry)
        else:
            approved.append(entry)
    
    return {
        "timestamp": now,
        "total_free": len(free_models),
        "banned_count": len(banned),
        "preview_count": len(preview),
        "approved_count": len(approved),
        "banned": banned,
        "preview": preview,
        "approved": approved,
    }


def build_routing_priority(approved_models):
    """Build a routing.json-compatible priority list."""
    # Sort by our priority order first, then alphabetically for unknowns
    priority_map = {mid: i for i, mid in enumerate(PRIORITY_ORDER)}
    
    sorted_models = sorted(approved_models, key=lambda m: (
        priority_map.get(m["id"], 999),
        m["id"]
    ))
    
    return [m["id"] for m in sorted_models]


def main():
    print("VibePilot Free Model Sync")
    print("=" * 40)
    
    try:
        free_models = fetch_free_models()
    except Exception as e:
        print(f"ERROR: Failed to fetch models: {e}")
        sys.exit(1)
    
    print(f"Fetched {len(free_models)} free models from OpenRouter")
    
    report = build_report(free_models)
    print(f"\nBanned: {report['banned_count']} models")
    for b in report["banned"]:
        print(f"  - {b['id']}: {b['reason']}")
    
    print(f"\nPreview (volatile): {report['preview_count']} models")
    for p in report["preview"]:
        print(f"  ! {p['id']}: {p.get('risk', 'unknown')}")
    
    print(f"\nApproved: {report['approved_count']} models")
    
    # Build routing priority
    priority = build_routing_priority(report["approved"])
    print(f"\nRouting priority ({len(priority)} models):")
    for i, mid in enumerate(priority):
        print(f"  {i+1}. {mid}")
    
    # Write outputs
    report_dir = os.environ.get("REPORT_DIR", "/tmp")
    report_path = os.path.join(report_dir, "free_model_report.json")
    routing_path = os.path.join(report_dir, "free_cascade_priority.json")
    
    with open(report_path, "w") as f:
        json.dump(report, f, indent=2)
    print(f"\nReport written to: {report_path}")
    
    with open(routing_path, "w") as f:
        json.dump(priority, f, indent=2)
    print(f"Routing priority written to: {routing_path}")
    
    # Check for newly appeared or disappeared models vs current routing
    routing_config = "/home/vibes/vibepilot/governor/config/routing.json"
    if os.path.exists(routing_config):
        with open(routing_config) as f:
            current = json.load(f)
        current_cascade = set(current.get("strategies", {}).get("free_cascade", {}).get("priority", []))
        new_cascade = set(priority)
        
        appeared = new_cascade - current_cascade
        disappeared = current_cascade - new_cascade
        
        if appeared:
            print(f"\nNEW models (not in current routing):")
            for m in appeared:
                print(f"  + {m}")
        if disappeared:
            print(f"\nREMOVED models (in routing but not free anymore):")
            for m in disappeared:
                print(f"  - {m}")
        if not appeared and not disappeared:
            print(f"\nNo changes detected vs current routing.json")


if __name__ == "__main__":
    main()
