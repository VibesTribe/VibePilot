package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/vibepilot/governor/internal/analyst"
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

	sup.SetRuleProvider(&supervisorRuleAdapter{db: database})
	test.SetRuleProvider(&testerRuleAdapter{db: database})

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

	analystSvc := analyst.New(database, cfg.Analyst, cfg.GitHub, repoPath)
	go analystSvc.Run(ctx)

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

type supervisorRuleAdapter struct {
	db *db.DB
}

func (a *supervisorRuleAdapter) GetSupervisorRules(ctx context.Context, taskType string, limit int) ([]supervisor.SupervisorRule, error) {
	rules, err := a.db.GetSupervisorRules(ctx, taskType, limit)
	if err != nil {
		return nil, err
	}
	result := make([]supervisor.SupervisorRule, len(rules))
	for i, r := range rules {
		result[i] = supervisor.SupervisorRule{
			ID:               r.ID,
			TriggerPattern:   r.TriggerPattern,
			TriggerCondition: r.TriggerCondition,
			Action:           r.Action,
			Reason:           r.Reason,
			TimesCaughtIssue: r.TimesCaughtIssue,
		}
	}
	return result, nil
}

func (a *supervisorRuleAdapter) RecordSupervisorRuleTriggered(ctx context.Context, ruleID string, caughtIssue bool) error {
	return a.db.RecordSupervisorRuleTriggered(ctx, ruleID, caughtIssue)
}

type testerRuleAdapter struct {
	db *db.DB
}

func (a *testerRuleAdapter) GetTesterRules(ctx context.Context, appliesTo string, limit int) ([]tester.TesterRule, error) {
	rules, err := a.db.GetTesterRules(ctx, appliesTo, limit)
	if err != nil {
		return nil, err
	}
	result := make([]tester.TesterRule, len(rules))
	for i, r := range rules {
		result[i] = tester.TesterRule{
			ID:             r.ID,
			AppliesTo:      r.AppliesTo,
			TestType:       r.TestType,
			TestCommand:    r.TestCommand,
			TriggerPattern: r.TriggerPattern,
			Priority:       r.Priority,
			CaughtBugs:     r.CaughtBugs,
			FalsePositives: r.FalsePositives,
		}
	}
	return result, nil
}

func (a *testerRuleAdapter) RecordTesterRuleCaughtBug(ctx context.Context, ruleID string) error {
	return a.db.RecordTesterRuleCaughtBug(ctx, ruleID)
}

func (a *testerRuleAdapter) RecordTesterRuleFalsePositive(ctx context.Context, ruleID string) error {
	return a.db.RecordTesterRuleFalsePositive(ctx, ruleID)
}
