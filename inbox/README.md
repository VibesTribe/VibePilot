# Agent Inbox System

Cross-session communication for multi-agent coordination.

## How It Works

Each agent has an inbox directory:
- `inbox/glm-5/` - Messages for GLM-5
- `inbox/kimi/` - Messages for Kimi

## File Format

```
filename: {priority}-{topic}.md
```

Priority prefixes:
- `high-` - Urgent, blocks work
- `med-` - Important but not blocking
- `low-` - Nice to have

## Message Structure

```markdown
---
from: glm-5
to: kimi
type: task | result | question | notice
created: 2026-02-18T21:30:00Z
---

## Subject
Brief subject line

## Body
Detailed message content...

## Expected Response
What you need back (if task or question)
```

## Workflow

### Sending a Message
1. Create file in recipient's inbox
2. Commit and push to main
3. Recipient sees it on next pull

### Checking Inbox
```bash
ls ~/vibepilot/inbox/{your-name}/
cat ~/vibepilot/inbox/{your-name}/{filename}.md
```

### Responding
1. Read the message
2. Do the work / research
3. Create response file in sender's inbox
4. Mark original as done: rename to `done-{filename}.md`

## Example

```bash
# GLM-5 sends task to Kimi
cat > inbox/kimi/high-dependency-rpc.md << 'EOF'
---
from: glm-5
to: kimi
type: task
---

## Research: Dependency RPC Schema Mismatch

The RPC functions (unlock_dependent_tasks, check_dependencies_complete) 
expect dependencies column to be jsonb, but it's currently uuid[].

Research:
1. What's the impact of changing to jsonb?
2. Any existing data that would break?
3. Recommended approach?

## Expected Response
A recommendation with code samples if possible.
EOF

# Kimi responds
cat > inbox/glm-5/result-dependency-rpc.md << 'EOF'
---
from: kimi
to: glm-5
type: result
---

## Research Result: Dependency Schema

After analyzing [files], here's my recommendation...

[Findings and code]
EOF
```

## Guidelines

1. Keep messages focused - one topic per file
2. Be specific about what you need
3. Include context - don't assume shared memory
4. Close the loop - respond to tasks, don't leave hanging
