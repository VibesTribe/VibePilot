// cleanup is a maintenance tool for cleaning up stale pipeline data.
// DEPRECATED: References Supabase env vars. Needs rewriting for local Postgres.
// Use psql TRUNCATE ... CASCADE directly instead.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vibepilot/governor/internal/db"
)

func main() {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_SERVICE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		log.Fatal("SUPABASE_URL and SUPABASE_SERVICE_KEY required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	database := db.New(supabaseURL, supabaseKey)
	defer database.Close()

	// Delete all tasks
	fmt.Println("Deleting all tasks...")
	_, err := database.REST(ctx, "DELETE", "tasks?id=not.is.null", nil)
	if err != nil {
		log.Printf("Warning: failed to delete tasks: %v", err)
	} else {
		fmt.Println("✓ Tasks deleted")
	}

	// Delete all task_runs
	fmt.Println("Deleting all task_runs...")
	_, err = database.REST(ctx, "DELETE", "task_runs?id=not.is.null", nil)
	if err != nil {
		log.Printf("Warning: failed to delete task_runs: %v", err)
	} else {
		fmt.Println("✓ Task runs deleted")
	}

	// Delete all plans
	fmt.Println("Deleting all plans...")
	_, err = database.REST(ctx, "DELETE", "plans?id=not.is.null", nil)
	if err != nil {
		log.Printf("Warning: failed to delete plans: %v", err)
	} else {
		fmt.Println("✓ Plans deleted")
	}

	fmt.Println("\n✓ Cleanup complete!")
}
