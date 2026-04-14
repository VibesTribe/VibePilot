package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/vibepilot/governor/internal/runtime"
)

// ToolBinding maps a discovered tool name back to its source server.
type ToolBinding struct {
	ServerName string
	Tool       mcp.Tool
}

// Registry manages connections to approved MCP servers and provides
// a unified interface for calling any discovered tool.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]*client.Client      // server_name -> client
	tools   map[string]ToolBinding         // tool_name -> binding
	configs []runtime.MCPServerConfig
}

// NewRegistry creates a registry from the approved server configs.
func NewRegistry(configs []runtime.MCPServerConfig) *Registry {
	return &Registry{
		clients: make(map[string]*client.Client),
		tools:   make(map[string]ToolBinding),
		configs: configs,
	}
}

// Start connects to all enabled servers and discovers their tools.
// Servers that fail to connect are logged and skipped -- graceful degradation.
func (r *Registry) Start(ctx context.Context) error {
	for _, cfg := range r.configs {
		if !cfg.Enabled {
			log.Printf("[MCP] Skipping disabled server: %s", cfg.Name)
			continue
		}

		c, err := r.createClient(cfg)
		if err != nil {
			log.Printf("[MCP] Failed to create client for %s: %v", cfg.Name, err)
			continue
		}

		initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := c.Start(initCtx); err != nil {
			log.Printf("[MCP] Failed to start %s: %v", cfg.Name, err)
			continue
		}

		// Initialize the MCP session
		serverResult, err := c.Initialize(initCtx, mcp.InitializeRequest{
			Params: mcp.InitializeParams{
				ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
				ClientInfo: mcp.Implementation{
					Name:    "VibePilot Governor",
					Version: "2.0.0",
				},
			},
		})
		if err != nil {
			log.Printf("[MCP] Failed to initialize %s: %v", cfg.Name, err)
			c.Close()
			continue
		}

		r.mu.Lock()
		r.clients[cfg.Name] = c
		r.mu.Unlock()

		// Discover tools from this server
		r.discoverTools(ctx, cfg.Name, c)

		log.Printf("[MCP] Connected to %s (server: %s)", cfg.Name, serverResult.ServerInfo.Name)
	}

	r.mu.RLock()
	count := len(r.clients)
	toolCount := len(r.tools)
	r.mu.RUnlock()

	log.Printf("[MCP] Registry started: %d servers, %d tools available", count, toolCount)
	return nil
}

func (r *Registry) createClient(cfg runtime.MCPServerConfig) (*client.Client, error) {
	switch cfg.Transport {
	case "stdio":
		env := make([]string, 0, len(cfg.Env))
		for k, v := range cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		t := transport.NewStdio(cfg.Command, env, cfg.Args...)
		return client.NewClient(t), nil

	case "http", "streamable-http":
		t, err := transport.NewStreamableHTTP(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("create HTTP transport: %w", err)
		}
		return client.NewClient(t), nil

	case "sse":
		t, err := transport.NewSSE(cfg.URL)
		if err != nil {
			return nil, fmt.Errorf("create SSE transport: %w", err)
		}
		return client.NewClient(t), nil

	default:
		return nil, fmt.Errorf("unsupported transport: %s", cfg.Transport)
	}
}

func (r *Registry) discoverTools(ctx context.Context, serverName string, c *client.Client) {
	discoverCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result, err := c.ListTools(discoverCtx, mcp.ListToolsRequest{})
	if err != nil {
		log.Printf("[MCP] Failed to list tools from %s: %v", serverName, err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, tool := range result.Tools {
		r.tools[tool.Name] = ToolBinding{
			ServerName: serverName,
			Tool:       tool,
		}
		log.Printf("[MCP] Discovered tool: %s (from %s)", tool.Name, serverName)
	}
}

// CallTool invokes a tool by name. Returns the tool result as JSON.
// Only works for tools from approved servers -- no roaming.
func (r *Registry) CallTool(ctx context.Context, toolName string, args map[string]any) (json.RawMessage, error) {
	r.mu.RLock()
	binding, ok := r.tools[toolName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool %s not found in approved MCP tools", toolName)
	}

	r.mu.RLock()
	c, ok := r.clients[binding.ServerName]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server %s not connected", binding.ServerName)
	}

	callCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := c.CallTool(callCtx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("call %s on %s: %w", toolName, binding.ServerName, err)
	}

	// Extract text content from result
	for _, content := range result.Content {
		if text, ok := content.(mcp.TextContent); ok {
			return json.RawMessage(text.Text), nil
		}
	}

	// Fallback: marshal entire result
	return json.Marshal(result)
}

// ListTools returns all discovered tools from approved servers.
func (r *Registry) ListTools() []ToolBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolBinding, 0, len(r.tools))
	for _, binding := range r.tools {
		tools = append(tools, binding)
	}
	return tools
}

// ListToolInfo returns tool descriptions in the format expected by ContextBuilder.
// Implements runtime.MCPToolLister interface.
func (r *Registry) ListToolInfo() []runtime.MCPToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]runtime.MCPToolInfo, 0, len(r.tools))
	for _, binding := range r.tools {
		result = append(result, runtime.MCPToolInfo{
			Name:        binding.Tool.Name,
			Description: binding.Tool.Description,
			ServerName:  binding.ServerName,
		})
	}
	return result
}

// HasTool checks if a tool is available from an approved server.
func (r *Registry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// ToolDescription returns the description of a tool for agent context.
func (r *Registry) ToolDescription(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if binding, ok := r.tools[name]; ok {
		return binding.Tool.Description
	}
	return ""
}

// Shutdown gracefully closes all MCP client connections.
func (r *Registry) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, c := range r.clients {
		if err := c.Close(); err != nil {
			log.Printf("[MCP] Error closing %s: %v", name, err)
		}
		delete(r.clients, name)
	}
	r.tools = make(map[string]ToolBinding)
	log.Println("[MCP] Registry shut down")
}
