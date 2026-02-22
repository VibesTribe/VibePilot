package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/sentry"
	"github.com/vibepilot/governor/internal/dispatcher"
	"github.com/vibepilot/governor/internal/janitor"
	"github.com/vibepilot/governor/internal/server"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/pkg/types"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg, err := config.Load("governor.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("VibePilot Governor %s starting...", version)
	log.Printf("Poll interval: %v, Max concurrent: %d", cfg.Governor.PollInterval, cfg.Governor.MaxConcurrent)

	// Initialize database connection
	database, err := db.New(cfg.Supabase.URL, cfg.Supabase.ServiceKey)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	log.Println("Connected to Supabase")

	// Initialize security components
	leakDetector := security.NewLeakDetector()
	httpAllowlist := security.NewHTTPAllowlist(cfg.Security.AllowedHosts)
	_ = httpAllowlist // Used by dispatcher for URL validation

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for task dispatch
	dispatchCh := make(chan types.Task, 10)

	// Start Sentry (poller)
	s := sentry.New(database, cfg.Governor.PollInterval, cfg.Governor.MaxConcurrent, dispatchCh)
	go s.Run(ctx)

	// Start Dispatcher
	d := dispatcher.New(database, cfg, leakDetector)
	go d.Run(ctx, dispatchCh)

	// Start Janitor
	j := janitor.New(database, cfg.Governor.StuckTimeout)
	go j.Run(ctx)

	// Start HTTP server
	srv := server.New(cfg.Server, database)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Println("Governor started. Press Ctrl+C to stop.")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down...")
	cancel()
	srv.Shutdown()
	
	log.Println("Governor stopped.")
}

func printBanner() {
	fmt.Printf(`
 _    _ _ _                          _____ _       
| |  | (_) |                        / ____| |      
| |  | |_| | _____      __ _ _ __  | |    | | _____ 
| |/\| | | |/ _ \ \ /\ / / _  '_ \ | |    | |/ / __|
\  /\  / | |  __/\ V  V /  __/ | | || |____|   <\__ \
 \/  \/|_|_|\___| \_/\_/ \___|_| |_| \_____|_|\_\___/
                                                      
Governor - The Iron Stack Orchestrator
Version: %s (%s)
Built: %s
`, version, commit, date)
}
