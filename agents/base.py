import os
import logging
from dataclasses import dataclass
from typing import Optional, Dict, Any
from abc import ABC, abstractmethod

logger = logging.getLogger("VibePilot")


@dataclass
class AgentResult:
    success: bool
    output: Any
    error: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class Agent(ABC):
    name: str = "BaseAgent"
    role: str = "Undefined"
    
    def __init__(self):
        self.logger = logging.getLogger(f"VibePilot.{self.name}")
        self.orchestrator = None  # Will be wired by orchestrator
    
    def set_orchestrator(self, orchestrator):
        """Wire orchestrator reference for LLM routing."""
        self.orchestrator = orchestrator
    
    def call_llm(self, prompt: str, max_tokens: int = 2000) -> str:
        """
        Route LLM call through orchestrator's runner pool.
        
        This ensures:
        - Proper model selection based on availability
        - Rate limit management
        - Token tracking
        - Fallback handling
        """
        if self.orchestrator:
            # Route through orchestrator for proper runner pool management
            result = self.orchestrator.run_agent_task(
                agent_role=self.role,
                prompt=prompt,
                max_tokens=max_tokens
            )
            if result.get("success"):
                return result.get("output", "")
            else:
                error = result.get("error", "Unknown error")
                self.logger.error(f"LLM call failed: {error}")
                raise Exception(f"LLM call failed: {error}")
        else:
            # Fallback: Use Kimi CLI directly (no orchestrator wired yet)
            self.logger.warning("No orchestrator wired, using Kimi CLI fallback")
            return self._call_llm_fallback(prompt, max_tokens)
    
    def _call_llm_fallback(self, prompt: str, max_tokens: int = 2000) -> str:
        """
        Fallback LLM call using Kimi CLI when orchestrator not available.
        This should only be used during testing or initialization.
        """
        try:
            # Try to use kimi_runner directly
            from runners.kimi_runner import KimiRunner
            runner = KimiRunner()
            result = runner.execute({
                "prompt": prompt,
                "max_tokens": max_tokens
            })
            
            if result.get("status") == "success":
                return result.get("output", "")
            else:
                error = result.get("error", "Unknown error")
                raise Exception(f"Kimi fallback failed: {error}")
                
        except Exception as e:
            self.logger.error(f"Fallback LLM call failed: {e}")
            raise Exception(
                f"No orchestrator wired and fallback failed: {e}. "
                "Ensure agent.set_orchestrator() is called before call_llm()."
            )
    
    @abstractmethod
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        pass
    
    def log(self, message: str, level: str = "info"):
        msg = f"[{self.name}] {message}"
        if level == "error":
            self.logger.error(msg)
        elif level == "warning":
            self.logger.warning(msg)
        else:
            self.logger.info(msg)
