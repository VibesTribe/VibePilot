# Standardized Model Comparison Benchmark Tasks

**Date:** 2026-02-18  
**Purpose:** Generic, challenging tasks for comparing AI models (Kimi vs GLM, or any platform evaluation)

---

## Task 1: Code Challenge - "The Traffic Light Controller"

**Difficulty:** Medium-Hard  
**Time Limit:** 15 minutes  
**Evaluation Criteria:** Correctness, edge cases, code quality, documentation

### Problem Statement

Design a traffic light controller for a 4-way intersection that:

1. Manages 4 directions (North, South, East, West)
2. Each direction has: Red, Yellow, Green lights
3. Implements configurable timing (green duration, yellow duration)
4. **Safety constraint:** Never show green in opposing directions simultaneously
5. **Optimization:** Minimize average wait time for vehicles
6. **Edge case:** Handle pedestrian crossing requests (pause all traffic)

### Requirements

```python
class TrafficLightController:
    def __init__(self, config: dict):
        """
        config = {
            'north_south': {'green': 30, 'yellow': 5},
            'east_west': {'green': 25, 'yellow': 5},
            'all_red_delay': 2  # Safety buffer between switches
        }
        """
        pass
    
    def get_current_state(self) -> dict:
        """Returns current light state for all directions"""
        pass
    
    def tick(self) -> dict:
        """Advance one time unit, return new state"""
        pass
    
    def request_pedestrian_crossing(self) -> None:
        """Signal to pause all traffic for pedestrians"""
        pass
    
    def get_average_wait_time(self) -> float:
        """Calculate average vehicle wait time"""
        pass
```

### Evaluation Rubric

| Criterion | Points | What to Look For |
|-----------|--------|------------------|
| **Correctness** | 30 | Never opposing greens, proper timing |
| **Safety** | 20 | All-red buffer, pedestrian handling |
| **Optimization** | 20 | Efficient timing, minimizes wait |
| **Edge Cases** | 15 | Empty intersection, stuck detection |
| **Code Quality** | 15 | Clean, documented, testable |

### Why This Task?

- **State management:** Complex state transitions
- **Safety critical:** Real-world consequences
- **Optimization:** Multiple valid solutions, trade-offs
- **Concurrency concept:** Without actual threading complexity
- **Generic:** No domain-specific knowledge needed

---

## Task 2: Strategic Analysis - "The Platform Dilemma"

**Difficulty:** Hard  
**Time Limit:** 20 minutes  
**Evaluation Criteria:** Depth of analysis, logical reasoning, actionable insights, trade-off clarity

### Scenario

You're building a SaaS product that uses AI for:
- Customer support chat (high volume, simple queries)
- Code review (medium volume, complex reasoning)
- Documentation generation (batch jobs, non-urgent)

**Available Options:**

| Platform | Cost | Quality | Speed | Reliability | Context |
|----------|------|---------|-------|-------------|---------|
| **Platform A** | $0.002/1K tokens | 9/10 | Fast | 99.9% | 128K |
| **Platform B** | $0.0005/1K tokens | 7/10 | Medium | 99.5% | 64K |
| **Platform C** | Free (100/day) | 8/10 | Slow | 95% | 200K |
| **Platform D** | $0.001/1K tokens | 6/10 | Fast | 98% | 32K |

**Constraints:**
- Monthly budget: $500
- Expected volume: 50K support chats, 5K code reviews, 1K doc jobs
- Support chats must be fast (<2s response)
- Code reviews need 200K context for large PRs
- Downtime on support is unacceptable

### The Question

**Design a routing strategy that:**
1. Stays within budget
2. Meets performance requirements
3. Minimizes risk of downtime
4. Optimizes for each use case
5. Includes fallback plans

**Deliver:**
- Routing matrix (which platform for which use case)
- Budget breakdown
- Risk mitigation strategy
- Monitoring approach
- Decision framework for future platform additions

### Evaluation Rubric

| Criterion | Points | What to Look For |
|-----------|--------|------------------|
| **Budget Math** | 25 | Accurate cost calculations, within $500 |
| **Constraint Satisfaction** | 25 | Meets speed, context, reliability needs |
| **Risk Management** | 20 | Fallbacks, graceful degradation |
| **Optimization** | 15 | Smart routing, not just cheapest |
| **Future-proofing** | 15 | Extensible framework, monitoring |

### Why This Task?

- **Real-world complexity:** Multiple constraints, no perfect solution
- **Trade-off analysis:** Cost vs quality vs speed
- **Systems thinking:** Interconnected decisions
- **Strategic planning:** Long-term thinking, not just immediate fix
- **Generic:** Applies to any multi-platform AI system

---

## Task 3: Debugging Challenge - "The Race Condition"

**Difficulty:** Hard  
**Time Limit:** 15 minutes  
**Evaluation Criteria:** Bug identification, root cause analysis, fix quality, prevention

### The Buggy Code

```python
import threading
import time

class TaskQueue:
    def __init__(self):
        self.tasks = []
        self.processing = set()
        self.completed = 0
        self.lock = threading.Lock()
    
    def add_task(self, task_id):
        with self.lock:
            self.tasks.append(task_id)
    
    def get_next_task(self):
        with self.lock:
            if self.tasks:
                task = self.tasks.pop(0)
                self.processing.add(task)
                return task
            return None
    
    def complete_task(self, task_id):
        with self.lock:
            self.processing.remove(task_id)
            self.completed += 1
    
    def get_stats(self):
        return {
            'pending': len(self.tasks),
            'processing': len(self.processing),
            'completed': self.completed
        }

def worker(queue, worker_id):
    while True:
        task = queue.get_next_task()
        if task is None:
            time.sleep(0.1)
            continue
        
        # Simulate work
        time.sleep(0.01)
        
        # This sometimes raises KeyError!
        queue.complete_task(task)
        print(f"Worker {worker_id} completed task {task}")

# Usage
queue = TaskQueue()
for i in range(100):
    queue.add_task(i)

threads = [threading.Thread(target=worker, args=(queue, i)) for i in range(5)]
for t in threads:
    t.start()
```

### The Problem

Intermittent `KeyError` in `complete_task()`:
```
KeyError: 42
```

**Questions:**
1. What causes the race condition?
2. Why is it intermittent?
3. How do you fix it?
4. How do you prevent similar bugs?

### Evaluation Rubric

| Criterion | Points | What to Look For |
|-----------|--------|------------------|
| **Root Cause** | 30 | Identifies exact race condition |
| **Fix Correctness** | 30 | Thread-safe solution |
| **Explanation** | 20 | Clear why it happens |
| **Prevention** | 20 | Static analysis, patterns, testing |

### Why This Task?

- **Concurrency:** Hard topic, separates good from great
- **Debugging skill:** Real-world intermittent bugs
- **Thread safety:** Critical for production systems
- **Explains why:** Not just fixes, but understands

---

## Task 4: Creative Problem Solving - "The Island Bridge"

**Difficulty:** Medium  
**Time Limit:** 10 minutes  
**Evaluation Criteria:** Creative thinking, feasibility, optimization, presentation

### The Challenge

You have 4 islands arranged in a square, 1km apart:

```
A --- 1km --- B
|             |
1km         1km
|             |
D --- 1km --- C
```

**Goal:** Build bridges so you can travel from any island to any other island.

**Constraints:**
- Bridge cost: $1 million per km
- You have $3.2 million budget
- Bridges can only connect islands (no intermediate points)
- All islands must be reachable from all others

**Questions:**
1. What's the cheapest solution?
2. What's the most resilient solution (if one bridge fails)?
3. What's the fastest travel time solution?
4. Given $3.2M, which solution do you choose and why?

### Evaluation Rubric

| Criterion | Points | What to Look For |
|-----------|--------|------------------|
| **Correctness** | 25 | All islands connected |
| **Budget** | 25 | Stays within $3.2M |
| **Trade-offs** | 25 | Understands cost/resilience/speed trade-offs |
| **Creativity** | 15 | Multiple solutions considered |
| **Justification** | 10 | Clear reasoning for final choice |

### Why This Task?

- **Graph theory:** Basic but important concept
- **Optimization:** Multiple valid solutions
- **Constraints:** Real-world limitations
- **Communication:** Explains reasoning clearly

---

## Comparison Framework

### For Each Task, Evaluate:

**Kimi vs GLM (or any two models):**

1. **Correctness** - Who got it right?
2. **Speed** - Who finished faster?
3. **Depth** - Who went deeper?
4. **Edge Cases** - Who thought of more?
5. **Clarity** - Whose solution is easier to understand?
6. **Creativity** - Who had novel approaches?

### Scoring Sheet

```
Task 1 (Traffic Light):
  Kimi:  [Correctness] [Safety] [Optimization] [Edge Cases] [Quality] = Total
  GLM:   [Correctness] [Safety] [Optimization] [Edge Cases] [Quality] = Total

Task 2 (Platform Dilemma):
  Kimi:  [Budget] [Constraints] [Risk] [Optimization] [Future] = Total
  GLM:   [Budget] [Constraints] [Risk] [Optimization] [Future] = Total

Task 3 (Race Condition):
  Kimi:  [Root Cause] [Fix] [Explanation] [Prevention] = Total
  GLM:   [Root Cause] [Fix] [Explanation] [Prevention] = Total

Task 4 (Island Bridge):
  Kimi:  [Correctness] [Budget] [Trade-offs] [Creativity] [Justify] = Total
  GLM:   [Correctness] [Budget] [Trade-offs] [Creativity] [Justify] = Total

OVERALL WINNER: _______
```

---

## How to Run the Comparison

### Option 1: Simultaneous (Fair)
1. Post both tasks to AGENT_CHAT.md
2. Both agents work independently
3. Compare results side-by-side
4. Human judges (you)

### Option 2: Arena Style (LM Arena)
1. Post tasks to LM Arena
2. Both models respond to same prompts
3. Vote which is better
4. Repeat for statistical significance

### Option 3: Time Trial (Speed vs Quality)
1. Same task, race to completion
2. Evaluate both results
3. Factor in speed vs quality trade-off

---

## Additional Benchmark Ideas

### For Code Tasks:
- **JSON Parser** - Parse nested JSON with error handling
- **Rate Limiter** - Token bucket algorithm
- **Cache Implementation** - LRU cache with TTL
- **API Client** - Retry logic, exponential backoff
- **Data Pipeline** - ETL with validation

### For Analysis Tasks:
- **Tech Stack Comparison** - Compare 3 frameworks for a use case
- **Scaling Strategy** - How to handle 10x traffic
- **Security Review** - Find vulnerabilities in code
- **Refactoring Plan** - Improve legacy codebase
- **Migration Strategy** - Move from Monolith to Microservices

### For Debugging:
- **Memory Leak** - Find and fix in Python
- **Performance Bottleneck** - Optimize slow function
- **Logic Error** - Subtle bug in business rules
- **Deadlock** - Threading issue resolution
- **Off-by-One** - Classic boundary error

---

## Conclusion

These tasks are:
- ✅ **Domain-agnostic** - No VibePilot knowledge needed
- ✅ **Challenging** - Separate good from great
- ✅ **Measurable** - Clear evaluation criteria
- ✅ **Fair** - Same task, same constraints
- ✅ **Useful** - Skills that transfer to real work

**Ready to run the comparison?** Pick 2 tasks and let's see who wins: Kimi vs GLM! 🎯
