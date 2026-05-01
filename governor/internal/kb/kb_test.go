package kb

import (
	"context"
	"testing"
)

func TestKBConnection(t *testing.T) {
	ctx := context.Background()
	kb, err := New(ctx, "dbname=vibepilot")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer kb.Close()

	stats, err := kb.Stats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if len(stats) == 0 {
		t.Fatal("Expected stats entries, got none")
	}

	for _, s := range stats {
		t.Logf("  %s: %d rows", s.TableName, s.RowCount)
		if s.RowCount < 0 {
			t.Errorf("Negative row count for %s", s.TableName)
		}
	}
}

func TestSearchSymbols(t *testing.T) {
	ctx := context.Background()
	kb, err := New(ctx, "dbname=vibepilot")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer kb.Close()

	symbols, err := kb.SearchSymbols(ctx, "handleTask", nil, nil, 5)
	if err != nil {
		t.Fatalf("SearchSymbols failed: %v", err)
	}

	if len(symbols) == 0 {
		t.Fatal("Expected symbol results, got none")
	}

	for _, s := range symbols {
		t.Logf("  %s (%s) @ %s:%d-%d", s.QualifiedName, s.Kind, s.FileID, s.LineStart, s.LineEnd)
		if s.Name == "" {
			t.Error("Empty symbol name")
		}
	}
}

func TestSearchDocs(t *testing.T) {
	ctx := context.Background()
	kb, err := New(ctx, "dbname=vibepilot")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer kb.Close()

	docs, err := kb.SearchDocs(ctx, "pipeline", nil, 5)
	if err != nil {
		t.Fatalf("SearchDocs failed: %v", err)
	}

	if len(docs) == 0 {
		t.Fatal("Expected doc results, got none")
	}

	for _, d := range docs {
		t.Logf("  [%s] %s (%s)", d.RepoID, d.Title, d.DocPath)
	}
}

func TestGetCallers(t *testing.T) {
	ctx := context.Background()
	kb, err := New(ctx, "dbname=vibepilot")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer kb.Close()

	// Use a qualified name that exists in the edges
	callers, err := kb.GetCallers(ctx, "handleTaskEvent", 10)
	if err != nil {
		t.Fatalf("GetCallers failed: %v", err)
	}

	t.Logf("Found %d callers for handleTaskEvent", len(callers))
	for _, c := range callers {
		t.Logf("  %s (%s)", c.SourceQualified, c.Kind)
	}
}

func TestSearchKnowledge(t *testing.T) {
	ctx := context.Background()
	kb, err := New(ctx, "dbname=vibepilot")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer kb.Close()

	items, err := kb.SearchKnowledge(ctx, "config", nil, 5)
	if err != nil {
		t.Fatalf("SearchKnowledge failed: %v", err)
	}

	if len(items) == 0 {
		t.Fatal("Expected knowledge results, got none")
	}

	for _, ki := range items {
		t.Logf("  [%s] %s", ki.ItemType, ki.Name)
	}
}

func TestSearchSkills(t *testing.T) {
	ctx := context.Background()
	kb, err := New(ctx, "dbname=vibepilot")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer kb.Close()

	skills, err := kb.SearchSkills(ctx, "mcp", nil, 5)
	if err != nil {
		t.Fatalf("SearchSkills failed: %v", err)
	}

	t.Logf("Found %d skills matching 'mcp'", len(skills))
	for _, s := range skills {
		t.Logf("  [%v] %s", s.Category, s.Name)
	}
}

func TestResolveFileID(t *testing.T) {
	fileID := ResolveFileID("vibepilot", "governor/internal/core/events.go")
	if fileID != "vibepilot:governor/internal/core/events.go" {
		t.Errorf("Expected 'vibepilot:governor/internal/core/events.go', got '%s'", fileID)
	}

	repo, path := ParseFileID(fileID)
	if repo != "vibepilot" {
		t.Errorf("Expected repo 'vibepilot', got '%s'", repo)
	}
	if path != "governor/internal/core/events.go" {
		t.Errorf("Expected path 'governor/internal/core/events.go', got '%s'", path)
	}
}
