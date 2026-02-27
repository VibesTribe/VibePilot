"""
VibePilot Config Loader

Central module for loading all JSON config files.
Provides cached access to skills, tools, models, platforms, agents.

Usage:
    from core.config_loader import ConfigLoader

    config = ConfigLoader()
    models = config.get_models()
    agent = config.get_agent("planner")
"""

import os
import json
import logging
from typing import Dict, Any, List, Optional
from pathlib import Path
from functools import lru_cache

logger = logging.getLogger("VibePilot.ConfigLoader")


class ConfigLoader:
    """
    Loads and caches all VibePilot configuration files.

    Config files are in ~/vibepilot/config/:
        - skills.json
        - tools.json
        - models.json
        - platforms.json
        - agents.json
        - prompts/*.md
    """

    DEFAULT_CONFIG_DIR = None

    def __init__(self, config_dir: str = None):
        if config_dir:
            self.config_dir = Path(config_dir)
        else:
            self.config_dir = self._find_config_dir()

        self._cache: Dict[str, Any] = {}
        self._prompt_cache: Dict[str, str] = {}

    def _find_config_dir(self) -> Path:
        """Find the config directory."""
        if self.DEFAULT_CONFIG_DIR:
            return Path(self.DEFAULT_CONFIG_DIR)

        current = Path(__file__).parent
        while current != current.parent:
            candidate = current / "vibepilot" / "config"
            if candidate.exists():
                return candidate
            candidate = current / "config"
            if candidate.exists():
                return candidate
            current = current.parent

        return Path.home() / "vibepilot" / "config"

    def _load_json(self, filename: str) -> Dict[str, Any]:
        """Load and cache a JSON config file."""
        if filename in self._cache:
            return self._cache[filename]

        filepath = self.config_dir / filename
        if not filepath.exists():
            logger.warning(f"Config file not found: {filepath}")
            return {}

        with open(filepath) as f:
            data = json.load(f)

        self._cache[filename] = data
        logger.debug(f"Loaded config: {filename}")
        return data

    def _load_prompt(self, prompt_file: str) -> str:
        """Load and cache a prompt file."""
        if prompt_file in self._prompt_cache:
            return self._prompt_cache[prompt_file]

        filepath = self.config_dir / prompt_file
        if not filepath.exists():
            logger.warning(f"Prompt file not found: {filepath}")
            return ""

        with open(filepath) as f:
            content = f.read()

        self._prompt_cache[prompt_file] = content
        return content

    def reload(self):
        """Clear cache and reload all configs."""
        self._cache.clear()
        self._prompt_cache.clear()
        logger.info("Config cache cleared")

    def get_skills(self) -> List[Dict[str, Any]]:
        """Get all skills from skills.json."""
        data = self._load_json("skills.json")
        return data.get("skills", [])

    def get_skill(self, skill_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific skill by ID."""
        for skill in self.get_skills():
            if skill.get("id") == skill_id:
                return skill
        return None

    def get_tools(self) -> List[Dict[str, Any]]:
        """Get all tools from tools.json."""
        data = self._load_json("tools.json")
        return data.get("tools", [])

    def get_tool(self, tool_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific tool by ID."""
        for tool in self.get_tools():
            if tool.get("id") == tool_id:
                return tool
        return None

    def get_models(self) -> List[Dict[str, Any]]:
        """Get all models from models.json."""
        data = self._load_json("models.json")
        return data.get("models", [])

    def get_model(self, model_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific model by ID."""
        for model in self.get_models():
            if model.get("id") == model_id:
                return model
        return None

    def get_active_models(self) -> List[Dict[str, Any]]:
        """Get all active models."""
        return [m for m in self.get_models() if m.get("status", "active") == "active"]

    def get_platforms(self) -> List[Dict[str, Any]]:
        """Get all platforms from platforms.json."""
        data = self._load_json("platforms.json")
        return data.get("platforms", [])

    def get_platform(self, platform_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific platform by ID."""
        for platform in self.get_platforms():
            if platform.get("id") == platform_id:
                return platform
        return None

    def get_agents(self) -> List[Dict[str, Any]]:
        """Get all agents from agents.json."""
        data = self._load_json("agents.json")
        return data.get("agents", [])

    def get_agent(self, agent_id: str) -> Optional[Dict[str, Any]]:
        """Get a specific agent by ID."""
        for agent in self.get_agents():
            if agent.get("id") == agent_id:
                return agent
        return None

    def get_agent_prompt(self, agent_id: str) -> str:
        """Get the prompt for a specific agent."""
        agent = self.get_agent(agent_id)
        if not agent:
            return ""

        prompt_file = agent.get("prompt", "")
        if not prompt_file:
            return ""

        return self._load_prompt(prompt_file)

    def get_agent_with_prompt(self, agent_id: str) -> Dict[str, Any]:
        """Get agent with resolved prompt content."""
        agent = self.get_agent(agent_id)
        if not agent:
            return {}

        result = dict(agent)
        result["prompt_content"] = self.get_agent_prompt(agent_id)

        skills = []
        for skill_id in agent.get("skills", []):
            skill = self.get_skill(skill_id)
            if skill:
                skills.append(skill)
        result["resolved_skills"] = skills

        tools = []
        for tool_id in agent.get("tools", []):
            tool = self.get_tool(tool_id)
            if tool:
                tools.append(tool)
        result["resolved_tools"] = tools

        return result

    def get_limit_policy(self) -> Dict[str, Any]:
        """Get the limit policy from models.json."""
        data = self._load_json("models.json")
        return data.get("limit_policy", {"pause_at_percent": 80})

    def get_model_for_task(self, task_requirements: Dict[str, Any]) -> Optional[str]:
        """
        Find the best model for a task based on requirements.

        Args:
            task_requirements: Dict with:
                - access_type: "courier"|"api"|"cli_subscription"
                - context_needed: int (tokens)
                - features: list (e.g., ["vision", "code"])

        Returns:
            Model ID or None if no match
        """
        access_type = task_requirements.get("access_type")
        context_needed = task_requirements.get("context_needed", 0)
        features = set(task_requirements.get("features", []))

        candidates = []
        for model in self.get_models():
            if model.get("status", "active") != "active":
                continue

            if access_type and model.get("access_type") != access_type:
                continue

            if model.get("context_limit", 0) < context_needed:
                continue

            score = 0
            model_features = set(model.get("features", []))
            if features and features.issubset(model_features):
                score += 10

            score -= model.get("credit_used", 0) / max(
                model.get("credit_remaining", 1), 0.01
            )

            candidates.append((model["id"], score))

        if not candidates:
            return None

        candidates.sort(key=lambda x: x[1], reverse=True)
        return candidates[0][0]

    def validate_configs(self) -> Dict[str, Any]:
        """
        Validate all config files are valid and consistent.

        Returns:
            Dict with "valid" bool and "errors" list
        """
        errors = []

        skills = {s["id"] for s in self.get_skills()}
        tools = {t["id"] for t in self.get_tools()}
        models = {m["id"] for m in self.get_models()}

        for agent in self.get_agents():
            agent_id = agent.get("id", "unknown")

            for skill_id in agent.get("skills", []):
                if skill_id not in skills:
                    errors.append(
                        f"Agent '{agent_id}' references unknown skill '{skill_id}'"
                    )

            for tool_id in agent.get("tools", []):
                if tool_id not in tools:
                    errors.append(
                        f"Agent '{agent_id}' references unknown tool '{tool_id}'"
                    )

            model_id = agent.get("model")
            if model_id and model_id not in models:
                errors.append(
                    f"Agent '{agent_id}' references unknown model '{model_id}'"
                )

            prompt_file = agent.get("prompt", "")
            if prompt_file:
                prompt_path = self.config_dir / prompt_file
                if not prompt_path.exists():
                    errors.append(
                        f"Agent '{agent_id}' prompt file not found: {prompt_file}"
                    )

        return {
            "valid": len(errors) == 0,
            "errors": errors,
            "stats": {
                "skills": len(skills),
                "tools": len(tools),
                "models": len(models),
                "agents": len(self.get_agents()),
                "platforms": len(self.get_platforms()),
            },
        }


_config_loader: Optional[ConfigLoader] = None


def get_config_loader(config_dir: str = None) -> ConfigLoader:
    """Get the singleton config loader instance."""
    global _config_loader
    if _config_loader is None:
        _config_loader = ConfigLoader(config_dir)
    return _config_loader


if __name__ == "__main__":
    import sys

    config = ConfigLoader()

    print("=== VibePilot Config Validation ===\n")

    result = config.validate_configs()

    if result["valid"]:
        print("✓ All configs valid\n")
    else:
        print("✗ Config errors found:\n")
        for error in result["errors"]:
            print(f"  - {error}")
        print()

    print("Config stats:")
    for key, value in result["stats"].items():
        print(f"  {key}: {value}")

    print("\n=== Sample Agent (Planner) ===\n")
    planner = config.get_agent_with_prompt("planner")
    if planner:
        print(f"Name: {planner.get('name')}")
        print(f"Model: {planner.get('model')}")
        print(f"Skills: {[s['id'] for s in planner.get('resolved_skills', [])]}")
        print(f"Tools: {[t['id'] for t in planner.get('resolved_tools', [])]}")
        prompt_preview = planner.get("prompt_content", "")[:200]
        print(f"Prompt preview: {prompt_preview}...")
    else:
        print("Planner agent not found")
