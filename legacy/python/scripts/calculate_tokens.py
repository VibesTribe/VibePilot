#!/usr/bin/env python3
"""
Token Calculator for VibePilot ROI
Simple estimation: ~4 characters per token for English text
More accurate with tiktoken when available.
"""

def estimate_tokens(text: str) -> int:
    """Estimate token count from text."""
    if not text:
        return 0
    # Rough estimate: 4 chars per token for English
    return max(1, len(text) // 4)


def calculate_roi(
    input_text: str,
    output_text: str,
    model: str = "kimi-cli",
    platform: str = "web"
) -> dict:
    """Calculate theoretical cost vs actual cost for a task."""
    
    input_tokens = estimate_tokens(input_text)
    output_tokens = estimate_tokens(output_text)
    total_tokens = input_tokens + output_tokens
    
    # Pricing (per 1K tokens)
    api_costs = {
        "deepseek-chat": {"input": 0.50, "output": 2.00},
        "deepseek-reasoner": {"input": 4.00, "output": 16.00},
        "gemini-2.0-flash": {"input": 0.10, "output": 0.40},
        "gemini-2.0-pro": {"input": 1.25, "output": 5.00},
    }
    
    # Subscription costs (monthly, amortized per task)
    # Assuming 100 tasks/day = 3000 tasks/month
    sub_costs = {
        "kimi-cli": 19.00 / 3000,  # $0.0063/task
        "opencode": 0.00,  # Free tier
    }
    
    # Calculate theoretical API cost
    if model in api_costs:
        cost_input = (input_tokens / 1000) * api_costs[model]["input"]
        cost_output = (output_tokens / 1000) * api_costs[model]["output"]
        theoretical_cost = cost_input + cost_output
    else:
        theoretical_cost = 0.0
    
    # Calculate actual cost
    actual_cost = sub_costs.get(model, 0.0)
    
    # Savings
    savings = theoretical_cost - actual_cost if theoretical_cost > actual_cost else 0.0
    
    return {
        "input_tokens": input_tokens,
        "output_tokens": output_tokens,
        "total_tokens": total_tokens,
        "theoretical_api_cost": theoretical_cost,
        "actual_cost": actual_cost,
        "savings": savings,
        "model": model,
        "platform": platform,
    }


# Example: Raindrop bookmark analysis
if __name__ == "__main__":
    # This is similar to what I did for the Raindrop research
    prompt = """Analyze this Raindrop bookmark for VibePilot relevance.

Title: GitHub - microsoft/playwright-cli
URL: https://github.com/microsoft/playwright-cli
Collection: vibeflow

Analyze relevance across these 15 categories and score 1-10."""

    output = """## Analysis: microsoft/playwright-cli

**Relevance Score: 8/10** 🔴 HIGH PRIORITY

**Primary Category:** Browser Automation (3)
**Secondary:** Agent Architecture (2), Cost Optimization (9)

### Key Insight
Playwright CLI provides token-efficient alternatives to vision-based approaches.
Accessibility tree navigation = 25x token savings vs vision.

**Recommended Action:** VET - Council review for integration"""

    result = calculate_roi(prompt, output, model="kimi-cli", platform="claude")
    
    print("=" * 50)
    print("VIBEPILOT TOKEN CALCULATION (Raindrop Research Example)")
    print("=" * 50)
    print(f"Input tokens:  {result['input_tokens']:,}")
    print(f"Output tokens: {result['output_tokens']:,}")
    print(f"Total tokens:  {result['total_tokens']:,}")
    print()
    print(f"Theoretical API cost: ${result['theoretical_api_cost']:.4f}")
    print(f"Actual cost (Kimi CLI): ${result['actual_cost']:.4f}")
    print(f"Savings: ${result['savings']:.4f}")
    print()
    print("=" * 50)
    
    # Compare with DeepSeek API
    deepseek_result = calculate_roi(prompt, output, model="deepseek-chat", platform="api")
    print("\nIf you used DeepSeek API instead:")
    print(f"Cost: ${deepseek_result['theoretical_api_cost']:.4f}")
    print(f"vs Kimi CLI: ${result['actual_cost']:.4f}")
    
    if deepseek_result['theoretical_api_cost'] > result['actual_cost']:
        print(f"✅ Kimi CLI saves ${deepseek_result['theoretical_api_cost'] - result['actual_cost']:.4f} per task")
    else:
        print(f"⚠️  DeepSeek API would be ${result['actual_cost'] - deepseek_result['theoretical_api_cost']:.4f} cheaper")
