"""
VibePilot Telemetry System

OpenTelemetry-based observability for all agent operations.
Tracks every LLM call, tool invocation, and task execution.

Usage:
    from core.telemetry import Telemetry, traced

    telemetry = Telemetry(service_name="vibepilot")
    telemetry.init()

    # Decorator approach
    @traced("task_execution")
    def execute_task(task_id):
        ...

    # Context manager approach
    with telemetry.span("llm_call", {"model": "kimi"}) as span:
        result = model.generate(prompt)
        span.set_attribute("tokens", result.tokens)
"""

import os
import json
import time
import logging
from typing import Dict, Any, Optional, Callable
from datetime import datetime
from functools import wraps
from contextlib import contextmanager

logger = logging.getLogger("VibePilot.Telemetry")

try:
    from opentelemetry import trace
    from opentelemetry.sdk.trace import TracerProvider
    from opentelemetry.sdk.trace.export import BatchSpanProcessor, ConsoleSpanExporter
    from opentelemetry.sdk.resources import Resource

    OPENTELEMETRY_AVAILABLE = True
except ImportError:
    OPENTELEMETRY_AVAILABLE = False
    logger.debug("OpenTelemetry not installed. Using fallback logging.")


class FallbackSpan:
    """Fallback span when OpenTelemetry is not available."""

    def __init__(self, name: str, attributes: Dict = None):
        self.name = name
        self.attributes = attributes or {}
        self.start_time = time.time()
        self.end_time = None

    def set_attribute(self, key: str, value: Any):
        self.attributes[key] = value

    def add_event(self, name: str, attributes: Dict = None):
        logger.info(f"[{self.name}] Event: {name} - {attributes}")

    def __enter__(self):
        logger.debug(f"[{self.name}] Span started")
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.end_time = time.time()
        duration_ms = (self.end_time - self.start_time) * 1000
        logger.debug(
            f"[{self.name}] Span ended ({duration_ms:.2f}ms) - {self.attributes}"
        )
        return False


class FallbackTracer:
    """Fallback tracer using standard logging."""

    def start_span(self, name: str, attributes: Dict = None) -> FallbackSpan:
        return FallbackSpan(name, attributes)

    @contextmanager
    def start_as_current_span(self, name: str, attributes: Dict = None):
        span = self.start_span(name, attributes)
        with span as s:
            yield s


class Telemetry:
    """
    VibePilot Telemetry System.

    Provides observability for:
    - LLM calls (tokens, cost, latency)
    - Tool invocations (what, when, result)
    - Task execution (status, duration, errors)
    - Agent operations (planning, review, approval)

    Export formats:
    - Console (development)
    - JSON file (production fallback)
    - OTLP (future: Grafana, Jaeger)
    """

    _instance = None

    def __init__(self, service_name: str = "vibepilot", enabled: bool = True):
        self.service_name = service_name
        self.enabled = enabled
        self.tracer = None
        self.spans_file = os.path.join(
            os.path.dirname(__file__), "..", "logs", "telemetry.jsonl"
        )

        os.makedirs(os.path.dirname(self.spans_file), exist_ok=True)

    def init(self):
        """Initialize telemetry. Call once at startup."""
        if not self.enabled:
            logger.info("Telemetry disabled")
            return

        if OPENTELEMETRY_AVAILABLE:
            resource = Resource.create({"service.name": self.service_name})
            provider = TracerProvider(resource=resource)

            provider.add_span_processor(BatchSpanProcessor(ConsoleSpanExporter()))

            trace.set_tracer_provider(provider)
            self.tracer = trace.get_tracer(self.service_name)

            logger.info("OpenTelemetry initialized")
        else:
            self.tracer = FallbackTracer()
            logger.info("Fallback telemetry initialized (logging only)")

    @contextmanager
    def span(self, name: str, attributes: Dict = None):
        """Context manager for tracing a span."""
        if not self.enabled:
            yield None
            return

        if self.tracer is None:
            self.init()

        attrs = attributes or {}
        attrs["service"] = self.service_name
        attrs["timestamp"] = datetime.utcnow().isoformat()

        with self.tracer.start_as_current_span(name, attributes=attrs) as s:
            start_time = time.time()
            try:
                yield s
            finally:
                duration_ms = (time.time() - start_time) * 1000
                if hasattr(s, "set_attribute"):
                    s.set_attribute("duration_ms", duration_ms)

                self._log_span(name, attrs, duration_ms)

    def _log_span(self, name: str, attributes: Dict, duration_ms: float):
        """Log span to file for persistence."""
        try:
            record = {
                "name": name,
                "attributes": attributes,
                "duration_ms": duration_ms,
                "timestamp": datetime.utcnow().isoformat(),
            }

            with open(self.spans_file, "a") as f:
                f.write(json.dumps(record) + "\n")
        except Exception as e:
            logger.warning(f"Failed to log span: {e}")

    def record_llm_call(
        self,
        model: str,
        prompt_tokens: int,
        completion_tokens: int,
        latency_ms: float,
        success: bool,
        error: str = None,
    ):
        """Record an LLM API call."""
        with self.span(
            "llm_call",
            {
                "model": model,
                "prompt_tokens": prompt_tokens,
                "completion_tokens": completion_tokens,
                "total_tokens": prompt_tokens + completion_tokens,
                "latency_ms": latency_ms,
                "success": success,
                "error": error,
            },
        ) as span:
            pass

    def record_tool_call(
        self,
        tool_name: str,
        inputs: Dict,
        outputs: Any,
        latency_ms: float,
        success: bool,
    ):
        """Record a tool invocation."""
        with self.span(
            "tool_call",
            {
                "tool": tool_name,
                "inputs": json.dumps(inputs)[:500],
                "success": success,
                "latency_ms": latency_ms,
            },
        ) as span:
            if span:
                span.set_attribute("output_size", len(str(outputs)))

    def record_task_execution(
        self,
        task_id: str,
        task_type: str,
        model: str,
        duration_ms: float,
        success: bool,
        tokens: int = 0,
    ):
        """Record a task execution."""
        with self.span(
            "task_execution",
            {
                "task_id": task_id,
                "task_type": task_type,
                "model": model,
                "duration_ms": duration_ms,
                "success": success,
                "tokens": tokens,
            },
        ) as span:
            pass

    def record_agent_operation(
        self, agent: str, operation: str, duration_ms: float, success: bool
    ):
        """Record an agent operation (planning, review, etc)."""
        with self.span(
            "agent_operation",
            {
                "agent": agent,
                "operation": operation,
                "duration_ms": duration_ms,
                "success": success,
            },
        ) as span:
            pass


_telemetry_instance: Optional[Telemetry] = None


def get_telemetry() -> Telemetry:
    """Get the global telemetry instance."""
    global _telemetry_instance
    if _telemetry_instance is None:
        _telemetry_instance = Telemetry()
        _telemetry_instance.init()
    return _telemetry_instance


def traced(name: str = None):
    """
    Decorator for tracing functions.

    Usage:
        @traced("my_function")
        def my_function(arg1, arg2):
            ...

        @traced()  # Uses function name
        def another_function():
            ...
    """

    def decorator(func: Callable) -> Callable:
        span_name = name or func.__name__

        @wraps(func)
        def wrapper(*args, **kwargs):
            telemetry = get_telemetry()
            start_time = time.time()
            success = True
            error = None

            try:
                with telemetry.span(span_name) as span:
                    result = func(*args, **kwargs)
                    return result
            except Exception as e:
                success = False
                error = str(e)
                raise
            finally:
                duration_ms = (time.time() - start_time) * 1000
                if not success:
                    logger.error(f"[{span_name}] Failed: {error}")

        return wrapper

    return decorator


class LoopDetector:
    """
    Detects loops and stuck states from telemetry data.
    Used by Watcher agent.
    """

    def __init__(self, telemetry_file: str = None):
        self.telemetry_file = telemetry_file or os.path.join(
            os.path.dirname(__file__), "..", "logs", "telemetry.jsonl"
        )

    def detect_loops(self, window_minutes: int = 30) -> Dict:
        """
        Analyze recent telemetry for loop patterns.

        Returns:
            {
                "loops_detected": bool,
                "patterns": [...],
                "recommendations": [...]
            }
        """
        patterns = []

        try:
            spans = self._load_recent_spans(window_minutes)

            same_tool_calls = self._detect_repeated_tool_calls(spans)
            if same_tool_calls:
                patterns.append(
                    {
                        "type": "repeated_tool",
                        "details": same_tool_calls,
                        "severity": "medium",
                    }
                )

            same_error = self._detect_repeated_errors(spans)
            if same_error:
                patterns.append(
                    {
                        "type": "repeated_error",
                        "details": same_error,
                        "severity": "high",
                    }
                )

            long_running = self._detect_long_running(spans)
            if long_running:
                patterns.append(
                    {
                        "type": "long_running",
                        "details": long_running,
                        "severity": "medium",
                    }
                )

            token_waste = self._detect_token_waste(spans)
            if token_waste:
                patterns.append(
                    {"type": "token_waste", "details": token_waste, "severity": "low"}
                )

        except Exception as e:
            logger.error(f"Loop detection failed: {e}")

        recommendations = self._generate_recommendations(patterns)

        return {
            "loops_detected": len(patterns) > 0,
            "patterns": patterns,
            "recommendations": recommendations,
        }

    def _load_recent_spans(self, window_minutes: int) -> list:
        """Load spans from the last N minutes."""
        if not os.path.exists(self.telemetry_file):
            return []

        cutoff = datetime.utcnow().timestamp() - (window_minutes * 60)
        spans = []

        try:
            with open(self.telemetry_file, "r") as f:
                for line in f:
                    try:
                        span = json.loads(line)
                        ts = datetime.fromisoformat(
                            span.get("timestamp", "")
                        ).timestamp()
                        if ts >= cutoff:
                            spans.append(span)
                    except:
                        continue
        except:
            pass

        return spans

    def _detect_repeated_tool_calls(self, spans: list) -> Optional[Dict]:
        """Detect same tool called multiple times with same inputs."""
        tool_calls = {}

        for span in spans:
            if span.get("name") == "tool_call":
                attrs = span.get("attributes", {})
                tool = attrs.get("tool")
                inputs = attrs.get("inputs", "")

                key = f"{tool}:{inputs[:100]}"
                tool_calls[key] = tool_calls.get(key, 0) + 1

        repeated = {k: v for k, v in tool_calls.items() if v >= 3}

        if repeated:
            return {"repeated_calls": repeated}
        return None

    def _detect_repeated_errors(self, spans: list) -> Optional[Dict]:
        """Detect same error occurring multiple times."""
        errors = {}

        for span in spans:
            attrs = span.get("attributes", {})
            if attrs.get("success") is False:
                error = attrs.get("error", "unknown")
                errors[error] = errors.get(error, 0) + 1

        repeated = {k: v for k, v in errors.items() if v >= 3}

        if repeated:
            return {"repeated_errors": repeated}
        return None

    def _detect_long_running(self, spans: list) -> Optional[Dict]:
        """Detect spans running longer than expected."""
        long_spans = []

        for span in spans:
            duration = span.get("attributes", {}).get("duration_ms", 0)
            if duration > 300000:  # 5 minutes
                long_spans.append({"name": span.get("name"), "duration_ms": duration})

        if long_spans:
            return {"long_running_spans": long_spans}
        return None

    def _detect_token_waste(self, spans: list) -> Optional[Dict]:
        """Detect potential token waste patterns."""
        llm_calls = [s for s in spans if s.get("name") == "llm_call"]

        if len(llm_calls) < 5:
            return None

        total_tokens = sum(
            s.get("attributes", {}).get("total_tokens", 0) for s in llm_calls
        )

        unique_models = len(
            set(s.get("attributes", {}).get("model") for s in llm_calls)
        )

        if total_tokens > 500000 and unique_models == 1:
            return {
                "total_tokens": total_tokens,
                "single_model": True,
                "suggestion": "Consider using caching or splitting context",
            }

        return None

    def _generate_recommendations(self, patterns: list) -> list:
        """Generate action recommendations based on detected patterns."""
        recommendations = []

        for pattern in patterns:
            ptype = pattern.get("type")
            severity = pattern.get("severity")

            if ptype == "repeated_error" and severity == "high":
                recommendations.append(
                    {
                        "action": "kill_and_reassign",
                        "reason": "Same error repeated 3+ times",
                    }
                )
            elif ptype == "repeated_tool" and severity == "medium":
                recommendations.append(
                    {
                        "action": "investigate_tool",
                        "reason": "Tool called repeatedly with same inputs",
                    }
                )
            elif ptype == "long_running":
                recommendations.append(
                    {"action": "check_timeout", "reason": "Task running over 5 minutes"}
                )
            elif ptype == "token_waste":
                recommendations.append(
                    {
                        "action": "enable_caching",
                        "reason": "High token usage without model diversity",
                    }
                )

        return recommendations


if __name__ == "__main__":
    print("=== VibePilot Telemetry ===\n")

    telemetry = Telemetry()
    telemetry.init()

    print("Testing LLM call recording...")
    telemetry.record_llm_call(
        model="kimi-k2.5",
        prompt_tokens=1000,
        completion_tokens=500,
        latency_ms=2500,
        success=True,
    )

    print("Testing tool call recording...")
    telemetry.record_tool_call(
        tool_name="git_commit",
        inputs={"message": "Add telemetry"},
        outputs={"success": True},
        latency_ms=150,
        success=True,
    )

    print("\nTesting loop detector...")
    detector = LoopDetector()
    result = detector.detect_loops()
    print(f"Loops detected: {result['loops_detected']}")

    print("\n✅ Telemetry ready for observability")
