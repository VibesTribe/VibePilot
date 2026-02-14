"""
Base API Runner with Prompt Caching Support

Prompt caching saves 75% on repeated context by caching:
- System prompts
- PRD/Plan context
- Large reference documents

Usage:
    runner = APIRunnerWithCaching(api_key, model="deepseek-chat")
    result = runner.execute_with_cache(
        prompt="...",
        cached_context=["System prompt", "PRD section", "Plan"]
    )
"""

import os
import json
import logging
import requests
from typing import Optional, Dict, Any, List
from abc import ABC, abstractmethod

logger = logging.getLogger("VibePilot.APIRunner")


class APIRunnerWithCaching:
    """Base class for API runners with prompt caching support."""

    def __init__(
        self,
        api_key: str,
        base_url: str,
        model: str,
        provider: str = "openai_compatible",
    ):
        self.api_key = api_key
        self.base_url = base_url.rstrip("/")
        self.model = model
        self.provider = provider
        self.logger = logger

    def _build_messages_with_cache(
        self, prompt: str, cached_context: List[str] = None, system_prompt: str = None
    ) -> List[Dict[str, Any]]:
        """
        Build messages array with cache control.

        Cached context is marked with ephemeral cache_control.
        This means the provider caches it and only charges for new tokens.
        """
        messages = []

        # System prompt (always cached if provided)
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})

        # Cached context (PRD, plans, reference docs)
        # These are marked for caching - provider stores them
        if cached_context:
            cached_content = "\n\n---\n\n".join(cached_context)
            messages.append(
                {
                    "role": "user",
                    "content": [
                        {
                            "type": "text",
                            "text": cached_content,
                            "cache_control": {"type": "ephemeral"},
                        }
                    ],
                }
            )
            messages.append(
                {
                    "role": "assistant",
                    "content": "I have reviewed the context. How can I help?",
                }
            )

        # Actual prompt (this is what we're paying for)
        messages.append({"role": "user", "content": prompt})

        return messages

    def execute(
        self,
        prompt: str,
        cached_context: List[str] = None,
        system_prompt: str = None,
        temperature: float = 0.7,
        max_tokens: int = 4000,
        timeout: int = 120,
    ) -> Dict[str, Any]:
        """
        Execute prompt with optional caching.

        Args:
            prompt: The actual question/task
            cached_context: List of context strings to cache (PRD, plans, etc)
            system_prompt: System instructions
            temperature: Randomness (0-1)
            max_tokens: Max response length
            timeout: Request timeout in seconds

        Returns:
            {"success": bool, "output": str, "tokens_used": int, "cached": bool, "error": str}
        """
        messages = self._build_messages_with_cache(
            prompt, cached_context, system_prompt
        )

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": max_tokens,
        }

        url = f"{self.base_url}/chat/completions"

        try:
            self.logger.info(f"Calling {self.provider} API: {self.model}")

            response = requests.post(
                url, headers=headers, json=payload, timeout=timeout
            )

            if response.status_code == 200:
                data = response.json()
                output = data["choices"][0]["message"]["content"]

                # Token usage
                usage = data.get("usage", {})
                tokens_used = usage.get("total_tokens", 0)

                # Check if caching was used (Anthropic returns cache stats)
                cached = usage.get("cache_read_input_tokens", 0) > 0

                self.logger.info(
                    f"API call successful. Tokens: {tokens_used}, Cached: {cached}"
                )

                return {
                    "success": True,
                    "output": output,
                    "tokens_used": tokens_used,
                    "cached": cached,
                    "model": self.model,
                    "error": None,
                }
            else:
                error = f"API error {response.status_code}: {response.text}"
                self.logger.error(error)
                return {
                    "success": False,
                    "output": None,
                    "tokens_used": 0,
                    "cached": False,
                    "model": self.model,
                    "error": error,
                }

        except requests.Timeout:
            error = f"Timeout after {timeout}s"
            self.logger.error(error)
            return {
                "success": False,
                "output": None,
                "tokens_used": 0,
                "cached": False,
                "model": self.model,
                "error": error,
            }
        except Exception as e:
            error = str(e)
            self.logger.error(f"API call failed: {error}")
            return {
                "success": False,
                "output": None,
                "tokens_used": 0,
                "cached": False,
                "model": self.model,
                "error": error,
            }


class DeepSeekRunner(APIRunnerWithCaching):
    """DeepSeek API runner with caching."""

    def __init__(self, api_key: str = None):
        api_key = api_key or os.getenv("DEEPSEEK_API_KEY")
        super().__init__(
            api_key=api_key,
            base_url="https://api.deepseek.com/v1",
            model="deepseek-chat",
            provider="deepseek",
        )


class GLMAPIRunner(APIRunnerWithCaching):
    """GLM API runner with caching."""

    def __init__(self, api_key: str = None):
        api_key = api_key or os.getenv("GLM_API_KEY")
        super().__init__(
            api_key=api_key,
            base_url="https://open.bigmodel.cn/api/paas/v4",
            model="glm-4",
            provider="zhipu",
        )


class GeminiRunner(APIRunnerWithCaching):
    """Gemini API runner with caching."""

    def __init__(self, api_key: str = None):
        api_key = api_key or os.getenv("GEMINI_API_KEY")
        super().__init__(
            api_key=api_key,
            base_url="https://generativelanguage.googleapis.com/v1beta",
            model="gemini-2.0-flash",
            provider="google",
        )


# Factory function
def get_runner(provider: str, api_key: str = None) -> APIRunnerWithCaching:
    """Get appropriate runner for provider."""
    runners = {
        "deepseek": DeepSeekRunner,
        "glm": GLMAPIRunner,
        "gemini": GeminiRunner,
    }

    if provider not in runners:
        raise ValueError(f"Unknown provider: {provider}")

    return runners[provider](api_key)


if __name__ == "__main__":
    from dotenv import load_dotenv

    load_dotenv()

    print("=== API Runner Test ===\n")

    # Test DeepSeek
    print("1. Testing DeepSeek with caching...")
    runner = DeepSeekRunner()

    # Cached context (simulating PRD/Plan)
    cached_context = [
        "# VibePilot PRD\n\nVibePilot is a sovereign AI execution engine...",
        "# Architecture\n\nAll state in Supabase, all code in GitHub...",
    ]

    result = runner.execute(
        prompt="What is VibePilot?",
        cached_context=cached_context,
        system_prompt="You are a helpful assistant.",
    )

    print(f"   Success: {result['success']}")
    print(f"   Tokens: {result['tokens_used']}")
    print(f"   Cached: {result['cached']}")
    if result["success"]:
        print(f"   Output: {result['output'][:200]}...")

    print("\n=== Test Complete ===")
