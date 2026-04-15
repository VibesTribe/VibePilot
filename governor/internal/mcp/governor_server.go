// Package mcp provides MCP (Model Context Protocol) connectivity for VibePilot.
// This file implements the MCP SERVER that exposes the governor's internal
// tool registry to external agents (Claude Code, Codex, OpenCode, etc.).
//
// Any agent that speaks MCP can connect and use governor tools directly:
// git operations, database queries, vault secrets, file ops, web search, etc.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/vibepilot/governor/internal/runtime"
)

// GovernorServer wraps an MCP server that exposes the governor's tool registry.
type GovernorServer struct {
	server   *mcpserver.MCPServer
	registry *runtime.ToolRegistry
	config   *runtime.Config
	stdio    *mcpserver.StdioServer
	sse      *mcpserver.SSEServer
	cfg      runtime.GovernorMCPConfig
}

// NewGovernorServer creates an MCP server exposing all registered governor tools.
func NewGovernorServer(registry *runtime.ToolRegistry, config *runtime.Config, cfg runtime.GovernorMCPConfig) *GovernorServer {
	return &GovernorServer{
		registry: registry,
		config:   config,
		cfg:      cfg,
	}
}

// Start initializes and starts the MCP server based on the configured transport.
func (gs *GovernorServer) Start(ctx context.Context) error {
	if !gs.cfg.Enabled {
		log.Println("[MCP-Server] Disabled in config, not starting")
		return nil
	}

	gs.server = mcpserver.NewMCPServer(
		"vibepilot-governor",
		"2.0.0",
		mcpserver.WithToolCapabilities(true),
	)

	if err := gs.registerTools(); err != nil {
		return fmt.Errorf("register tools: %w", err)
	}

	log.Printf("[MCP-Server] Registered %d governor tools", len(gs.registry.ListTools()))

	switch gs.cfg.Transport {
	case "stdio":
		return gs.startStdio(ctx)
	case "sse":
		return gs.startSSE(ctx)
	default:
		return fmt.Errorf("unsupported MCP server transport: %s (use 'stdio' or 'sse')", gs.cfg.Transport)
	}
}

// registerTools iterates over the tool registry and registers each tool with the MCP server.
func (gs *GovernorServer) registerTools() error {
	for _, toolName := range gs.registry.ListTools() {
		toolCfg := gs.config.GetTool(toolName)
		if toolCfg == nil {
			log.Printf("[MCP-Server] Warning: tool %s has no config definition, skipping", toolName)
			continue
		}

		mcpTool := gs.buildMCPTool(toolName, toolCfg)
		handler := gs.makeHandler(toolName)

		gs.server.AddTool(mcpTool, handler)
	}
	return nil
}

// buildMCPTool converts a governor ToolConfig into an mcp-go Tool definition.
func (gs *GovernorServer) buildMCPTool(name string, cfg *runtime.ToolConfig) mcplib.Tool {
	opts := []mcplib.ToolOption{
		mcplib.WithDescription(cfg.Description),
	}

	// Convert each parameter to MCP schema
	for paramName, paramCfg := range cfg.Parameters {
		opts = append(opts, gs.paramToOption(paramName, paramCfg))
	}

	return mcplib.NewTool(name, opts...)
}

// paramToOption converts a governor ToolParam to the appropriate MCP ToolOption.
func (gs *GovernorServer) paramToOption(name string, param runtime.ToolParam) mcplib.ToolOption {
	propOpts := []mcplib.PropertyOption{}
	if param.Required {
		propOpts = append(propOpts, mcplib.Required())
	}

	switch param.Type {
	case "string":
		return mcplib.WithString(name, propOpts...)
	case "integer", "int":
		return mcplib.WithNumber(name, propOpts...)
	case "number", "float":
		return mcplib.WithNumber(name, propOpts...)
	case "boolean", "bool":
		return mcplib.WithBoolean(name, propOpts...)
	case "array":
		return mcplib.WithArray(name, propOpts...)
	case "object":
		return mcplib.WithObject(name, propOpts...)
	default:
		// Unknown type -- treat as string
		return mcplib.WithString(name, propOpts...)
	}
}

// makeHandler creates an MCP ToolHandlerFunc that bridges to the governor's tool registry.
func (gs *GovernorServer) makeHandler(toolName string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args := request.GetArguments()
		if args == nil {
			args = make(map[string]any)
		}

		log.Printf("[MCP-Server] Tool called: %s", toolName)

		result := gs.registry.Execute(ctx, toolName, args)

		var textContent string
		if result.Success {
			if result.Result != nil {
				textContent = string(result.Result)
			} else {
				textContent = `{"success": true}`
			}
		} else {
			textContent = fmt.Sprintf(`{"success": false, "error": %s}`, jsonEscape(result.Error))
		}

		return &mcplib.CallToolResult{
			Content: []mcplib.Content{
				mcplib.TextContent{
					Type: "text",
					Text: textContent,
				},
			},
			IsError: !result.Success,
		}, nil
	}
}

// startStdio starts the MCP server in stdio mode (for CLI agent pipes).
func (gs *GovernorServer) startStdio(ctx context.Context) error {
	gs.stdio = mcpserver.NewStdioServer(gs.server)
	log.Println("[MCP-Server] Starting in stdio mode (listening on stdin/stdout)")
	go func() {
		if err := gs.stdio.Listen(ctx, os.Stdin, os.Stdout); err != nil {
			log.Printf("[MCP-Server] Stdio server error: %v", err)
		}
	}()
	return nil
}

// startSSE starts the MCP server in SSE mode (for HTTP connections).
func (gs *GovernorServer) startSSE(ctx context.Context) error {
	port := gs.cfg.Port
	if port == 0 {
		port = 8081 // default MCP server port, offset from webhook port 8080
	}
	addr := ":" + strconv.Itoa(port)
	gs.sse = mcpserver.NewSSEServer(gs.server)

	log.Printf("[MCP-Server] Starting in SSE mode on %s", addr)
	go func() {
		if err := gs.sse.Start(addr); err != nil {
			log.Printf("[MCP-Server] SSE server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		log.Println("[MCP-Server] Shutting down SSE server")
		gs.sse.Shutdown(ctx)
	}()

	return nil
}

// Shutdown gracefully stops the MCP server.
func (gs *GovernorServer) Shutdown() {
	if gs.sse != nil {
		gs.sse.Shutdown(context.Background())
	}
	log.Println("[MCP-Server] Stopped")
}

// jsonEscape wraps a string in JSON quotes, escaping special characters.
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
