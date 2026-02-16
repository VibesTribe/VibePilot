# Session Notes - 2026-02-16

## What Went Wrong (Critical Learning)

### Near Type 1 Errors Caught

1. **Hardcoding agents/workers** - I hardcoded `max_workers=5`, then "20 tasks", fixed numbers everywhere. User caught it. Should be dynamic based on available runners.

2. **Task packet templates** - I created a schema but no actual templates. Without consistent structure, every task packet would be different → no quality control → tracking impossible.

3. **Courier confusion** - I described couriers as "when API runners aren't suitable" - WRONG. Couriers are for web platforms ALWAYS, bring back chat URLs + model info + task number.

4. **Planning before understanding** - I created a 20-task plan before:
   - Reviewing what's already built
   - Understanding Vibeflow patterns
   - Clarifying requirements with user

### Root Cause

Acting before reading. Solving before understanding.

The user had to stop me from solidifying ambiguity into code.

## What User Taught Me

- "Prevention = 1% of cure cost"
- Type 1 errors ruin everything downstream
- If the plan has ambiguity, Council can't save it
- Never hardcode what should be dynamic
- Task packets need TEMPLATES, not just schemas
- Couriers: web platforms, free tiers, learning model strengths, chat URLs

## Files Created This Session (Review Needed)

- `contracts/task_packet.schema.json` - Schema exists, but need TEMPLATES
- `plans/vibepilot_prd.json` - Draft, needs review
- `plans/vibepilot_plan.json` - INCOMPLETE, has Type 1 errors, needs redo

## Next Session Must Do

1. Deep Vibeflow review - understand patterns, templates, task packet structure
2. Create proper task packet templates (not just schema)
3. Understand courier fully before planning
4. Check what's already built vs what needs building
5. THEN create plan with zero ambiguity

## Swarm Test - SUCCESS

```
3 tasks in parallel → 12.53s → 3/3 success
```

This proves the backbone works. Now need to ensure the planning doesn't break it.

---

**Lesson:** Stop. Read. Understand. Confirm. Then act.
