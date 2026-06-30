package runtime

import (
	"testing"
)

func TestLoadProjectContext_Sealed(t *testing.T) {
	ctx := LoadProjectContext("sealed")
	if ctx == nil {
		t.Fatal("Expected non-nil ProjectContext for sealed")
	}
	if ctx.Slug != "sealed" {
		t.Errorf("Expected slug 'sealed', got '%s'", ctx.Slug)
	}
	if ctx.Manifest == nil {
		t.Fatal("Expected non-nil Manifest")
	}
	if ctx.Manifest.DisplayName != "Sealed" {
		t.Errorf("Expected DisplayName 'Sealed', got '%s'", ctx.Manifest.DisplayName)
	}
	if ctx.Manifest.AgentRuntime != "hermes" {
		t.Errorf("Expected AgentRuntime 'hermes', got '%s'", ctx.Manifest.AgentRuntime)
	}
	if ctx.Manifest.DeployTarget != "cloudflare" {
		t.Errorf("Expected DeployTarget 'cloudflare', got '%s'", ctx.Manifest.DeployTarget)
	}
	if ctx.Manifest.DatabaseType != "sqlite" {
		t.Errorf("Expected DatabaseType 'sqlite', got '%s'", ctx.Manifest.DatabaseType)
	}
	if ctx.HermesMD == "" {
		t.Error("Expected non-empty HermesMD")
	}
	if ctx.FileTree == "" {
		t.Error("Expected non-empty FileTree")
	}

	// Test prompt assembly
	prompt := ctx.AssemblePrompt()
	if prompt == "" {
		t.Error("Expected non-empty assembled prompt")
	}
}

func TestLoadProjectContext_Vibepilot(t *testing.T) {
	ctx := LoadProjectContext("vibepilot")
	if ctx != nil {
		t.Error("Expected nil for vibepilot (uses default context builder)")
	}
}

func TestLoadProjectContext_NonExistent(t *testing.T) {
	ctx := LoadProjectContext("does-not-exist-12345")
	if ctx != nil {
		t.Error("Expected nil for non-existent project")
	}
}
