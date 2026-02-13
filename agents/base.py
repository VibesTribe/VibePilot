import os
import logging
import requests
from dataclasses import dataclass
from typing import Optional, Dict, Any
from abc import ABC, abstractmethod

logger = logging.getLogger("VibePilot")

DS_KEY = os.getenv("DEEPSEEK_KEY")
DS_URL = "https://api.deepseek.com/v1/chat/completions"


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
    
    def call_llm(self, prompt: str, max_tokens: int = 2000) -> str:
        if not DS_KEY:
            raise ValueError("DEEPSEEK_KEY not configured")
        
        headers = {
            "Authorization": f"Bearer {DS_KEY}",
            "Content-Type": "application/json"
        }
        payload = {
            "model": "deepseek-chat",
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": max_tokens
        }
        
        try:
            r = requests.post(DS_URL, headers=headers, json=payload, timeout=60)
            if r.status_code == 200:
                return r.json()["choices"][0]["message"]["content"]
            else:
                raise Exception(f"API Error [{r.status_code}]: {r.text[:200]}")
        except requests.Timeout:
            raise Exception("LLM request timed out")
    
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
