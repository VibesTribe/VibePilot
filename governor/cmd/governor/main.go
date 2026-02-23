package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/courier"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/dispatcher"
	"github.com/vibepilot/governor/internal/janitor"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/internal/sentry"
	"github.com/vibepilot/governor/internal/server"
	"github.com/vibepilot/governor/internal/throttle"
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
	log.Printf("Poll interval: %v, Max concurrent: %d, Max per module: %d",
		cfg.Governor.PollInterval, cfg.Governor.MaxConcurrent, cfg.Governor.MaxPerModule)

	database := db.New(cfg.Supabase.URL, cfg.Supabase.ServiceKey)
	defer database.Close()

	log.Println("Connected to Supabase")

	leakDetector := security.NewLeakDetector()
	httpAllowlist := security.NewHTTPAllowlist(cfg.Security.AllowedHosts)
	_ = httpAllowlist

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatchCh := make(chan types.Task, 10)

	moduleLimiter := throttle.NewModuleLimiter(cfg.Governor.MaxPerModule)

	s := sentry.New(database, cfg.Governor.PollInterval, cfg.Governor.MaxConcurrent, dispatchCh, moduleLimiter)
	go s.Run(ctx)

	d := dispatcher.New(database, cfg, leakDetector, moduleLimiter)

	if cfg.Courier.Enabled && cfg.GitHub.Token != "" {
		courierDispatcher := courier.NewDispatcher(
			cfg.GitHub.Token,
			cfg.GitHub.Owner,
			cfg.GitHub.Repo,
			cfg.GitHub.Workflow,
			cfg.Courier.CallbackURL,
			cfg.Courier.MaxInFlight,
		)
		go courierDispatcher.Start(ctx)
		d.SetCourier(courierDispatcher)
		log.Println("Courier enabled: GitHub Actions dispatch active")
	}

	go d.Run(ctx, dispatchCh)

	j := janitor.New(database, cfg.Governor.StuckTimeout)
	go j.Run(ctx)

	srv := server.New(&cfg.Server, database)
	srv.SetCourierCallback(d.OnCourierResult)
	srv.SetModuleCountsGetter(s.ModuleCounts)
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	log.Println("Governor started. Press Ctrl+C to stop.")

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
