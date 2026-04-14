# VibePilot MCP Integration Design

## SDK: mark3labs/mcp-go

The only mature Go MCP SDK. Supports both client and server modes with stdio, SSE, and streamable HTTP transports.

- Repo: https://github.com/mark3labs/mcp-go
- License: MIT
- Go version: 1.23+ (VibePilot is on 1.24, compatible)
- Active maintenance, good test coverage, clean high-level API

## Two Phases

### Phase 1: VibePilot AS MCP CLIENT (build now)

VibePilot connects to external MCP servers to give agents access to specialized tools.

**Use cases:**
- jCodeMunch for code analysis during build/review
- Any future MCP server (database tools, API testers, etc.)
- Config-driven -- add a server in config, agents can use its tools

**Architecture:**

```
config/system.json
  └─ mcp_servers[]
       ├─ name: "jcodemunch"
       ├─ transport: "stdio" | "http" | "sse"
       ├─ command: "path/to/server" (stdio)
       ├─ url: "http://..." (http/sse)
       └─ env: { "KEY": "value" }
```

**Integration points:**

1. **Startup** (cmd/governor/main.go):
   - Load MCP server configs from system.json
   - Create `mcp.Client` per configured server
   - Call `client.Start()` + `client.Initialize()`
   - Discover available tools via `client.ListTools()`
   - Register discovered tools in the tool registry

2. **Agent sessions** (internal/runtime/session.go):
   - When building agent context, include available MCP tools
   - Agent can call MCP tools through the session's tool handler
   - MCP tool calls go: agent -> session -> mcp.Client -> external server

3. **Tool registry** (new: internal/mcp/registry.go):
   - Wraps mcp-go clients
   - Maps tool names to their MCP server client
   - Provides unified `CallTool(ctx, name, args)` interface
   - Handles timeouts, errors, retries

**Key interfaces:**

```go
// internal/mcp/registry.go
type Registry struct {
    clients map[string]*client.Client  // server_name -> client
    tools   map[string]ToolBinding     // tool_name -> server+client
}

type ToolBinding struct {
    ServerName string
    Client     *client.Client
    ToolDef    mcp.Tool
}

func (r *Registry) CallTool(ctx context.Context, toolName string, args map[string]any) (string, error)
func (r *Registry) ListTools() []ToolBinding
func (r *Registry) Shutdown()
```

**Config format** (added to system.json):

```json
{
  "mcp_servers": [
    {
      "name": "jcodemunch",
      "transport": "stdio",
      "command": "/usr/local/bin/jcodemunch",
      "args": ["--mode", "analyze"],
      "env": {},
      "enabled": true
    },
    {
      "name": "example-http",
      "transport": "http",
      "url": "http://localhost:8080/mcp",
      "enabled": false
    }
  ]
}
```

**Dependencies added to go.mod:**
```
github.com/mark3labs/mcp-go v0.x
```

### Phase 2: VibePilot AS MCP SERVER (build later)

Expose VibePilot's tools as an MCP server so any agent (Hermes, Claude, Codex, etc.) can connect.

**Tools to expose:**
- `task_create` - create a new task
- `task_list` - list tasks by status
- `task_transition` - move task through pipeline
- `git_create_branch` - create task branch
- `git_merge` - merge branch to target
- `council_review` - trigger council review
- `planner_rules_list` - get learned rules
- `planner_rule_create` - add a learned rule
- `project_status` - get overall project status

**Transport:** Streamable HTTP (most compatible with remote agents)

**This phase starts after VibePilot is fully functional and optimized.**

## Implementation Order (Phase 1)

1. Add `mcp-go` dependency
2. Create `internal/mcp/registry.go` with client management
3. Add config loading for `mcp_servers` in main.go
4. Wire tool discovery into session context builder
5. Add MCP tool execution path in session.go
6. Test with a simple MCP server (echo or similar)

## Principles Alignment

- **Config-driven** -- servers defined in system.json, no hardcoded providers
- **Modular** -- each MCP server is independent, can be enabled/disabled
- **Agnostic** -- works with any MCP-compliant server
- **Graceful degradation** -- if an MCP server is down, agents continue without it
- **Recoverable** -- MCP config lives in system.json which is in git
