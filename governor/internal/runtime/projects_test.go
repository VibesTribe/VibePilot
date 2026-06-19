package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/vibepilot/governor/internal/db"
)

// TestProjectResolver_LoadVibePilot verifies that the resolver can load the
// "vibepilot" project that was seeded by migration 047.
// This test requires a live database (DATABASE_URL env var).
func TestProjectResolver_LoadVibePilot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Connect to the database using the same mechanism as main.go
	pgURL := getTestDBURL(t)
	database, err := db.NewPostgres(ctx, pgURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer database.Close()

	resolver := NewProjectResolver(database)

	// Load by slug
	project, err := resolver.GetProjectBySlug(ctx, "vibepilot")
	if err != nil {
		t.Fatalf("GetProjectBySlug failed: %v", err)
	}

	// Verify identity fields
	if project.Slug != "vibepilot" {
		t.Errorf("expected slug 'vibepilot', got %q", project.Slug)
	}
	if project.DisplayName != "VibePilot" {
		t.Errorf("expected display_name 'VibePilot', got %q", project.DisplayName)
	}

	// Verify git config
	if project.GitHubOwner != "VibesTribe" {
		t.Errorf("expected github_owner 'VibesTribe', got %q", project.GitHubOwner)
	}
	if project.GitHubRepo != "VibePilot" {
		t.Errorf("expected github_repo 'VibePilot', got %q", project.GitHubRepo)
	}
	if project.RepoPath != "/home/vibes/vibepilot" {
		t.Errorf("expected repo_path '/home/vibes/vibepilot', got %q", project.RepoPath)
	}

	// Verify model keys
	if len(project.ModelKeys) == 0 {
		t.Error("expected model_keys to be populated, got empty slice")
	}

	// Verify connected services
	if len(project.ConnectedServices) == 0 {
		t.Error("expected connected_services to be populated, got empty slice")
	}

	// Verify theme
	if project.Theme.PrimaryColor == "" {
		t.Error("expected theme.primary_color to be set")
	}

	// Verify protected branches
	if len(project.ProtectedBranches) == 0 {
		t.Error("expected protected_branches to contain 'main'")
	} else if project.ProtectedBranches[0] != "main" {
		t.Errorf("expected first protected branch 'main', got %q", project.ProtectedBranches[0])
	}

	// Now load by ID and verify it returns the same project
	project2, err := resolver.GetProjectByID(ctx, project.ID)
	if err != nil {
		t.Fatalf("GetProjectByID failed: %v", err)
	}
	if project2.Slug != project.Slug {
		t.Errorf("ID lookup returned different slug: %q vs %q", project2.Slug, project.Slug)
	}
}

// TestProjectResolver_NotFound verifies error handling for missing projects.
func TestProjectResolver_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	pgURL := getTestDBURL(t)
	database, err := db.NewPostgres(ctx, pgURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer database.Close()

	resolver := NewProjectResolver(database)

	_, err = resolver.GetProjectBySlug(ctx, "nonexistent-project-12345")
	if err == nil {
		t.Error("expected error for nonexistent project, got nil")
	}
}

// TestProjectResolver_Cache verifies that repeated calls use the cache.
func TestProjectResolver_Cache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	pgURL := getTestDBURL(t)
	database, err := db.NewPostgres(ctx, pgURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer database.Close()

	resolver := NewProjectResolver(database)

	// First call hits the DB
	p1, err := resolver.GetProjectBySlug(ctx, "vibepilot")
	if err != nil {
		t.Fatalf("first GetProjectBySlug failed: %v", err)
	}

	// Second call should hit cache (same pointer)
	p2, err := resolver.GetProjectBySlug(ctx, "vibepilot")
	if err != nil {
		t.Fatalf("second GetProjectBySlug failed: %v", err)
	}

	if p1 != p2 {
		t.Error("expected cached call to return same pointer")
	}

	// After invalidation, should return a fresh copy from DB
	resolver.InvalidateCache()
	p3, err := resolver.GetProjectBySlug(ctx, "vibepilot")
	if err != nil {
		t.Fatalf("post-invalidate GetProjectBySlug failed: %v", err)
	}

	// p3 should have the same slug but could be a different pointer
	if p3.Slug != p1.Slug {
		t.Error("post-invalidate project has different slug")
	}
}

// getTestDBURL reads the database URL from the environment, falling back to
// the default local connection string.
func getTestDBURL(t *testing.T) string {
	t.Helper()
	// Read from the same place main.go does
	url := getEnvOrDefault("DATABASE_URL", "postgres://vibes@/vibepilot?host=/var/run/postgresql")
	return url
}

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
