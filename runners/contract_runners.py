"""
Contract-Compliant Runners

These runners follow the RUNNER_INTERFACE contract exactly.
They can be called via:
    python runners/contract_runners.py --runner kimi --task packet.json
    cat packet.json | python runners/contract_runners.py --runner kimi
    python runners/contract_runners.py --runner kimi --probe
"""

import os
import sys
import json
import time
import subprocess
import logging
from typing import Dict, Any, Optional
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from runners.base_runner import BaseRunner, run_runner
from vault_manager import get_api_key

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s | %(levelname)s | %(name)s | %(message)s"
)
logger = logging.getLogger("VibePilot.ContractRunners")


class KimiContractRunner(BaseRunner):
    """
    Kimi CLI runner following the contract interface.

    Wraps the existing Kimi CLI subscription for tasks requiring
    codebase access and larger context.
    """

    VERSION = "1.0.0"
    RUNNER_TYPE = "cli"

    COST_INPUT_PER_1K = 0.0
    COST_OUTPUT_PER_1K = 0.0

    def __init__(self):
        super().__init__(runner_id="kimi-cli")
        self.kimi_path = self._find_kimi()

    def _find_kimi(self) -> str:
        result = subprocess.run(["which", "kimi"], capture_output=True, text=True)
        if result.returncode == 0:
            return result.stdout.strip()
        return os.path.expanduser("~/.local/bin/kimi")

    def probe(self) -> tuple[bool, str]:
        """Check if Kimi CLI is available and authenticated."""
        try:
            result = subprocess.run(
                [self.kimi_path, "--version"],
                capture_output=True,
                text=True,
                timeout=10,
            )
            if result.returncode == 0:
                return True, "OK"
            return False, f"PROBE_FAILED: Kimi returned {result.returncode}"
        except subprocess.TimeoutExpired:
            return False, "PROBE_FAILED: Timeout checking Kimi"
        except FileNotFoundError:
            return False, "PROBE_FAILED: Kimi CLI not found"
        except Exception as e:
            return False, f"PROBE_FAILED: {e}"

    def execute(self, task_packet: Dict[str, Any]) -> Dict[str, Any]:
        """Execute task via Kimi CLI."""
        task_id = task_packet.get("task_id", "unknown")
        prompt = task_packet.get("prompt", "")
        constraints = task_packet.get("constraints", {})
        runner_context = task_packet.get("runner_context", {})

        timeout = constraints.get("timeout_seconds", 300)
        work_dir = runner_context.get("work_dir", os.getcwd())

        codebase_files = task_packet.get("codebase_files", {})

        full_prompt = prompt
        if codebase_files:
            file_context = "\n\n".join(
                [
                    f"--- {filename} ---\n{content}"
                    for filename, content in codebase_files.items()
                ]
            )
            full_prompt = f"Context files:\n\n{file_context}\n\n---\n\nTask:\n{prompt}"

        cmd = [
            self.kimi_path,
            "--yolo",
            "--print",
            "--output-format",
            "text",
            "--final-message-only",
            "--prompt",
            full_prompt,
        ]

        start_time = time.time()

        try:
            result = subprocess.run(
                cmd, capture_output=True, text=True, timeout=timeout, cwd=work_dir
            )

            duration = time.time() - start_time

            if result.returncode == 0:
                output = result.stdout.strip()
                tokens_in = len(full_prompt) // 4
                tokens_out = len(output) // 4

                return self.build_success_result(
                    task_id=task_id,
                    output=output,
                    tokens_in=tokens_in,
                    tokens_out=tokens_out,
                    duration_seconds=duration,
                    files_read=len(codebase_files),
                    files_modified=0,
                )
            else:
                error = result.stderr.strip() or f"Kimi returned {result.returncode}"
                return self.build_failure_result(
                    task_id=task_id,
                    error_code="KIMI_ERROR",
                    error_message=error,
                    suggested_next_step="retry",
                )

        except subprocess.TimeoutExpired:
            return self.build_failure_result(
                task_id=task_id,
                error_code="TIMEOUT",
                error_message=f"Task timed out after {timeout}s",
                suggested_next_step="split",
            )
        except Exception as e:
            return self.build_failure_result(
                task_id=task_id,
                error_code="RUNNER_ERROR",
                error_message=str(e),
                suggested_next_step="retry",
            )

    def _calculate_virtual_cost(self, tokens_in: int, tokens_out: int) -> float:
        return (tokens_in * self.COST_INPUT_PER_1K / 1000) + (
            tokens_out * self.COST_OUTPUT_PER_1K / 1000
        )


class DeepSeekContractRunner(BaseRunner):
    """
    DeepSeek API runner following the contract interface.

    Uses API credits sparingly. Good for tasks without codebase access.
    """

    VERSION = "1.0.0"
    RUNNER_TYPE = "api"

    COST_INPUT_PER_1K = 0.00014
    COST_OUTPUT_PER_1K = 0.00028
    BASE_URL = "https://api.deepseek.com/v1"
    MODEL = "deepseek-chat"

    def __init__(self, api_key: str = None):
        super().__init__(runner_id="deepseek-chat")
        self.api_key = api_key or self._get_api_key()

    def _get_api_key(self) -> str:
        try:
            return get_api_key("DEEPSEEK_API_KEY")
        except Exception as e:
            self.logger.error(f"Failed to get DeepSeek API key: {e}")
            return None

    def probe(self) -> tuple[bool, str]:
        """Check if DeepSeek API key is available."""
        if not self.api_key:
            return False, "PROBE_FAILED: DEEPSEEK_API_KEY not in vault"

        try:
            import requests

            response = requests.get(
                f"{self.BASE_URL}/models",
                headers={"Authorization": f"Bearer {self.api_key}"},
                timeout=10,
            )
            if response.status_code == 200:
                return True, "OK"
            return False, f"PROBE_FAILED: API returned {response.status_code}"
        except Exception as e:
            return False, f"PROBE_FAILED: {e}"

    def execute(self, task_packet: Dict[str, Any]) -> Dict[str, Any]:
        """Execute task via DeepSeek API."""
        import requests

        task_id = task_packet.get("task_id", "unknown")
        prompt = task_packet.get("prompt", "")
        constraints = task_packet.get("constraints", {})

        if not self.api_key:
            return self.build_failure_result(
                task_id=task_id,
                error_code="NO_API_KEY",
                error_message="DeepSeek API key not available",
                suggested_next_step="reassign",
            )

        max_tokens = constraints.get("max_tokens", 4000)
        timeout = constraints.get("timeout_seconds", 120)

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

        payload = {
            "model": self.MODEL,
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": max_tokens,
            "temperature": 0.7,
        }

        start_time = time.time()

        try:
            response = requests.post(
                f"{self.BASE_URL}/chat/completions",
                headers=headers,
                json=payload,
                timeout=timeout,
            )

            duration = time.time() - start_time

            if response.status_code == 200:
                data = response.json()
                output = data["choices"][0]["message"]["content"]
                usage = data.get("usage", {})
                tokens_in = usage.get("prompt_tokens", 0)
                tokens_out = usage.get("completion_tokens", 0)

                return self.build_success_result(
                    task_id=task_id,
                    output=output,
                    tokens_in=tokens_in,
                    tokens_out=tokens_out,
                    duration_seconds=duration,
                )
            else:
                error_msg = f"API error {response.status_code}: {response.text[:200]}"
                suggested = (
                    "reassign" if response.status_code in [401, 403, 429] else "retry"
                )
                return self.build_failure_result(
                    task_id=task_id,
                    error_code="API_ERROR",
                    error_message=error_msg,
                    suggested_next_step=suggested,
                )

        except requests.Timeout:
            return self.build_failure_result(
                task_id=task_id,
                error_code="TIMEOUT",
                error_message=f"API request timed out after {timeout}s",
                suggested_next_step="retry",
            )
        except Exception as e:
            return self.build_failure_result(
                task_id=task_id,
                error_code="RUNNER_ERROR",
                error_message=str(e),
                suggested_next_step="retry",
            )

    def _calculate_virtual_cost(self, tokens_in: int, tokens_out: int) -> float:
        return (tokens_in * self.COST_INPUT_PER_1K / 1000) + (
            tokens_out * self.COST_OUTPUT_PER_1K / 1000
        )


class GeminiContractRunner(BaseRunner):
    """
    Gemini API runner following the contract interface.

    Free tier API, good for research and tasks without codebase access.
    """

    VERSION = "1.0.0"
    RUNNER_TYPE = "api"

    COST_INPUT_PER_1K = 0.0
    COST_OUTPUT_PER_1K = 0.0
    BASE_URL = "https://generativelanguage.googleapis.com/v1beta"
    MODEL = "gemini-2.0-flash"

    def __init__(self, api_key: str = None):
        super().__init__(runner_id="gemini-api")
        self.api_key = api_key or self._get_api_key()

    def _get_api_key(self) -> str:
        try:
            return get_api_key("GEMINI_API_KEY")
        except Exception as e:
            self.logger.error(f"Failed to get Gemini API key: {e}")
            return None

    def probe(self) -> tuple[bool, str]:
        """Check if Gemini API key is available."""
        if not self.api_key:
            return False, "PROBE_FAILED: GEMINI_API_KEY not in vault"
        return True, "OK"

    def execute(self, task_packet: Dict[str, Any]) -> Dict[str, Any]:
        """Execute task via Gemini API."""
        import requests

        task_id = task_packet.get("task_id", "unknown")
        prompt = task_packet.get("prompt", "")
        constraints = task_packet.get("constraints", {})

        if not self.api_key:
            return self.build_failure_result(
                task_id=task_id,
                error_code="NO_API_KEY",
                error_message="Gemini API key not available",
                suggested_next_step="reassign",
            )

        max_tokens = constraints.get("max_tokens", 4000)
        timeout = constraints.get("timeout_seconds", 120)

        url = f"{self.BASE_URL}/models/{self.MODEL}:generateContent?key={self.api_key}"

        payload = {
            "contents": [{"parts": [{"text": prompt}]}],
            "generationConfig": {"maxOutputTokens": max_tokens, "temperature": 0.7},
        }

        start_time = time.time()

        try:
            response = requests.post(url, json=payload, timeout=timeout)

            duration = time.time() - start_time

            if response.status_code == 200:
                data = response.json()

                candidates = data.get("candidates", [])
                if candidates:
                    output = (
                        candidates[0]
                        .get("content", {})
                        .get("parts", [{}])[0]
                        .get("text", "")
                    )
                else:
                    output = ""

                usage = data.get("usageMetadata", {})
                tokens_in = usage.get("promptTokenCount", 0)
                tokens_out = usage.get("candidatesTokenCount", 0)

                return self.build_success_result(
                    task_id=task_id,
                    output=output,
                    tokens_in=tokens_in,
                    tokens_out=tokens_out,
                    duration_seconds=duration,
                )
            else:
                error_msg = f"API error {response.status_code}: {response.text[:200]}"
                suggested = (
                    "reassign" if response.status_code in [401, 403, 429] else "retry"
                )
                return self.build_failure_result(
                    task_id=task_id,
                    error_code="API_ERROR",
                    error_message=error_msg,
                    suggested_next_step=suggested,
                )

        except requests.Timeout:
            return self.build_failure_result(
                task_id=task_id,
                error_code="TIMEOUT",
                error_message=f"API request timed out after {timeout}s",
                suggested_next_step="retry",
            )
        except Exception as e:
            return self.build_failure_result(
                task_id=task_id,
                error_code="RUNNER_ERROR",
                error_message=str(e),
                suggested_next_step="retry",
            )


class CourierContractRunner(BaseRunner):
    """
    Web platform courier runner following the contract interface.

    Two separate concepts:
    1. LLM that drives browser-use (from config, any API model)
    2. Web platform destination (ChatGPT web, Claude web, Gemini web)

    The LLM is assigned via config/models.json. Swap by changing one line.
    """

    VERSION = "1.0.0"
    RUNNER_TYPE = "courier"

    WEB_PLATFORMS = {
        "chatgpt": {
            "url": "https://chat.openai.com",
            "new_chat_url": "https://chat.openai.com/?model=auto",
            "name": "ChatGPT",
        },
        "claude": {
            "url": "https://claude.ai",
            "new_chat_url": "https://claude.ai/new",
            "name": "Claude",
        },
        "gemini": {
            "url": "https://gemini.google.com",
            "new_chat_url": "https://gemini.google.com/app",
            "name": "Gemini",
        },
    }

    def __init__(self, platform: str = "gemini", llm_model_id: str = None):
        super().__init__(runner_id=f"courier-{platform}")
        self.platform = platform
        self.llm_model_id = llm_model_id or "gemini-api"
        self._llm = None

    def _get_llm(self):
        """
        Get LLM for browser-use based on model_id from config.
        Supports any API model - swap by changing llm_model_id.
        """
        if self._llm:
            return self._llm

        from core.config_loader import get_config_loader

        config = get_config_loader()
        model_config = config.get_model(self.llm_model_id)

        if not model_config:
            self.logger.error(f"Model not found: {self.llm_model_id}")
            return None

        provider = model_config.get("provider", "")
        access_type = model_config.get("access_type", "")

        if access_type != "api":
            self.logger.error(
                f"Model {self.llm_model_id} is not API-accessible (type: {access_type})"
            )
            return None

        api_key_ref = model_config.get("api_key_ref")
        api_key = get_api_key(api_key_ref) if api_key_ref else None

        if not api_key:
            self.logger.error(f"No API key for {self.llm_model_id}")
            return None

        if provider == "google":
            return self._create_gemini_adapter(api_key, model_config)
        elif provider == "deepseek":
            return self._create_openai_compatible_adapter(
                api_key,
                "https://api.deepseek.com/v1",
                model_config.get("id", "deepseek-chat"),
            )
        else:
            self.logger.error(f"Unknown provider: {provider}")
            return None

    def _create_gemini_adapter(self, api_key: str, model_config: dict):
        """Create browser-use compatible Gemini adapter."""
        try:
            from google import genai
            from browser_use.llm.messages import UserMessage, AssistantMessage

            model_name = model_config.get("id", "gemini-api")
            if model_name == "gemini-api":
                model_name = "gemini-2.0-flash"

            class GeminiAdapter:
                def __init__(self, api_key, model_name):
                    self.client = genai.Client(api_key=api_key)
                    self.model_name = model_name
                    self.name = model_name
                    self.provider = "google"

                async def ainvoke(self, messages):
                    contents = []
                    for msg in messages:
                        role = "user" if isinstance(msg, UserMessage) else "model"
                        content = (
                            msg.content
                            if isinstance(msg.content, str)
                            else str(msg.content)
                        )
                        contents.append({"role": role, "parts": [content]})

                    response = self.client.models.generate_content(
                        model=self.model_name,
                        contents=contents,
                    )
                    return AssistantMessage(content=response.text)

            self._llm = GeminiAdapter(api_key, model_name)
            return self._llm

        except Exception as e:
            self.logger.error(f"Failed to create Gemini adapter: {e}")
            return None

    def _create_openai_compatible_adapter(
        self, api_key: str, base_url: str, model_id: str
    ):
        """Create browser-use compatible adapter for OpenAI-compatible APIs (DeepSeek, etc)."""
        try:
            import requests
            from browser_use.llm.messages import UserMessage, AssistantMessage

            class OpenAICompatibleAdapter:
                def __init__(self, api_key, base_url, model_id):
                    self.api_key = api_key
                    self.base_url = base_url.rstrip("/")
                    self.model_name = model_id
                    self.name = model_id
                    self.provider = "openai-compatible"

                async def ainvoke(self, messages):
                    formatted = []
                    for msg in messages:
                        role = "user" if isinstance(msg, UserMessage) else "assistant"
                        content = (
                            msg.content
                            if isinstance(msg.content, str)
                            else str(msg.content)
                        )
                        formatted.append({"role": role, "content": content})

                    response = requests.post(
                        f"{self.base_url}/chat/completions",
                        headers={
                            "Authorization": f"Bearer {self.api_key}",
                            "Content-Type": "application/json",
                        },
                        json={"model": self.model_name, "messages": formatted},
                        timeout=60,
                    )

                    if response.status_code == 200:
                        return AssistantMessage(
                            content=response.json()["choices"][0]["message"]["content"]
                        )
                    else:
                        raise Exception(f"API error: {response.status_code}")

            self._llm = OpenAICompatibleAdapter(api_key, base_url, model_id)
            return self._llm

        except Exception as e:
            self.logger.error(f"Failed to create OpenAI-compatible adapter: {e}")
            return None

    def probe(self) -> tuple[bool, str]:
        """Check if browser automation is available."""
        try:
            llm = self._get_llm()
            if not llm:
                return (
                    False,
                    f"PROBE_FAILED: No LLM available (model: {self.llm_model_id})",
                )
            return True, "OK"
        except Exception as e:
            return False, f"PROBE_FAILED: {e}"

    def execute(self, task_packet: Dict[str, Any]) -> Dict[str, Any]:
        """Execute task via web platform using browser-use."""
        task_id = task_packet.get("task_id", "unknown")
        prompt = task_packet.get("prompt", "")
        constraints = task_packet.get("constraints", {})
        runner_context = task_packet.get("runner_context", {})

        timeout = constraints.get("timeout_seconds", 180)

        platform = runner_context.get("web_platform", self.platform)
        platform_config = self.WEB_PLATFORMS.get(platform)

        if not platform_config:
            return self.build_failure_result(
                task_id=task_id,
                error_code="UNKNOWN_PLATFORM",
                error_message=f"Unknown web platform: {platform}",
                suggested_next_step="reassign",
            )

        llm = self._get_llm()
        if not llm:
            return self.build_failure_result(
                task_id=task_id,
                error_code="NO_LLM",
                error_message=f"No LLM available for browser automation (model: {self.llm_model_id})",
                suggested_next_step="reassign",
            )

        start_time = time.time()

        try:
            from browser_use import Agent
            import asyncio

            browser_task = f"""
Go to {platform_config["new_chat_url"]}

Wait for the page to load.

Find the text input box or chat composer.

Enter this exact prompt:
---
{prompt}
---

Submit the prompt and wait for the response to complete.

Once the response is complete:
1. Copy the entire response text
2. Get the current URL (this is the chat_url for future reference)

Return a JSON object with:
{{"response": "the full response text", "chat_url": "the current page URL"}}
"""

            agent = Agent(
                task=browser_task,
                llm=llm,
            )

            result = asyncio.run(agent.run())

            duration = time.time() - start_time

            output = str(result)
            chat_url = platform_config["url"]

            import re

            url_match = re.search(r'https://[^\s"\']+', output)
            if url_match:
                chat_url = url_match.group(0)

            tokens_in = len(browser_task) // 4
            tokens_out = len(output) // 4

            return self.build_success_result(
                task_id=task_id,
                output=output,
                tokens_in=tokens_in,
                tokens_out=tokens_out,
                duration_seconds=duration,
                chat_url=chat_url,
                web_platform=platform,
                llm_model=self.llm_model_id,
            )

        except asyncio.TimeoutError:
            return self.build_failure_result(
                task_id=task_id,
                error_code="TIMEOUT",
                error_message=f"Browser automation timed out after {timeout}s",
                suggested_next_step="retry",
            )
        except Exception as e:
            self.logger.error(f"Courier execution failed: {e}")
            return self.build_failure_result(
                task_id=task_id,
                error_code="BROWSER_ERROR",
                error_message=str(e),
                suggested_next_step="retry",
            )


RUNNER_REGISTRY = {
    "kimi": KimiContractRunner,
    "kimi-cli": KimiContractRunner,
    "deepseek": DeepSeekContractRunner,
    "deepseek-chat": DeepSeekContractRunner,
    "gemini": GeminiContractRunner,
    "gemini-api": GeminiContractRunner,
    "courier": CourierContractRunner,
    "courier-chatgpt": lambda: CourierContractRunner("chatgpt"),
    "courier-claude": lambda: CourierContractRunner("claude"),
    "courier-gemini": lambda: CourierContractRunner("gemini"),
}


def get_runner(runner_id: str, llm_model_id: str = None) -> BaseRunner:
    """
    Get a runner instance by ID.

    Args:
        runner_id: Runner type (kimi, deepseek, courier, etc.)
        llm_model_id: For couriers, which LLM to use for browser-use
    """
    if runner_id not in RUNNER_REGISTRY:
        raise ValueError(
            f"Unknown runner: {runner_id}. Available: {list(RUNNER_REGISTRY.keys())}"
        )

    runner_class = RUNNER_REGISTRY[runner_id]

    if callable(runner_class) and not isinstance(runner_class, type):
        return runner_class()

    return runner_class()
    return runner_class()


if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="VibePilot Contract Runner")
    parser.add_argument(
        "--runner",
        "-r",
        required=True,
        help="Runner ID (kimi, deepseek, gemini, courier)",
    )
    parser.add_argument("--probe", action="store_true", help="Run health check")
    parser.add_argument("--task", type=str, help="Path to task packet JSON file")
    parser.add_argument("--output", type=str, help="Path to write result JSON file")

    args = parser.parse_args()

    try:
        runner = get_runner(args.runner)
    except ValueError as e:
        print(json.dumps({"error": str(e)}))
        sys.exit(1)

    if args.probe:
        success, message = runner.probe()
        print(message)
        sys.exit(0 if success else 1)

    if args.task:
        task_path = Path(args.task)
        if not task_path.exists():
            print(json.dumps({"error": f"Task file not found: {args.task}"}))
            sys.exit(1)

        with open(task_path) as f:
            task_packet = json.load(f)

        exit_code = runner.run_with_packet(task_packet)
        sys.exit(exit_code)

    exit_code = runner.run_from_stdin()
    sys.exit(exit_code)
