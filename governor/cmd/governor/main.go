package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vibepilot/governor/internal/config"
	"github.com/vibepilot/governor/internal/courier"
	"github.com/vibepilot/governor/internal/db"
	"github.com/vibepilot/governor/internal/dispatcher"
	"github.com/vibepilot/governor/internal/janitor"
	"github.com/vibepilot/governor/internal/maintenance"
	"github.com/vibepilot/governor/internal/orchestrator"
	"github.com/vibepilot/governor/internal/researcher"
	"github.com/vibepilot/governor/internal/security"
	"github.com/vibepilot/governor/internal/sentry"
	"github.com/vibepilot/governor/internal/server"
	"github.com/vibepilot/governor/internal/supervisor"
	"github.com/vibepilot/governor/internal/tester"
	"github.com/vibepilot/governor/internal/throttle"
	"github.com/vibepilot/governor/internal/visual"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatchCh := make(chan types.Task, 10)

	moduleLimiter := throttle.NewModuleLimiter(cfg.Governor.MaxPerModule)

	repoPath := cfg.Governor.RepoPath
	if repoPath == "" {
		repoPath = "."
	}

	maint := maintenance.New(&maintenance.Config{RepoPath: repoPath})
	sup := supervisor.New()
	test := tester.New(&tester.Config{RepoPath: repoPath})
	visTest := visual.New(&visual.Config{RepoPath: repoPath})
	res := researcher.New(database)
	orch := orchestrator.New(database, maint, sup, test, visTest, res)

	s := sentry.New(database, cfg.Governor.PollInterval, cfg.Governor.MaxConcurrent, dispatchCh, moduleLimiter)
	go s.Run(ctx)

	d := dispatcher.New(database, cfg, leakDetector)
	d.SetOrchestrator(orch)
	d.SetMaintenance(maint)
	d.SetFinalizer(s)

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
	go orch.Run(ctx)

	j := janitor.New(database, cfg.Governor.StuckTimeout, cfg.Deprecation)
	go j.Run(ctx)

	srv := server.New(&cfg.Server, &cfg.Governor, database)
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
