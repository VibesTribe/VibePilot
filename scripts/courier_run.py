#!/usr/bin/env python3
"""
Courier agent: browser-use + Playwright for automated web AI platform interaction.
Runs on GitHub Actions (or locally) to dispatch tasks to web-based AI platforms.

Flow:
1. Receive task packet from environment variables
2. Navigate to web platform URL via browser-use
3. Paste the prompt into the chat interface
4. Wait for the response
5. Extract the response text
6. Write result back to Supabase task_runs table
"""

import os
import sys
import json
import time
import asyncio
import urllib.request
import urllib.error
from datetime import datetime, timezone


# --- Platform-specific selectors for chat interfaces ---
# Each platform has its own DOM structure for input and response areas.
# These selectors tell browser-use where to type and where to read.

PLATFORM_SELECTORS = {
    # ChatGPT web interface
    "chat.openai.com": {
        "input_selector": 'div[contenteditable="true"]',
        "submit_selector": 'button[data-testid="send-button"]',
        "response_selector": 'div[data-message-author-role="assistant"]:last-child',
        "wait_for_response": 30,
    },
    # Gemini web interface
    "gemini.google.com": {
        "input_selector": 'div.ql-editor[contenteditable="true"]',
        "submit_selector": 'button[aria-label="Send message"]',
        "response_selector": 'model-response:last-child message-content',
        "wait_for_response": 30,
    },
    # DeepSeek web interface
    "chat.deepseek.com": {
        "input_selector": 'textarea[placeholder*="message"]',
        "submit_selector": 'div[class*="send-button"]',
        "response_selector": 'div[class*="assistant-message"]:last-child',
        "wait_for_response": 45,
    },
    # Qwen/ChatGLM web interface (Tongyi Qianwen)
    "tongyi.aliyun.com": {
        "input_selector": 'div[contenteditable="true"]',
        "submit_selector": 'button[class*="send"]',
        "response_selector": 'div[class*="assistant"]:last-child',
        "wait_for_response": 30,
    },
    # Generic fallback for unknown platforms
    "_default": {
        "input_selector": 'textarea, div[contenteditable="true"], input[type="text"]',
        "submit_selector": 'button[type="submit"], button[aria-label*="send"], button[aria-label*="Send"]',
        "response_selector": 'div[class*="response"], div[class*="assistant"], div[class*="answer"]',
        "wait_for_response": 30,
    },
}


def get_platform_config(url: str) -> dict:
    """Match URL to platform-specific selectors."""
    from urllib.parse import urlparse
    host = urlparse(url).hostname or ""
    for domain, config in PLATFORM_SELECTORS.items():
        if domain != "_default" and domain in host:
            return config
    return PLATFORM_SELECTORS["_default"]


def update_supabase(task_id: str, status: str, output: str = "",
                    error: str = "", tokens_in: int = 0, tokens_out: int = 0):
    """Write result back to Supabase task_runs table."""
    supabase_url = os.environ["SUPABASE_URL"]
    supabase_key = os.environ["SUPABASE_KEY"]

    data = {
        "status": status,
        "completed_at": datetime.now(timezone.utc).isoformat(),
    }
    if output:
        data["output"] = output
    if error:
        data["error"] = error
    if tokens_in > 0:
        data["tokens_in"] = tokens_in
    if tokens_out > 0:
        data["tokens_out"] = tokens_out

    payload = json.dumps(data).encode()
    req = urllib.request.Request(
        f"{supabase_url}/rest/v1/task_runs?id=eq.{task_id}",
        data=payload,
        method="PATCH",
    )
    req.add_header("apikey", supabase_key)
    req.add_header("Authorization", f"Bearer {supabase_key}")
    req.add_header("Content-Type", "application/json")
    req.add_header("Prefer", "return=minimal")

    try:
        urllib.request.urlopen(req, timeout=10)
    except urllib.error.URLError as e:
        print(f"ERROR: Failed to update Supabase: {e}", file=sys.stderr)
        sys.exit(1)


async def run_courier():
    """Main courier execution using browser-use."""
    task_id = os.environ["TASK_ID"]
    prompt = os.environ["PROMPT"]
    platform_url = os.environ["WEB_PLATFORM_URL"]
    timeout_secs = int(os.environ.get("COURIER_TIMEOUT", "300"))

    print(f"[Courier] Task: {task_id}")
    print(f"[Courier] Platform: {platform_url}")
    print(f"[Courier] Prompt length: {len(prompt)} chars")

    config = get_platform_config(platform_url)
    print(f"[Courier] Using selectors for: {platform_url}")

    try:
        from browser_use import Agent, Browser, BrowserConfig
        from langchain_openai import ChatOpenAI

        # Build the LLM for browser-use agent
        # browser-use uses an LLM to decide how to interact with the page
        llm_provider = os.environ.get("LLM_PROVIDER", "openai")
        llm_model = os.environ.get("LLM_MODEL", "gpt-4o-mini")
        llm_api_key = os.environ.get("LLM_API_KEY", "")

        if not llm_api_key:
            raise ValueError("LLM_API_KEY is required for browser-use agent")

        # Map provider to OpenAI-compatible base URL
        base_url = None
        if llm_provider == "groq":
            base_url = "https://api.groq.com/openai/v1"
        elif llm_provider == "openrouter":
            base_url = "https://openrouter.ai/api/v1"
        elif llm_provider == "nvidia":
            base_url = "https://integrate.api.nvidia.com/v1"
        elif llm_provider == "google":
            base_url = "https://generativelanguage.googleapis.com/v1beta/openai"

        llm_kwargs = {"model": llm_model, "api_key": llm_api_key}
        if base_url:
            llm_kwargs["base_url"] = base_url

        llm = ChatOpenAI(**llm_kwargs)

        # Configure headless Chromium
        browser = Browser(config=BrowserConfig(
            headless=True,
            disable_security=True,
        ))

        # Build the task instruction for browser-use
        task_instruction = f"""Navigate to {platform_url} and submit this prompt:

PROMPT TO SUBMIT:
{prompt}

Steps:
1. Go to {platform_url}
2. Find the chat input area
3. Type/paste the exact prompt above
4. Submit it (press Enter or click send)
5. Wait for the complete response
6. Extract and return ONLY the text of the response

Return the full response text."""

        agent = Agent(
            task=task_instruction,
            llm=llm,
            browser=browser,
            max_actions_per_step=5,
        )

        # Run with timeout
        result = await asyncio.wait_for(agent.run(), timeout=timeout_secs)

        # Extract the response text
        output = ""
        if hasattr(result, 'result') and result.result:
            output = str(result.result)
        elif hasattr(result, '__str__'):
            output = str(result)

        tokens_in = len(prompt) // 4  # rough estimate
        tokens_out = len(output) // 4

        print(f"[Courier] Success! Output length: {len(output)} chars")
        print(f"[Courier] Estimated tokens: {tokens_in}/{tokens_out}")

        update_supabase(task_id, "completed", output=output,
                       tokens_in=tokens_in, tokens_out=tokens_out)

    except asyncio.TimeoutError:
        print(f"[Courier] Timeout after {timeout_secs}s", file=sys.stderr)
        update_supabase(task_id, "failed", error=f"Timeout after {timeout_secs}s")

    except Exception as e:
        print(f"[Courier] Error: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        update_supabase(task_id, "failed", error=str(e))


if __name__ == "__main__":
    asyncio.run(run_courier())
