from .base import Agent, AgentResult
from typing import Dict, Any


class ConsultantAgent(Agent):
    name = "Consultant"
    role = "Spec Engineer - Turns vague ideas into hardened PRDs"
    
    def execute(self, task: Dict[str, Any]) -> AgentResult:
        idea = task.get("description", "")
        
        if not idea:
            return AgentResult(
                success=False,
                output=None,
                error="No idea description provided"
            )
        
        self.log("Analyzing idea and generating PRD...")
        
        prompt = f"""You are The Consultant - a spec engineer. Your job is to turn vague ideas into hardened Product Requirements Documents (PRDs).

USER IDEA:
{idea}

Generate a complete PRD with these sections:
1. OVERVIEW - What is this product?
2. OBJECTIVES - What are the main goals?
3. TECHNICAL STACK - Recommended technologies (be specific)
4. FEATURES - List core features with priorities (P0, P1, P2)
5. ARCHITECTURE - High-level system design
6. SECURITY REQUIREMENTS - Auth, data protection, etc.
7. DEPLOYMENT STRATEGY - Hosting, CI/CD, monitoring
8. SUCCESS METRICS - How do we measure success?

Be specific and technical. No fluff."""

        try:
            prd = self.call_llm(prompt, max_tokens=3000)
            self.log("PRD generated successfully")
            
            return AgentResult(
                success=True,
                output=prd,
                metadata={"type": "prd", "source_idea": idea[:100]}
            )
        except Exception as e:
            self.log(f"Failed to generate PRD: {e}", level="error")
            return AgentResult(
                success=False,
                output=None,
                error=str(e)
            )
