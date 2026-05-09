#!/usr/bin/env python3
"""
Local courier agent for VibePilot.
Uses browser-harness to interact with web AI platforms via CDP.

This file is a REFERENCE for the platform interaction logic.
The actual execution happens via browser-harness -c, with the
Go governor generating the script dynamically from these patterns.

Platform selectors and interaction patterns documented here
so any agent can understand and maintain the courier logic.
"""

# ============================================================
# PLATFORM CONFIGS
# ============================================================
# Each platform defines:
#   url: base URL for new conversations
#   input_coords: x,y coordinates for clicking the input area
#   response_selector: CSS selector for extracting model responses
#   submit_key: key to press after typing (usually "Enter")
#   wait_initial: seconds to wait after submit before polling
#   wait_stable: consecutive stable polls before considering done

PLATFORMS = {
    "gemini": {
        "url": "https://gemini.google.com/app",
        "input_coords": (482, 283),
        "response_selector": "message-content",
        "submit_key": "Enter",
        "wait_initial": 8,
        "wait_stable": 3,
    },
    "chatgpt": {
        "url": "https://chatgpt.com",
        "input_coords": (640, 700),  # approx bottom center
        "response_selector": "[data-message-author-role='assistant']",
        "submit_key": "Enter",
        "wait_initial": 8,
        "wait_stable": 3,
    },
    "claude": {
        "url": "https://claude.ai/new",
        "input_coords": (640, 700),
        "response_selector": "[data-testid='assistant-message']",
        "submit_key": "Enter",
        "wait_initial": 10,
        "wait_stable": 3,
    },
    "deepseek": {
        "url": "https://chat.deepseek.com",
        "input_coords": (640, 700),
        "response_selector": ".ds-markdown",
        "submit_key": "Enter",
        "wait_initial": 8,
        "wait_stable": 3,
    },
}

# ============================================================
# BROWSER-HARNESS SCRIPT TEMPLATE
# ============================================================
# This is the script template that the Go governor fills in and
# passes to `browser-harness -c`. The {placeholders} are replaced
# with actual values from the task packet.

BROWSER_HARNESS_SCRIPT = """
import json, time, sys

# Read task packet from temp file
with open("{task_file}") as f:
    task = json.load(f)

prompt = task.get("prompt", task.get("task_prompt", ""))
platform_url = task.get("web_platform_url", "")
input_x = {input_x}
input_y = {input_y}
response_sel = "{response_selector}"
submit_key = "{submit_key}"
wait_initial = {wait_initial}
wait_stable = {wait_stable}

# Open tab on platform
tid = new_tab(platform_url)
try:
    wait_for_load()
    time.sleep(3)

    # Click on input area to focus it
    click_at_xy(input_x, input_y)
    time.sleep(0.5)

    # Type the prompt
    type_text(prompt)
    time.sleep(0.5)

    # Submit
    press_key(submit_key)

    # Wait for response to start generating
    time.sleep(wait_initial)

    # Poll for response completion (text stops changing)
    last_text = ""
    stable_count = 0
    for i in range(36):
        time.sleep(5)
        current = js("document.body.innerText")
        if current == last_text:
            stable_count += 1
            if stable_count >= wait_stable:
                break
        else:
            stable_count = 0
            last_text = current

    # Extract response using platform-specific selector
    response = js(
        "Array.from(document.querySelectorAll('" + response_sel + "'))"
        ".map(e => e.textContent).join('|||')"
    )

    # Fallback: if selector found nothing, use text diff
    if not response:
        before = js("document.body.innerText")
        # Re-read is same as current since we already waited
        response = "FALLBACK: selector found no response"

    result = {{
        "status": "success",
        "output": response,
        "tokens_in": len(prompt) // 4,
        "tokens_out": len(response) // 4
    }}
    print(json.dumps(result))

except Exception as e:
    print(json.dumps({{"status": "error", "error": str(e)}}))

finally:
    # Always close the tab
    try:
        cdp("Target.closeTarget", targetId=tid)
    except:
        pass
"""

# ============================================================
# PLATFORM DETECTION
# ============================================================

def detect_platform(url):
    """Detect which platform handler to use from URL."""
    url_lower = url.lower()
    if "gemini.google" in url_lower:
        return "gemini"
    elif "chatgpt.com" in url_lower or "chat.openai.com" in url_lower:
        return "chatgpt"
    elif "claude.ai" in url_lower:
        return "claude"
    elif "deepseek" in url_lower:
        return "deepseek"
    return None
