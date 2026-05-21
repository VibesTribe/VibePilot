# VibePilot Code Map
# Generated: 2026-05-21T10:54:56Z | Commit: 3a51ef44
# Auto-generated. Run build.sh to regenerate.

## governor/cmd/cleanup/main.go
main.go [68L]
  deps: syscall, context, log, github.com/vibepilot/governor/internal/db, os/signal, fmt, time, os
  API:
    fn main()

## governor/cmd/encrypt_secret/main.go
main.go [26L]
  deps: github.com/vibepilot/governor/internal/vault, fmt, os
  API:
    fn main()

## governor/cmd/governor/adapters.go
adapters.go [36L]
  deps: encoding/json, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/core, context
  API:
    cl dbCheckpointAdapter
      fn ⊛ RPC(ctx context.Context, fn string, args map[string]any) → (json.RawMessage, error)
      fn ⊛ Save(ctx context.Context, taskID string, checkpoint core.Checkpoint) → error
      fn ⊛ Load(ctx context.Context, taskID string) → (*core.Checkpoint, error)
      fn ⊛ Delete(ctx context.Context, taskID string) → error

## governor/cmd/governor/content_fetcher.go
content_fetcher.go [48L]
  deps: os, fmt, log, net/http, path/filepath, io, context
  API:
    fn fetchContent(ctx context.Context, repoPath, filePath string) → ([]byte, error)

## governor/cmd/governor/handlers_council.go
handlers_council.go [522L]
  deps: github.com/vibepilot/governor/internal/gitree, context, sync, os, encoding/json, github.com/vibepilot/governor/internal/runtime, fmt, errors, github.com/vibepilot/governor/internal/db, log, time
  exports: NewCouncilHandler
  API:
    cl ⊛ CouncilHandler
    fn ⊛ NewCouncilHandler(database db.Database, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, cfg *runtime.Config, git *gitree.Gitree, usageTracker *runtime.UsageTracker, ) → *CouncilHandler
      fn ⊛ Register(router *runtime.EventRouter)
    fn mapStrAny(m map[string]interface{}) → map[string]any
    fn setupCouncilHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, )

## governor/cmd/governor/handlers_maint.go
handlers_maint.go [664L]
  deps: log, time, context, encoding/json, fmt, github.com/vibepilot/governor/internal/gitree, strings, os/exec, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/runtime
  exports: NewMaintenanceHandler
  API:
    cl ⊛ MaintenanceHandler
    fn ⊛ NewMaintenanceHandler(database db.Database, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, cfg *runtime.Config, git *gitree.Gitree, worktreeMgr *gitree.WorktreeManager, usageTracker *runtime.UsageTracker, ) → *MaintenanceHandler
      fn ⊛ Register(router *runtime.EventRouter)
    fn execCmd(cmd *exec.Cmd, dir string, out *strings.Builder)
    fn setupMaintenanceHandler(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, worktreeMgr *gitree.WorktreeManager, usageTracker *runtime.UsageTracker, )

## governor/cmd/governor/handlers_plan.go
handlers_plan.go [745L]
  deps: encoding/json, github.com/vibepilot/governor/internal/db, os, context, strings, log, path/filepath, fmt, time, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/runtime
  API:
    fn setupPlanHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, )
    fn handlePlanCreated(ctx context.Context, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, event runtime.Event, )
    fn runPlanReview(ctx context.Context, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, plan map[string]any, )
    fn handlePlanReview(ctx context.Context, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, event runtime.Event, )
    fn setPlanError(ctx context.Context, database db.Database, planID string, reason string)

## governor/cmd/governor/handlers_research.go
handlers_research.go [1193L]
  deps: github.com/vibepilot/governor/internal/runtime, fmt, strings, time, github.com/vibepilot/governor/internal/db, os, context, os/exec, encoding/json, log
  exports: NewResearchHandler
  API:
    cl ⊛ ResearchHandler
    cl councilMemberResult
    cl reportItemResult
    fn ⊛ NewResearchHandler(database db.Database, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, cfg *runtime.Config, usageTracker *runtime.UsageTracker, actionApplier *runtime.ResearchActionApplier, ) → *ResearchHandler
      fn ⊛ Register(router *runtime.EventRouter)
    cl itemAgg
    fn slugFromTitle(title string) → string
    fn shellQuote(s string) → string
    fn execCommand(name string, args ...string) → ([]byte, error)
    fn extractComparisonField(details map[string]any, field string) → string
    fn setupResearchHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, usageTracker *runtime.UsageTracker, actionApplier *runtime.ResearchActionApplier, )
    fn suggestionTitle(ctx context.Context, database db.Database, id string) → string

## governor/cmd/governor/handlers_task.go
handlers_task.go [1966L]
  deps: path/filepath, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/security, log, strings, fmt, github.com/vibepilot/governor/internal/core, github.com/vibepilot/governor/internal/runtime, context, github.com/vibepilot/governor/internal/connectors, time, encoding/json, os, github.com/vibepilot/governor/internal/db
  exports: NewTaskHandler
  API:
    cl ⊛ TaskHandler
    if vaultProvider
    fn ⊛ NewTaskHandler(database db.Database, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, git *gitree.Gitree, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, cfg *runtime.Config, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v vaultProvider, ) → *TaskHandler
      fn ⊛ SetContextBuilder(cb *runtime.ContextBuilder)
      fn ⊛ Register(router *runtime.EventRouter)
    fn recordEvent(ctx context.Context, database db.Database, eventType, taskID, modelID, reason string, details map[string]any)
    cl costResult
    fn isRateLimitError(err error) → bool
    fn setupTaskHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v vaultProvider, contextBuilder *runtime.ContextBuilder, )
    fn unlockDependentsByTaskNumber(ctx context.Context, database db.Database, completedTaskNumber string)

## governor/cmd/governor/handlers_testing.go
handlers_testing.go [1495L]
  deps: time, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/runtime, bytes, os/exec, log, fmt, strings, os, context, encoding/json, path/filepath
  exports: NewTestingHandler
  API:
    cl ⊛ TestingHandler
    fn ⊛ NewTestingHandler(database db.Database, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, git *gitree.Gitree, cfg *runtime.Config, worktreeMgr *gitree.WorktreeManager, usageTracker *runtime.UsageTracker, ) → *TestingHandler
      fn ⊛ Register(router *runtime.EventRouter)
    ty projectType
    fn detectProjectType(dir string) → projectType
    fn setupTestingHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, worktreeMgr *gitree.WorktreeManager, usageTracker *runtime.UsageTracker, )
    fn toStringSlice(items []any) → []string

## governor/cmd/governor/helpers.go
helpers.go [227L]
  deps: strings, context, encoding/json, log, github.com/vibepilot/governor/internal/db
  API:
    fn getString(m map[string]any, key string) → string
    fn getStringOr(m map[string]any, key, def string) → string
    fn parseBool(data []byte) → bool
    fn truncateID(id string) → string
    fn truncateOutput(output string) → string
    fn extractCouncilReviews(plan map[string]any) → []map[string]any
    fn accumulateFailedModel(ctx context.Context, database db.Database, taskID string, prefix string, modelID string) → []string
    fn parseFailedModels(flagReason string) → []string
    fn isPromptSuspect(flagReason string) → bool
    fn recordModelSuccess(ctx context.Context, database db.Database, modelID, taskType string, durationSeconds float64)
    fn recordModelFailure(ctx context.Context, database db.Database, modelID, taskID, failureType string)
    fn unwrapRPCResult(data []byte, funcName string) → []byte

## governor/cmd/governor/helpers_record.go
helpers_record.go [48L]
  deps: github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/runtime, context, fmt, encoding/json
  API:
    fn fetchRecord(ctx context.Context, database db.Database, event runtime.Event) → (map[string]any, error)

## governor/cmd/governor/kb_adapter.go
kb_adapter.go [106L]
  deps: context, github.com/vibepilot/governor/internal/kb, github.com/vibepilot/governor/internal/runtime
  API:
    cl kbAdapter
    fn convertSymbol(s kb.Symbol) → runtime.KBSymbol
      fn ⊛ SearchSymbols(ctx context.Context, query string, filterKind, filterRepo *string, limit int) → ([]runtime.KBSymbol, error)
      fn ⊛ GetFileSymbols(ctx context.Context, fileID string, limit int) → ([]runtime.KBSymbol, error)
      fn ⊛ SearchDocs(ctx context.Context, query string, filterRepo *string, limit int) → ([]runtime.KBDocSection, error)
      fn ⊛ SearchKnowledge(ctx context.Context, query string, filterType *string, limit int) → ([]runtime.KBKnowledgeItem, error)
      fn ⊛ Stats(ctx context.Context) → ([]runtime.KBStatsEntry, error)

## governor/cmd/governor/main.go
main.go [907L]
  deps: os, github.com/vibepilot/governor/internal/connectors, fmt, syscall, github.com/vibepilot/governor/internal/runtime, time, github.com/vibepilot/governor/internal/tools, github.com/vibepilot/governor/internal/core, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/gitree, path/filepath, github.com/vibepilot/governor/internal/mcp, github.com/vibepilot/governor/internal/security, github.com/vibepilot/governor/internal/memory, context, log, os/signal, github.com/vibepilot/governor/internal/dag, github.com/vibepilot/governor/internal/pgnotify, encoding/json, github.com/vibepilot/governor/internal/vault, github.com/vibepilot/governor/internal/webhooks, github.com/vibepilot/governor/internal/kb, github.com/vibepilot/governor/internal/visualqa, github.com/vibepilot/governor/internal/designpreview
  API:
    fn main()
    fn getConfigDir() → string
    fn getEnvOrDefault(key, defaultVal string) → string
    fn runVaultCLI(args []string)
    fn registerConnectors(factory *runtime.SessionFactory, cfg *runtime.Config, v *vault.Vault, repoPath string)
    fn setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database db.Database, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, connRouter *runtime.Router, git *gitree.Gitree, stateMachine *core.StateMachine, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v *vault.Vault, configDir string, contextBuilder *runtime.ContextBuilder)
    cl visualQAServerAdapter
      fn ⊛ Run(ctx context.Context, triggeredBy, triggerDetail string) → (any, error)
      fn ⊛ GetEnabled() → bool
      fn ⊛ ApproveBaseline(ctx context.Context, pageName string, viewportWidth int) → error
      fn ⊛ RecordIssueFeedback(ctx context.Context, runID, issueType, issueElement, issueDescription string, viewport int, verdict, userNote, patternKey string) → error
      fn ⊛ GetIssueFeedback(ctx context.Context) → ([]map[string]any, error)
    cl vqaDBAdapter
    cl dpDBAdapter
      fn ⊛ Insert(ctx context.Context, table string, data map[string]any) → (json.RawMessage, error)
      fn ⊛ Update(ctx context.Context, table, id string, data map[string]any) → (json.RawMessage, error)
      fn ⊛ Query(ctx context.Context, table string, filters map[string]any) → (json.RawMessage, error)
    cl dpServerAdapter
      fn ⊛ Generate(ctx context.Context, req webhooks.DesignPreviewRequest) → (*webhooks.DesignPreviewResponse, error)
      fn ⊛ Approve(ctx context.Context, previewID, reviewer, notes string) → error
      fn ⊛ Reject(ctx context.Context, previewID, reviewer, notes string) → error
      fn ⊛ GetEnabled() → bool

## governor/cmd/governor/pipeline_events.go
pipeline_events.go [43L]
  deps: context, github.com/vibepilot/governor/internal/db, log
  API:
    fn recordPipelineEvent(ctx context.Context, database db.Database, eventType, taskID, modelID, reason string, details map[string]any)

## governor/cmd/governor/recovery.go
recovery.go [443L]
  deps: log, github.com/vibepilot/governor/internal/runtime, github.com/vibepilot/governor/internal/db, time, encoding/json, context, fmt, github.com/vibepilot/governor/internal/core
  API:
    fn getRecoveryConfig(cfg *runtime.Config) → RecoveryConfig
    fn runStartupRecovery(ctx context.Context, database db.Database, cfg RecoveryConfig)
    fn runProcessingRecovery(ctx context.Context, database db.Database, cfg *runtime.Config)
    fn recoverStaleProcessing(ctx context.Context, database db.Database, table string, timeout int)
    fn runCheckpointRecovery(ctx context.Context, database db.Database, cfg *runtime.Config, checkpointMgr *core.CheckpointManager)
    fn recoverOrphanedPlans(ctx context.Context, database db.Database)
    fn recoverOrphanedTasks(ctx context.Context, database db.Database)
    fn recoverPendingResources(ctx context.Context, database db.Database)
    fn runStartupRehydration(ctx context.Context, database db.Database, router *runtime.EventRouter)

## governor/cmd/governor/smoke.go
smoke.go [252L]
  deps: fmt, net/http, io, os, encoding/json, bytes, log, time
  API:
    fn runSmokeTest(dbURL, dbKey string)
    cl stageCheck
    fn taskSummaries(tasks []struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Title      string `json:"title"`
	TaskNumber int    `json:"task_number"`
}) → string
    fn smokeREST(dbURL, dbKey, method, path string, body map[string]any) → ([]byte, error)

## governor/cmd/governor/startup_validate.go
startup_validate.go [508L]
  deps: time, context, log, os/exec, path/filepath, os, fmt, encoding/json, strings
  API:
    fn startupValidate(configDir string, database interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
}) → int
    fn validateConfigDir(configDir string) → int
    fn validatePromptsDir(configDir string) → int
    fn validateConnectorCommands(configDir string) → int
    cl connectorStub
    cl connectorsConfig
    fn validateRPCs(ctx context.Context, database interface {
	RPC(ctx context.Context, name string, params map[string]interface{}) ([]byte, error)
}) → int
    fn validateAgentIDs(configDir string) → int
    cl agentStub
    cl agentsConfig
    cl connStub
    cl connectorsCfg
    cl agentFull
    cl agentsFull
    cl modelStub
    cl modelsConfig
    fn validateCascadeModelIDs(configDir string) → int
    cl modelStub
    cl modelsConfig
    cl strategyStub
    cl routingConfig
    fn loadJSONFile(path string) → (*T, error)
    cl startupDBAdapter
      fn ⊛ RPC(ctx context.Context, name string, params map[string]interface{}) → ([]byte, error)

## governor/cmd/governor/types.go
types.go [7L]
  API:
    cl ⊛ RecoveryConfig

## governor/cmd/governor/validation.go
validation.go [474L]
  deps: fmt, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/runtime, strconv, context, log, strings, regexp, encoding/json
  API:
    cl ⊛ TaskData
    cl ⊛ ValidationError
      fn ⊛ Error() → string
    cl ⊛ ValidationFailedError
      fn ⊛ Error() → string
    fn validateTasks(tasks []TaskData, cfg *runtime.ValidationConfig) → *ValidationFailedError
    fn createTasksFromApprovedPlan(ctx context.Context, database db.Database, plan map[string]any, cfg *runtime.ValidationConfig, repoPath string, git *gitree.Gitree) → error
    fn parseTasksFromPlanMarkdown(content string) → ([]TaskData, error)
    fn parseTaskSection(section string) → (TaskData, error)
    fn extractSection(body, heading string) → string
    fn sanitizeFilePaths(paths []string) → []string

## governor/cmd/migrate_vault/main.go
main.go [200L]
  deps: log, net/http, crypto/cipher, crypto/rand, bytes, crypto/aes, encoding/base64, io, os, crypto/sha256, fmt, golang.org/x/crypto/pbkdf2, encoding/json
  API:
    fn main()
    cl ⊛ Secret
    fn fetchSecrets(baseURL, serviceKey string) → ([]Secret, error)
    fn updateSecret(baseURL, serviceKey, keyName, encryptedValue string) → error
    fn decryptOld(encrypted, masterKey string) → (string, error)
    fn encryptNew(plaintext, masterKey string) → (string, error)

## governor/cmd/vault_decrypt/main.go
main.go [58L]
  deps: os, crypto/sha256, golang.org/x/crypto/pbkdf2, crypto/aes, crypto/cipher, fmt, encoding/base64
  API:
    fn main()

## governor/cmd/vault_encrypt/main.go
main.go [27L]
  deps: log, github.com/vibepilot/governor/internal/vault, os, fmt
  API:
    fn main()

## governor/internal/connectors/courier.go
courier.go [577L]
  deps: time, io, bytes, net/http, context, os/exec, encoding/json, sync, os, fmt, strings
  exports: NewCourierRunner
  API:
    if ⊛ CourierDB
    cl courierWaiter
    cl ⊛ CourierRunner
    cl platformConfig
    fn ⊛ NewCourierRunner(githubToken, githubRepo string, db CourierDB, timeoutSecs int) → *CourierRunner
      fn ⊛ SetGovernorURL(url string)
    fn detectPlatform(url string) → (string, platformConfig, bool)
      fn ⊛ Run(ctx context.Context, prompt string, timeout int) → (string, int, int, error)
      fn ⊛ NotifyResult(taskID string, result *TaskRunResult)
    cl ⊛ TaskRunResult
    fn truncateCourierID(id string) → string
    fn extractPlatformID(url string) → string
    fn minInt(a, b int) → int

## governor/internal/connectors/runners.go
runners.go [513L]
  deps: context, os/exec, bufio, time, bytes, io, github.com/vibepilot/governor/internal/runtime, encoding/json, fmt, net/http, github.com/vibepilot/governor/internal/vault, strings
  exports: NewCLIRunner, NewCLIRunnerWithArgs, NewCLIRunnerWithWorkDir, NewAPIRunner, NewAPIRunnerFromConfig, NewVaultAdapter
  API:
    if ⊛ SecretProvider
    cl ⊛ CLIRunner
    fn ⊛ NewCLIRunner(command string, timeoutSecs int) → *CLIRunner
    fn ⊛ NewCLIRunnerWithArgs(command string, cliArgs []string, timeoutSecs int) → *CLIRunner
    fn ⊛ NewCLIRunnerWithWorkDir(command string, cliArgs []string, timeoutSecs int, workDir string) → *CLIRunner
      fn ⊛ Run(ctx context.Context, prompt string, timeout int) → (string, int, int, error)
    fn stripUICrhome(output string) → string
      fn ⊛ RunWithSystemPrompt(ctx context.Context, systemPrompt, userPrompt string, timeout int) → (string, int, int, error)
    cl ⊛ APIRunner
    cl ⊛ APIRunnerConfig
    fn ⊛ NewAPIRunner(cfg *APIRunnerConfig) → *APIRunner
    fn ⊛ NewAPIRunnerFromConfig(conn runtime.ConnectorConfig, secrets SecretProvider) → *APIRunner
    fn detectProvider(endpoint string) → string
      fn ⊛ Run(ctx context.Context, prompt string, timeout int) → (string, int, int, error)
      fn ⊛ HealthCheck(ctx context.Context) → error
    fn parseGeminiResponse(body []byte) → (string, int, int)
    fn parseOpenAIResponse(body []byte) → (string, int, int)
    cl ⊛ VaultAdapter
    fn ⊛ NewVaultAdapter(v *vault.Vault) → *VaultAdapter
      fn ⊛ GetSecret(ctx context.Context, keyName string) → (string, error)

## governor/internal/core/analyst.go
analyst.go [116L]
  deps: time, encoding/json, context, fmt
  exports: NewAnalyst
  API:
    cl ⊛ Analyst
    if ⊛ DBInterface
    cl ⊛ AnalysisResult
    fn ⊛ NewAnalyst(sm *StateMachine, db DBInterface, checkpointMgr *CheckpointManager) → *Analyst
      fn ⊛ RunDailyAnalysis(ctx context.Context) → (*AnalysisResult, error)

## governor/internal/core/checkpoint.go
checkpoint.go [143L]
  deps: fmt, encoding/json, time, context
  exports: NewCheckpointManager, NewMemoryCheckpointStorage, NewDBCheckpointStorage
  API:
    cl ⊛ CheckpointManager
    if ⊛ CheckpointStorage
    fn ⊛ NewCheckpointManager(sm *StateMachine, storage CheckpointStorage) → *CheckpointManager
      fn ⊛ SaveProgress(ctx context.Context, taskID string, step string, progress int, output string, files []string) → error
      fn ⊛ Resume(ctx context.Context, taskID string) → (*Checkpoint, error)
      fn ⊛ Complete(ctx context.Context, taskID string) → error
    cl ⊛ MemoryCheckpointStorage
    fn ⊛ NewMemoryCheckpointStorage() → *MemoryCheckpointStorage
      fn ⊛ Save(ctx context.Context, taskID string, checkpoint Checkpoint) → error
      fn ⊛ Load(ctx context.Context, taskID string) → (*Checkpoint, error)
      fn ⊛ Delete(ctx context.Context, taskID string) → error
    cl ⊛ DBCheckpointStorage
    fn ⊛ NewDBCheckpointStorage(db interface {
	RPC(ctx context.Context, fn string, args map[string]any) (json.RawMessage, error)
}) → *DBCheckpointStorage
      fn ⊛ Save(ctx context.Context, taskID string, checkpoint Checkpoint) → error
      fn ⊛ Load(ctx context.Context, taskID string) → (*Checkpoint, error)
      fn ⊛ Delete(ctx context.Context, taskID string) → error

## governor/internal/core/state.go
state.go [299L]
  deps: fmt, time, sync, context, encoding/json
  exports: NewStateMachine
  API:
    cl ⊛ SystemState
    cl ⊛ Metrics
    cl ⊛ AgentState
    cl ⊛ PlanState
    cl ⊛ TaskState
    cl ⊛ SliceState
    cl ⊛ FailureState
    cl ⊛ LearningState
    cl ⊛ ModelScore
    cl ⊛ Pattern
    cl ⊛ ImprovementSuggestion
    cl ⊛ Checkpoint
    cl ⊛ TaskRef
    ty ⊛ EventType
    cl ⊛ Event
    cl ⊛ StateMachine
    ty ⊛ EventHandler
    fn ⊛ NewStateMachine() → *StateMachine
      fn ⊛ RegisterHandler(eventType EventType, handler EventHandler)
      fn ⊛ Emit(ctx context.Context, event Event) → error
      fn ⊛ GetState() → *SystemState
      fn ⊛ UpdatePlan(planID string, update func(*PlanState))
      fn ⊛ UpdateTask(taskID string, update func(*TaskState))
      fn ⊛ AddPlan(plan PlanState)
      fn ⊛ AddTask(task TaskState)
      fn ⊛ ToJSON() → ([]byte, error)

## governor/internal/core/test_runner.go
test_runner.go [296L]
  deps: time, encoding/json, context, fmt, path/filepath, strings, os/exec, os
  exports: NewTestRunner
  API:
    cl ⊛ TestRunner
    cl ⊛ TestResult
    cl ⊛ TestConfig
    fn ⊛ NewTestRunner(sm *StateMachine, repoPath string, sandboxDir string, timeoutSecs int) → *TestRunner
      fn ⊛ RunTests(ctx context.Context, taskID string, config TestConfig) → (*TestResult, error)
      fn ⊛ RunLint(ctx context.Context, taskID string, config TestConfig) → (*TestResult, error)
      fn ⊛ RunTypecheck(ctx context.Context, taskID string, config TestConfig) → (*TestResult, error)
      fn ⊛ ToJSON(result *TestResult) → ([]byte, error)

## governor/internal/dag/engine.go
engine.go [233L]
  deps: sync, time, context, fmt, log, strings
  exports: NewEngine
  API:
    cl ⊛ NodeOutput
    if ⊛ NodeExecutor
    cl ⊛ Engine
    fn ⊛ NewEngine(workflow *Workflow, executors ...NodeExecutor) → *Engine
      fn ⊛ Run(ctx context.Context, variables map[string]string) → error
      fn ⊛ GetOutputs() → map[string]NodeOutput

## governor/internal/dag/registry.go
registry.go [123L]
  deps: os, sync, fmt, path/filepath
  exports: NewRegistry
  API:
    cl ⊛ Registry
    fn ⊛ NewRegistry(pipelinesDir string) → *Registry
      fn ⊛ LoadAll() → error
      fn ⊛ Get(name string) → *Workflow
      fn ⊛ List() → []string
      fn ⊛ Reload() → error

## governor/internal/dag/workflow.go
workflow.go [212L]
  deps: gopkg.in/yaml.v3, fmt
  exports: LoadWorkflow, TopologicalLayers
  API:
    cl ⊛ Workflow
    cl ⊛ Node
    cl ⊛ PromptNode
    cl ⊛ BashNode
    cl ⊛ ApprovalNode
    cl ⊛ AgentNode
    cl ⊛ EmitNode
    cl ⊛ LoopNode
    cl ⊛ RetryConfig
    fn ⊛ LoadWorkflow(data []byte) → (*Workflow, error)
    fn detectCycle(wf *Workflow) → error
    fn ⊛ TopologicalLayers(wf *Workflow) → [][]Node

## governor/internal/db/interface.go
interface.go [40L]
  deps: time, context, encoding/json
  API:
    if ⊛ Database

## governor/internal/db/postgres.go
postgres.go [693L]
  deps: encoding/json, reflect, context, github.com/jackc/pgx/v5/pgtype, github.com/jackc/pgx/v5/pgxpool, encoding/binary, strconv, strings, fmt, time, github.com/jackc/pgx/v5
  exports: NewPostgres
  API:
    cl ⊛ PostgresDB
    fn ⊛ NewPostgres(ctx context.Context, connString string) → (*PostgresDB, error)
      fn ⊛ Close() → error
      fn ⊛ Query(ctx context.Context, table string, filters map[string]any) → (json.RawMessage, error)
      fn ⊛ Insert(ctx context.Context, table string, data map[string]any) → (json.RawMessage, error)
      fn ⊛ Update(ctx context.Context, table, id string, data map[string]any) → (json.RawMessage, error)
      fn ⊛ Delete(ctx context.Context, table, id string) → error
      fn ⊛ RPC(ctx context.Context, name string, params map[string]interface{}) → ([]byte, error)
      fn ⊛ CallRPC(ctx context.Context, name string, params map[string]any) → (json.RawMessage, error)
      fn ⊛ CallRPCInto(ctx context.Context, name string, params map[string]any, dest any) → error
      fn ⊛ RecordStateTransition(ctx context.Context, entityType, entityID, fromState, toState, reason string, metadata map[string]any) → error
      fn ⊛ RecordPerformanceMetric(ctx context.Context, metricType, entityID string, duration time.Duration, success bool, metadata map[string]any) → error
      fn ⊛ GetLatestState(ctx context.Context, entityType, entityID string) → (toState string, reason string, createdAt time.Time, err error)
      fn ⊛ ClearProcessingAndRecordTransition(ctx context.Context, table, id, fromState, toState, reason string) → error
      fn ⊛ GetDestination(ctx context.Context, id string) → (*Destination, error)
      fn ⊛ GetRunners(ctx context.Context) → ([]Runner, error)
      fn ⊛ GetTaskPacket(ctx context.Context, taskID string) → (*TaskPacket, error)
    fn buildFilterClause(col, val string, argIdx int) → (string, int, []any)
    fn rowsToJSON(rows pgx.Rows) → (json.RawMessage, error)
    fn convertValue(v any) → any
    fn toInt(v any) → (int, bool)

## governor/internal/db/rpc.go
rpc.go [303L]
  deps: encoding/json, sync, fmt, context
  exports: NewRPCAllowlist, ParseRPCCall
  API:
    cl ⊛ RPCAllowlist
    fn ⊛ NewRPCAllowlist() → *RPCAllowlist
      fn ⊛ Add(name string)
      fn ⊛ Remove(name string)
      fn ⊛ Allowed(name string) → bool
      fn ⊛ List() → []string
      fn ⊛ CallRPC(ctx context.Context, name string, params map[string]any) → (json.RawMessage, error)
      fn ⊛ CallRPCInto(ctx context.Context, name string, params map[string]any, dest any) → error
    cl ⊛ RPCCall
    fn ⊛ ParseRPCCall(data string) → (*RPCCall, error)

## governor/internal/db/state.go
state.go [86L]
  deps: context, fmt, encoding/json, time
  API:
      fn ⊛ RecordStateTransition(ctx context.Context, entityType string, entityID string, fromState string, toState string, reason string, metadata map[string]any) → error
      fn ⊛ RecordPerformanceMetric(ctx context.Context, metricType string, entityID string, duration time.Duration, success bool, metadata map[string]any) → error
      fn ⊛ GetLatestState(ctx context.Context, entityType string, entityID string) → (toState string, reason string, createdAt time.Time, err error)
      fn ⊛ ClearProcessingAndRecordTransition(ctx context.Context, table string, id string, fromState string, toState string, reason string) → error

## governor/internal/db/supabase.go
supabase.go [285L]
  deps: net/http, fmt, bytes, context, io, encoding/json, regexp, net/url, strings, time
  exports: New, NewWithConfig
  API:
    fn isValidTableName(name string) → bool
    cl ⊛ DBConfig
    cl ⊛ DB
    fn ⊛ New(url, key string) → *DB
    fn ⊛ NewWithConfig(url, key string, cfg *DBConfig) → *DB
      fn ⊛ Close() → error
      fn ⊛ REST(ctx context.Context, method, path string, body interface{}) → ([]byte, error)
      fn ⊛ RESTWithHeaders(ctx context.Context, method, path string, body interface{}, extraHeaders map[string]string) → ([]byte, error)
      fn ⊛ RPC(ctx context.Context, name string, params map[string]interface{}) → ([]byte, error)
      fn ⊛ Query(ctx context.Context, table string, filters map[string]any) → (json.RawMessage, error)
      fn ⊛ Insert(ctx context.Context, table string, data map[string]any) → (json.RawMessage, error)
      fn ⊛ Update(ctx context.Context, table, id string, data map[string]any) → (json.RawMessage, error)
      fn ⊛ Delete(ctx context.Context, table, id string) → error
    cl ⊛ Destination
      fn ⊛ GetDestination(ctx context.Context, id string) → (*Destination, error)
    cl ⊛ Runner
      fn ⊛ GetRunners(ctx context.Context) → ([]Runner, error)
    cl ⊛ TaskPacket
      fn ⊛ GetTaskPacket(ctx context.Context, taskID string) → (*TaskPacket, error)

## governor/internal/designpreview/designpreview.go
designpreview.go [176L]
  deps: context, encoding/json, fmt, time, github.com/google/uuid
  exports: NewDesignPreview, EnsureDBSchema
  API:
    cl ⊛ Config
    if ⊛ DB
    ty ⊛ PreviewStatus
    cl ⊛ DesignPreview
    cl ⊛ GenerateRequest
    cl ⊛ GenerateResponse
    fn ⊛ NewDesignPreview(config Config, apiKey string, db DB) → *DesignPreview
      fn ⊛ Generate(ctx context.Context, req GenerateRequest) → (*GenerateResponse, error)
      fn ⊛ Approve(ctx context.Context, previewID, reviewer, notes string) → error
      fn ⊛ Reject(ctx context.Context, previewID, reviewer, notes string) → error
      fn ⊛ GetEnabled() → bool
    fn ⊛ EnsureDBSchema() → error
      fn ⊛ MarshalJSON() → ([]byte, error)

## governor/internal/designpreview/generator.go
generator.go [236L]
  deps: fmt, net/http, context, os, os/exec, path/filepath, encoding/base64, encoding/json, io, bytes, time
  exports: NewGenerator
  API:
    cl ⊛ Generator
    fn ⊛ NewGenerator(config Config, apiKey string) → *Generator
      fn ⊛ GenerateHTMLMockup(ctx context.Context, taskTitle, taskDescription string) → (string, int, int, error)
      fn ⊛ SaveHTML(repoPath, designDir, taskID string, version int, htmlContent string) → (string, error)
      fn ⊛ CaptureScreenshot(ctx context.Context, htmlFilePath string, viewportWidth int) → (string, error)
    fn stripMarkdownCodeBlock(text string) → string
    fn findClosingBackticks(text string, startFrom int) → int
    fn encodeFileToBase64(filePath string) → (string, error)

## governor/internal/designpreview/persist.go
persist.go [94L]
  deps: context, path/filepath, fmt, time, github.com/google/uuid, os, encoding/json
  exports: SaveManifest, LoadManifest, AddToManifest
  API:
    cl ⊛ Manifest
    cl ⊛ ManifestEntry
    cl ⊛ PGXDB
    fn ⊛ SaveManifest(repoPath, manifestFile string, manifest *Manifest) → error
    fn ⊛ LoadManifest(repoPath, manifestFile string) → (*Manifest, error)
    fn ⊛ AddToManifest(repoPath, manifestFile string, taskID string, version int, htmlPath, screenshotPath string) → error

## governor/internal/gitree/gitree.go
gitree.go [976L]
  deps: regexp, bytes, fmt, log, strings, time, context, encoding/json, path/filepath, os/exec, os
  exports: New
  API:
    fn isValidBranchName(name string) → bool
    cl ⊛ Gitree
    cl ⊛ Config
    fn ⊛ New(cfg *Config) → *Gitree
      fn ⊛ MainBranch() → string
      fn ⊛ ResetToMain(ctx context.Context) → error
      fn ⊛ CreateBranch(ctx context.Context, branchName string) → error
      fn ⊛ CreateBranchFrom(ctx context.Context, branchName, sourceBranch string) → error
      fn ⊛ CommitOutput(ctx context.Context, branchName string, output interface{}) → error
      fn ⊛ ReadBranchOutput(ctx context.Context, branchName string) → ([]string, error)
      fn ⊛ MergeBranch(ctx context.Context, sourceBranch, targetBranch string) → error
      fn ⊛ MergeBranchToSubdir(ctx context.Context, sourceBranch, targetBranch, prefix string) → error
      fn ⊛ DeleteBranch(ctx context.Context, branchName string) → error
      fn ⊛ ClearBranch(ctx context.Context, branchName string) → error
      fn ⊛ CreateModuleBranch(ctx context.Context, sliceID string) → error
      fn ⊛ CommitAndPush(ctx context.Context, filePath, message string) → error
      fn ⊛ Pull(ctx context.Context) → error
      fn ⊛ CommitOutputToWorktree(ctx context.Context, worktreePath string, branchName string, output interface{}) → error
    cl ⊛ DiskFile
      fn ⊛ ScanWorktreeFiles(ctx context.Context, worktreePath string, excludePaths []string) → ([]DiskFile, error)
      fn ⊛ ScanBranchFiles(ctx context.Context, branchName string, excludePaths []string) → ([]DiskFile, error)
      fn ⊛ DiffWorktreeFiles(ctx context.Context, worktreePath string) → ([]DiskFile, error)

## governor/internal/gitree/managed_repo.go
managed_repo.go [310L]
  deps: fmt, context, path/filepath, strings, os/exec, log, time, os
  exports: NewManagedRepo
  API:
    cl ⊛ ManagedRepo
    cl ⊛ ManagedRepoConfig
    fn ⊛ NewManagedRepo(ctx context.Context, cfg ManagedRepoConfig) → (*ManagedRepo, error)
      fn ⊛ Gitree() → *Gitree
      fn ⊛ LocalPath() → string
      fn ⊛ WorktreeBasePath() → string
      fn ⊛ Reset(ctx context.Context) → error
      fn ⊛ CleanStaleBranches(ctx context.Context)
      fn ⊛ CleanStaleWorktrees(ctx context.Context)

## governor/internal/gitree/worktree.go
worktree.go [416L]
  deps: os, path/filepath, log, regexp, strings, context, fmt, time
  exports: NewWorktreeManager, TaskBranchName
  API:
    cl ⊛ WorktreeManager
    cl ⊛ WorktreeInfo
    fn ⊛ NewWorktreeManager(g *Gitree, basePath string) → *WorktreeManager
      fn ⊛ CreateWorktree(ctx context.Context, taskID, branchName string) → (*WorktreeInfo, error)
      fn ⊛ RemoveWorktree(ctx context.Context, taskID string) → error
      fn ⊛ GetWorktreePath(taskID string) → string
      fn ⊛ ListWorktrees(ctx context.Context) → ([]WorktreeInfo, error)
      fn ⊛ PruneWorktrees(ctx context.Context) → error
      fn ⊛ CleanAllWorktrees(ctx context.Context) → error
    fn ⊛ TaskBranchName(taskID, slug string) → string
    cl ⊛ ShadowMergeResult
      fn ⊛ ShadowMerge(ctx context.Context, sourceBranch, targetBranch string) → (*ShadowMergeResult, error)
      fn ⊛ BootstrapWorktree(ctx context.Context, worktreePath string) → error
      fn ⊛ BasePath() → string

## governor/internal/hello/hello.go
hello.go [6L]
  exports: Greet
  API:
    fn ⊛ Greet(name string) → string

## governor/internal/kb/kb.go
kb.go [344L]
  deps: context, github.com/jackc/pgx/v5/pgxpool, fmt, strings
  exports: New, ResolveFileID, ParseFileID
  API:
    cl ⊛ KB
    cl ⊛ Symbol
    cl ⊛ DocSection
    cl ⊛ Edge
    cl ⊛ KnowledgeItem
    cl ⊛ BlastRadiusEntry
    cl ⊛ Skill
    cl ⊛ StatsEntry
    fn ⊛ New(ctx context.Context, connString string) → (*KB, error)
      fn ⊛ Close()
      fn ⊛ SearchSymbols(ctx context.Context, query string, filterKind, filterRepo *string, limit int) → ([]Symbol, error)
      fn ⊛ SearchDocs(ctx context.Context, query string, filterRepo *string, limit int) → ([]DocSection, error)
      fn ⊛ GetCallers(ctx context.Context, qualifiedName string, limit int) → ([]Edge, error)
      fn ⊛ GetCallees(ctx context.Context, qualifiedName string, limit int) → ([]Edge, error)
      fn ⊛ GetBlastRadius(ctx context.Context, qualifiedName string, maxDepth int) → ([]BlastRadiusEntry, error)
      fn ⊛ GetFileSymbols(ctx context.Context, fileID string, limit int) → ([]Symbol, error)
      fn ⊛ SearchKnowledge(ctx context.Context, query string, filterType *string, limit int) → ([]KnowledgeItem, error)
      fn ⊛ SearchSkills(ctx context.Context, query string, filterCategory *string, limit int) → ([]Skill, error)
      fn ⊛ Stats(ctx context.Context) → ([]StatsEntry, error)
    fn ⊛ ResolveFileID(repoID, relativePath string) → string
    fn ⊛ ParseFileID(fileID string) → (repoID, relativePath string)
    cl ⊛ SemanticResult
      fn ⊛ SearchAllSemantic(ctx context.Context, embedding string, limit int, minSimilarity float64) → ([]SemanticResult, error)

## governor/internal/maintenance/maintenance.go
maintenance.go [346L]
  deps: strings, log, os, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/pkg/types, path/filepath, context, fmt, github.com/vibepilot/governor/internal/db
  exports: New
  API:
    ty ⊛ RiskLevel
    ty ⊛ ChangeType
    cl ⊛ Change
    cl ⊛ ExecutionResult
    cl ⊛ Maintenance
    cl ⊛ Config
    fn ⊛ New(cfg *Config, database db.Database, git *gitree.Gitree) → *Maintenance
      fn ⊛ ClassifyRisk(change *Change) → RiskLevel
      fn ⊛ RequiresSandbox(change *Change) → bool
      fn ⊛ Execute(ctx context.Context, task *types.Task, packet *types.PromptPacket, output interface{}) → (*ExecutionResult, error)
      fn ⊛ CheckApprovalChain(ctx context.Context, change *Change) → error
      fn ⊛ IsSystemChange(change *Change) → bool
      fn ⊛ RepoPath() → string

## governor/internal/maintenance/sandbox.go
sandbox.go [165L]
  deps: os, context, time, fmt, log, path/filepath, io, os/exec
  API:
      fn ⊛ CreateSandbox() → (string, error)
      fn ⊛ ApplyToSandbox(sandboxPath string, change *Change) → error
    cl ⊛ SandboxTestResult
      fn ⊛ TestInSandbox(sandboxPath string) → (SandboxTestResult, error)
      fn ⊛ CleanupSandbox(sandboxPath string) → error
    fn copyFile(src, dst string) → error
    fn skipDir(name string) → bool
    fn skipFile(name string) → bool

## governor/internal/maintenance/validation.go
validation.go [248L]
  deps: path/filepath, time, encoding/json, strings, fmt, log, os
  API:
      fn ⊛ Backup(target string) → (string, error)
      fn ⊛ Rollback(backupPath, target string) → error
      fn ⊛ ValidateConfig(path string, content []byte) → error
      fn ⊛ ValidatePlatforms(content []byte) → error
      fn ⊛ ValidateModels(content []byte) → error
      fn ⊛ ValidateRoles(content []byte) → error
      fn ⊛ CanRollback(changeID string) → (bool, error)
      fn ⊛ GetBackups() → ([]string, error)
      fn ⊛ CleanupOldBackups(maxAge time.Duration) → error

## governor/internal/mcp/executor.go
executor.go [44L]
  deps: github.com/vibepilot/governor/internal/runtime, context, fmt, encoding/json
  exports: NewMCPToolExecutor
  API:
    cl ⊛ MCPToolExecutor
    fn ⊛ NewMCPToolExecutor(registry *Registry, toolName string) → *MCPToolExecutor
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
      fn ⊛ RegisterToolsInRegistry(toolRegistry *runtime.ToolRegistry)

## governor/internal/mcp/governor_server.go
governor_server.go [211L]
  deps: encoding/json, fmt, context, strconv, github.com/mark3labs/mcp-go/server, github.com/vibepilot/governor/internal/runtime, github.com/mark3labs/mcp-go/mcp, os, log
  exports: NewGovernorServer
  API:
    cl ⊛ GovernorServer
    fn ⊛ NewGovernorServer(registry *runtime.ToolRegistry, config *runtime.Config, cfg runtime.GovernorMCPConfig) → *GovernorServer
      fn ⊛ Start(ctx context.Context) → error
      fn ⊛ Shutdown()
    fn jsonEscape(s string) → string

## governor/internal/mcp/registry.go
registry.go [253L]
  deps: github.com/mark3labs/mcp-go/mcp, github.com/mark3labs/mcp-go/client, context, sync, encoding/json, log, time, github.com/mark3labs/mcp-go/client/transport, github.com/vibepilot/governor/internal/runtime, fmt
  exports: NewRegistry
  API:
    cl ⊛ ToolBinding
    cl ⊛ Registry
    fn ⊛ NewRegistry(configs []runtime.MCPServerConfig) → *Registry
      fn ⊛ Start(ctx context.Context) → error
      fn ⊛ CallTool(ctx context.Context, toolName string, args map[string]any) → (json.RawMessage, error)
      fn ⊛ ListTools() → []ToolBinding
      fn ⊛ ListToolInfo() → []runtime.MCPToolInfo
      fn ⊛ HasTool(name string) → bool
      fn ⊛ ToolDescription(name string) → string
      fn ⊛ Shutdown()

## governor/internal/memory/compactor.go
compactor.go [248L]
  deps: encoding/json, context, strings, time, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/runtime, fmt
  exports: NewCompactor
  API:
    cl ⊛ SessionSummary
    cl ⊛ Compactor
    fn ⊛ NewCompactor(dbClient db.Database) → *Compactor
      fn ⊛ Compact(ctx context.Context, result *runtime.SessionResult, taskID string) → (*SessionSummary, error)
      fn ⊛ CompactSession(ctx context.Context, result *runtime.SessionResult, taskID string)
      fn ⊛ GetRecentSummaries(ctx context.Context, agentID string, limit int) → ([]SessionSummary, error)
      fn ⊛ BuildCompactionContext(ctx context.Context, taskID string, maxSummaries int) → (string, error)

## governor/internal/memory/service.go
service.go [302L]
  deps: github.com/vibepilot/governor/internal/db, context, encoding/json, fmt, time
  exports: New
  API:
    cl ⊛ Config
    cl ⊛ Rule
    cl ⊛ MemoryService
    fn ⊛ New(database db.Database, cfg Config) → *MemoryService
      fn ⊛ StoreShortTerm(ctx context.Context, sessionID, agentType string, contextData map[string]any) → error
      fn ⊛ GetShortTerm(ctx context.Context, sessionID string) → (map[string]any, error)
      fn ⊛ StoreProjectState(ctx context.Context, projectID, key string, value map[string]any) → error
      fn ⊛ GetProjectState(ctx context.Context, projectID, key string) → (map[string]any, error)
      fn ⊛ StoreRule(ctx context.Context, category, ruleText, source string, priority int) → error
      fn ⊛ GetRulesByCategory(ctx context.Context, category string) → ([]Rule, error)
      fn ⊛ GetRulesByPriority(ctx context.Context, minPriority int) → ([]Rule, error)
      fn ⊛ CleanExpired(ctx context.Context) → error

## governor/internal/notifications/alert_dedup.go
alert_dedup.go [216L]
  deps: strings, context, sync, fmt, log, encoding/json, os/exec
  exports: NewAlertDedup, SendEmailUnconditional
  API:
    if ⊛ DBInserter
    cl ⊛ AlertDedup
    fn ⊛ NewAlertDedup() → *AlertDedup
      fn ⊛ SetDB(db DBInserter)
      fn ⊛ AlreadySent(key string) → bool
      fn ⊛ MarkSent(key string)
      fn ⊛ Clear(key string)
      fn ⊛ ClearAll()
      fn ⊛ LoadFromDB(ctx context.Context)
      fn ⊛ SetDBWithLoad(ctx context.Context, db DBInserter, querier func(ctx context.Context, query string, args ...interface{}) ([]string, error))
      fn ⊛ SendEmailIfNew(key string, subject string, body string) → bool
    fn ⊛ SendEmailUnconditional(subject string, body string) → error
    cl ⊛ SubscriptionAlert
      fn ⊛ DedupKey() → string
    if ⊛ DBQuerier
      fn ⊛ CheckAndAlert(ctx context.Context, db DBQuerier) → int

## governor/internal/pgnotify/listener.go
listener.go [291L]
  deps: log, time, fmt, github.com/jackc/pgx/v5, github.com/vibepilot/governor/internal/runtime, encoding/json, context
  exports: NewListener
  API:
    if ⊛ SSEBroadcaster
    cl ⊛ Listener
    fn ⊛ NewListener(ctx context.Context, connString string, router *runtime.EventRouter, broadcaster SSEBroadcaster) → (*Listener, error)
      fn ⊛ Close() → error
    cl notifyPayload

## governor/internal/reviewitems/reviewitems.go
reviewitems.go [151L]
  deps: context, fmt, encoding/json, time, github.com/vibepilot/governor/internal/db
  exports: Create, List, UpdateStatus, Exists
  API:
    cl ⊛ ReviewItem
    fn ⊛ Create(ctx context.Context, database db.Database, item ReviewItem) → (*ReviewItem, error)
    fn ⊛ List(ctx context.Context, database db.Database, status string, itemType string) → ([]ReviewItem, error)
    fn ⊛ UpdateStatus(ctx context.Context, database db.Database, id string, status string, notes string) → (*ReviewItem, error)
    fn ⊛ Exists(ctx context.Context, database db.Database, itemType string, sourceID string, status string) → (bool, error)

## governor/internal/runtime/config.go
config.go [1415L]
  deps: sync, context, encoding/json, fmt, path/filepath, log, os
  exports: DefaultCodeMapConfig, LoadConfig
  API:
    cl ⊛ SystemConfig
    cl ⊛ DesignPreviewConfig
    cl ⊛ CodeMapConfig
    fn ⊛ DefaultCodeMapConfig() → *CodeMapConfig
    cl ⊛ GovernorMCPConfig
    cl ⊛ WorktreeConfig
    cl ⊛ MCPServerConfig
    cl ⊛ WebhooksConfig
    cl ⊛ CoreConfig
      fn ⊛ GetCheckpointIntervalPercent() → int
      fn ⊛ GetStateSyncIntervalSeconds() → int
      fn ⊛ IsCheckpointEnabled() → bool
      fn ⊛ IsRecoveryEnabled() → bool
    cl ⊛ ValidationConfig
    cl ⊛ DatabaseConfig
    cl ⊛ VaultConfig
    cl ⊛ DBConfig
    cl ⊛ HTTPConfig
    cl ⊛ ExecutionConfig
    cl ⊛ SessionConfig
    cl ⊛ CourierConfig
    cl ⊛ GitConfig
    cl ⊛ BranchPrefixConfig
    cl ⊛ LoggingConfig
    cl ⊛ RuntimeConfig
    cl ⊛ ConcurrencyConfig
      fn ⊛ GetLimit(destination string) → int
    cl ⊛ SecurityConfig
    cl ⊛ EventsConfig
    cl ⊛ SandboxConfig
    cl ⊛ WebToolsConfig
    cl ⊛ AgentConfig
      fn ⊛ HasCapability(capability string) → bool
    cl ⊛ AgentsFile
      fn ⊛ GetAgent(id string) → *AgentConfig
    cl ⊛ ToolParam
    cl ⊛ ToolConfig
    cl ⊛ ToolsFile
    cl ⊛ PlatformLimitSchema
    cl ⊛ ConnectorConfig
    cl ⊛ ConnectorsFile
    cl ⊛ ModelConfig
    cl ⊛ ModelsFile
    cl ⊛ RoutingStrategy
    cl ⊛ RoutingConfig
    cl ⊛ PlanLifecycleConfig
    cl ⊛ StateConfig
    cl ⊛ RevisionRulesConfig
    cl ⊛ ComplexityRulesConfig
    cl ⊛ ComplexityCondition
    cl ⊛ ConsensusRulesConfig
    cl ⊛ ConsensusMethodConfig
    cl ⊛ CouncilRulesConfig
    cl ⊛ CouncilStrategyConfig
    cl ⊛ CouncilContextConfig
    cl ⊛ Config
    if ⊛ PromptLoader
      fn ⊛ SetDatabase(db PromptLoader)
      fn ⊛ SyncPromptsToDB() → error
    fn ⊛ LoadConfig(configDir string) → (*Config, error)
      fn ⊛ Reload() → error
      fn ⊛ GetAgent(id string) → *AgentConfig
      fn ⊛ GetTool(name string) → *ToolConfig
      fn ⊛ GetConnector(id string) → *ConnectorConfig
      fn ⊛ GetModel(id string) → *ModelConfig
      fn ⊛ LoadPrompt(promptPath string) → (string, error)
      fn ⊛ AgentHasCapability(agentID, capability string) → bool
      fn ⊛ GetDatabaseURL() → string
      fn ⊛ SetPromptsDir(dir string)
      fn ⊛ GetDatabaseKey() → string
      fn ⊛ GetDatabaseType() → string
      fn ⊛ GetPostgresURL() → string
      fn ⊛ GetRealtimeURL() → string
      fn ⊛ GetVaultKey() → string
      fn ⊛ GetVaultKeyEnv() → string
      fn ⊛ GetProtectedBranches() → []string
      fn ⊛ GetRepoPath() → string
      fn ⊛ SetRepoPath(path string)
      fn ⊛ GetGitTimeout() → int
      fn ⊛ GetDefaultMergeTarget() → string
      fn ⊛ GetBranchNamePattern() → string
      fn ⊛ GetGitTimeoutSeconds() → int
      fn ⊛ GetRemoteName() → string
      fn ⊛ GetGitUserEmail() → string
      fn ⊛ GetGitUserName() → string
      fn ⊛ GetGitHubOwner() → string
      fn ⊛ GetGitHubRepoName() → string
      fn ⊛ GetDataDir() → string
      fn ⊛ GetWorktreeBasePath() → string
      fn ⊛ GetTaskBranchPrefix() → string
      fn ⊛ GetModuleBranchPrefix() → string
      fn ⊛ GetLoggingConfig() → *LoggingConfig
      fn ⊛ GetHTTPAllowlist() → []string
      fn ⊛ GetEventsConfig() → *EventsConfig
      fn ⊛ GetSandboxConfig() → *SandboxConfig
      fn ⊛ GetMaxRevisionRounds() → int
      fn ⊛ GetOnMaxRoundsAction() → string
      fn ⊛ GetCouncilLenses() → []string
      fn ⊛ GetCouncilMemberCount() → int
      fn ⊛ ShouldCouncilIncludePRD() → bool
      fn ⊛ GetConsensusMethod() → string
      fn ⊛ GetMaxRetries() → int
      fn ⊛ GetDiagnosticTriggerAttempts() → int
      fn ⊛ GetRepoSyncIntervalSeconds() → int
      fn ⊛ GetCooldownPollIntervalMs() → int
      fn ⊛ GetWebToolsConfig() → *WebToolsConfig
      fn ⊛ GetRuntimeConfig() → *RuntimeConfig
      fn ⊛ GetValidationConfig() → *ValidationConfig
      fn ⊛ GetCoreConfig() → *CoreConfig
      fn ⊛ GetWebhooksConfig() → *WebhooksConfig
      fn ⊛ IsWebhooksEnabled() → bool
      fn ⊛ GetProcessingTimeoutSeconds() → int
      fn ⊛ GetProcessingRecoveryIntervalSeconds() → int
      fn ⊛ GetOrphanThresholdSeconds() → int
      fn ⊛ GetRoutingStrategy(agentID string) → string
      fn ⊛ GetStrategyPriority(strategyName string) → []string
      fn ⊛ GetConnectorCategory(connID string) → string
      fn ⊛ GetConnectorsInCategory(category string) → []ConnectorConfig
    fn loadJSON(path string) → (*T, error)
      fn ⊛ GetRunnerTimeoutSecs() → int
      fn ⊛ GetSessionTimeoutSecs() → int
      fn ⊛ GetDBHTTPTimeoutSecs() → int
      fn ⊛ GetDBErrorTruncateLen() → int
      fn ⊛ GetSandboxTimeoutSecs() → int
      fn ⊛ GetLintTimeoutSecs() → int
      fn ⊛ GetTypecheckTimeoutSecs() → int
      fn ⊛ GetHTTPClientTimeoutSecs() → int
      fn ⊛ GetHTTPIdleTimeoutSecs() → int
      fn ⊛ GetCourierTimeoutSecs() → int
      fn ⊛ GetCourierExternalURL() → string
      fn ⊛ GetDefaultCLIArgs() → []string

## governor/internal/runtime/connector_tracker.go
connector_tracker.go [283L]
  deps: context, encoding/json, sync, time, log
  exports: NewConnectorUsageTracker
  API:
    cl ⊛ ConnectorProfile
    cl ⊛ ConnectorUsageTracker
    fn ⊛ NewConnectorUsageTracker(bufferPct int) → *ConnectorUsageTracker
      fn ⊛ RegisterConnector(connectorID string, sharedLimits RateLimits)
      fn ⊛ RecordUsage(ctx context.Context, connectorID string, tokensIn, tokensOut int)
      fn ⊛ CanMakeRequest(ctx context.Context, connectorID string, estimatedTokens int) → (canProceed bool, waitTime time.Duration)
      fn ⊛ HasConnector(connectorID string) → bool
      fn ⊛ GetConnectorStatus(connectorID string) → map[string]interface{}
      fn ⊛ PersistToDatabase(ctx context.Context, db Querier)
      fn ⊛ LoadFromDatabase(ctx context.Context, db Querier)

## governor/internal/runtime/context_builder.go
context_builder.go [609L]
  deps: context, strings, encoding/json, sync, time, fmt, path/filepath, os
  exports: NewContextBuilder
  API:
    if ⊛ RPCQuerier
    if ⊛ MCPToolLister
    cl ⊛ MCPToolInfo
    cl ⊛ ContextBuilder
    fn ⊛ NewContextBuilder(db RPCQuerier, repoPath string, cfg *CodeMapConfig) → *ContextBuilder
      fn ⊛ SetMCPRegistry(registry MCPToolLister)
      fn ⊛ InvalidateCache()
      fn ⊛ ReadFileContent(relPath string) → (string, bool)
      fn ⊛ BuildBaseContext() → string
      fn ⊛ BuildTargetedContext(targetFiles []string) → string
      fn ⊛ BuildPlannerContext(ctx context.Context, projectType string) → (string, error)
      fn ⊛ BuildKBContextPack(ctx context.Context, topic string) → string
    fn formatSymbolSection(sb *strings.Builder, raw json.RawMessage)
    fn formatDecisionSection(sb *strings.Builder, raw json.RawMessage)
    fn formatRuleSection(sb *strings.Builder, raw json.RawMessage)
    fn formatFileMapSection(sb *strings.Builder, raw json.RawMessage)
    fn formatRawSection(sb *strings.Builder, raw json.RawMessage)
    fn formatOverviewStruct(sb *strings.Builder, raw json.RawMessage)
      fn ⊛ BuildSupervisorContext(ctx context.Context, taskType string) → (string, error)
      fn ⊛ BuildCouncilContext(ctx context.Context, taskType string) → (string, error)
      fn ⊛ BuildTesterContext(ctx context.Context, taskType string) → (string, error)
      fn ⊛ GetRoutingHeuristic(ctx context.Context, taskType string) → (modelID string, action map[string]any)
      fn ⊛ GetProblemSolution(ctx context.Context, failureType, taskType string) → (solutionType string, solutionModel string, details map[string]any)

## governor/internal/runtime/context_builder_kb.go
context_builder_kb.go [188L]
  deps: fmt, sort, context, strings
  API:
    if ⊛ KBProvider
    cl ⊛ KBSymbol
    cl ⊛ KBDocSection
    cl ⊛ KBKnowledgeItem
    cl ⊛ KBStatsEntry
    cl kbFileGroup
      fn ⊛ SetKBProvider(kb KBProvider)
    fn symbolKindPrefix(kind string) → string

## governor/internal/runtime/cooldown_watcher.go
cooldown_watcher.go [223L]
  deps: log, context, sync, time
  exports: NewCooldownWatcher
  API:
    cl ⊛ CooldownWatcher
    if ⊛ ConnectorFactory
    fn ⊛ NewCooldownWatcher(tracker *UsageTracker, factory ConnectorFactory, cfg *Config, db Querier) → *CooldownWatcher
      fn ⊛ SetPollInterval(d time.Duration)
      fn ⊛ SetProbeTimeout(d time.Duration)
      fn ⊛ Start(ctx context.Context)
      fn ⊛ Stop()
    cl modelInfo

## governor/internal/runtime/credit_poller.go
credit_poller.go [347L]
  deps: log, encoding/json, io, strings, sync, net/http, github.com/vibepilot/governor/internal/notifications, fmt, time, context
  exports: NewCreditPoller
  API:
    if ⊛ CreditDB
    cl ⊛ CreditPoller
    cl creditPollTarget
    cl deepseekBalanceResponse
    fn ⊛ NewCreditPoller(tracker *PlatformUsageTrackerV2, vault VaultKeyGetter, db CreditDB) → *CreditPoller
      fn ⊛ StartBackgroundPolling(ctx context.Context)
      fn ⊛ GetBalances() → map[string]int

## governor/internal/runtime/decision_escape_fix.go
decision_escape_fix.go [16L]
  API:
    fn unescapePlanContent(s string) → string

## governor/internal/runtime/decision.go
decision.go [836L]
  deps: encoding/json, strings, regexp, fmt
  exports: ParseAnalystDecision, ParseAnalystDecisionFromMap, ParseResearchReview, ParseSupervisorDecision, ParseCouncilVote, ParseReportCouncilVotes, ParsePlannerOutput, ParseTestResults, ParseInitialReview, ParseTaskRunnerOutput, CategorizeFailure
  API:
    cl ⊛ Issue
    cl ⊛ AnalystFix
    cl ⊛ AnalystDecision
    fn ⊛ ParseAnalystDecision(output string) → (*AnalystDecision, error)
    fn ⊛ ParseAnalystDecisionFromMap(data map[string]any) → (*AnalystDecision, error)
    cl ⊛ SupervisorDecision
    cl ⊛ CouncilVote
    cl ⊛ PlannerOutput
    cl ⊛ TestResults
    cl ⊛ InitialReviewDecision
    cl ⊛ ResearchReviewDecision
    cl ⊛ MaintenanceCommand
    fn ⊛ ParseResearchReview(output string) → (*ResearchReviewDecision, error)
    cl ⊛ File
    cl ⊛ TaskRunnerOutput
    cl ⊛ TestSection
    fn ⊛ ParseSupervisorDecision(output string) → (*SupervisorDecision, error)
    fn parseIssues(raw json.RawMessage) → []Issue
    fn ⊛ ParseCouncilVote(output string) → (*CouncilVote, error)
    fn ⊛ ParseReportCouncilVotes(output string) → map[int]map[string]any
    fn ⊛ ParsePlannerOutput(output string) → (*PlannerOutput, error)
    fn ⊛ ParseTestResults(output string) → (*TestResults, error)
    fn ⊛ ParseInitialReview(output string) → (*InitialReviewDecision, error)
    fn ⊛ ParseTaskRunnerOutput(output string) → (*TaskRunnerOutput, error)
    fn tryJSONExtraction(output string) → *TaskRunnerOutput
    fn parseFilesArray(raw json.RawMessage) → []File
    fn hasFiles(files []File) → bool
    fn extractCodeBlockFiles(output string) → []File
    fn findFilenameNearBlock(output string, lang string) → string
    fn findFilenameForBlock(output string, blocks [][]string, idx int, lang string) → string
    fn langToExt(lang string) → string
    fn extractSummaryFromProse(output string) → string
    fn extractJSON(output string) → string
    fn ⊛ CategorizeFailure(issueType string) → string
    fn sanitizeJSON(input string) → string
    fn extractPlanContent(raw string) → (planContent string, cleanJSON string)

## governor/internal/runtime/events.go
events.go [131L]
  deps: encoding/json, time, log, context, sync
  exports: NewEventRouter
  API:
    ty ⊛ EventType
    cl ⊛ Event
    ty ⊛ EventHandler
    if ⊛ EventWatcher
    if ⊛ Querier
    cl ⊛ NopWatcher
      fn ⊛ Subscribe(ctx context.Context, handler EventHandler) → error
      fn ⊛ Close() → error
    cl ⊛ EventRouter
    fn ⊛ NewEventRouter(watcher EventWatcher) → *EventRouter
      fn ⊛ On(eventType EventType, handler EventHandler)
      fn ⊛ Start(ctx context.Context) → error
      fn ⊛ Route(event Event)
    fn hasCouncilReviews(plan map[string]any) → bool

## governor/internal/runtime/model_loader.go
model_loader.go [354L]
  deps: context, fmt, log, os, github.com/vibepilot/governor/internal/db, encoding/json, time, path/filepath
  exports: NewModelLoader, LoadModelsFromConfig
  API:
    cl ⊛ ModelsConfigFile
    cl ⊛ ModelLoader
    fn ⊛ NewModelLoader(configPath string, database db.Database, tracker *UsageTracker) → *ModelLoader
      fn ⊛ Load(ctx context.Context) → error
      fn ⊛ Reload(ctx context.Context) → error
    fn ⊛ LoadModelsFromConfig(configDir string, database db.Database, tracker *UsageTracker) → (*ModelLoader, error)
    fn loadConnectorSharedLimits(connectorsPath string, tracker *UsageTracker)
    fn loadWebPlatformLimits(connectorsPath string, tracker *UsageTracker)
      fn ⊛ GetModel(modelID string) → *ModelProfile
      fn ⊛ ListModels() → []string
      fn ⊛ GetActiveModels() → []string
      fn ⊛ GetAvailableModels(ctx context.Context) → []string
      fn ⊛ UpdateLearnedData(ctx context.Context, modelID string, learned LearnedData) → error

## governor/internal/runtime/model_scanner.go
model_scanner.go [519L]
  deps: sync, fmt, context, strings, encoding/json, net/http, strconv, io, time, log, github.com/vibepilot/governor/internal/db
  exports: NewModelScanner
  API:
    cl ⊛ ModelScannerConfig
    cl ⊛ ScannerProvider
    cl ⊛ ModelDiscovery
    cl ⊛ ModelScanner
    if ⊛ VaultKeyGetter
    fn ⊛ NewModelScanner(cfg *ModelScannerConfig, database db.Database, vault VaultKeyGetter) → *ModelScanner
      fn ⊛ ScanAll(ctx context.Context) → ([]*ModelDiscovery, error)
      fn ⊛ GetDiscoveries() → []*ModelDiscovery
      fn ⊛ GetNewModels() → []*ModelDiscovery
      fn ⊛ GetDisappearedModels() → []*ModelDiscovery
      fn ⊛ GetChangedModels() → []*ModelDiscovery
      fn ⊛ LastScan() → time.Time
      fn ⊛ StartBackgroundScanner(ctx context.Context)
      fn ⊛ PersistDiscoveries(ctx context.Context) → error
    fn min(a, b int) → int

## governor/internal/runtime/parallel.go
parallel.go [230L]
  deps: sync/atomic, context, log, fmt, sync
  exports: NewAgentPool, NewAgentPoolWithConcurrency
  API:
    cl ⊛ AgentPool
    fn ⊛ NewAgentPool(maxPerModule, maxTotal int) → *AgentPool
    fn ⊛ NewAgentPoolWithConcurrency(maxPerModule, maxTotal int, concurrency *ConcurrencyConfig) → *AgentPool
      fn ⊛ Submit(ctx context.Context, moduleID string, fn func() error) → error
      fn ⊛ SubmitWithDestination(ctx context.Context, moduleID, destination string, fn func() error) → error
      fn ⊛ Wait()
      fn ⊛ Errors() → <-chan error
      fn ⊛ DrainErrors() → []error
      fn ⊛ ActiveCount() → int
      fn ⊛ HasCapacity(moduleID, destination string) → bool
      fn ⊛ ModuleCount(moduleID string) → int
      fn ⊛ Stats() → map[string]interface{}

## governor/internal/runtime/platform_tracker.go
platform_tracker.go [361L]
  deps: sync, time, encoding/json, context, log
  exports: NewPlatformUsageTracker
  API:
    cl ⊛ PlatformProfile
    cl ⊛ PlatformUsageTracker
    fn ⊛ NewPlatformUsageTracker(bufferPct int) → *PlatformUsageTracker
      fn ⊛ RegisterPlatform(platformID string, limits PlatformLimitSchema)
      fn ⊛ RecordMessageSent(ctx context.Context, platformID string, tokensUsed int)
      fn ⊛ CanMakeRequest(ctx context.Context, platformID string, estimatedTokens int) → (canProceed bool, waitTime time.Duration)
      fn ⊛ NewSession(platformID string)
      fn ⊛ GetPlatformStatus(platformID string) → map[string]interface{}
      fn ⊛ HasPlatform(platformID string) → bool
    cl platformWindowsJSON
      fn ⊛ PersistToDatabase(ctx context.Context, db Querier)
      fn ⊛ LoadFromDatabase(ctx context.Context, db Querier)

## governor/internal/runtime/platform_tracker_v2.go
platform_tracker_v2.go [481L]
  deps: context, fmt, log, sync, time, encoding/json
  exports: NewPlatformUsageTrackerV2
  API:
    ty ⊛ LimitType
    ty ⊛ LimitWindow
    cl ⊛ PlatformLimit
    cl ⊛ PlatformProfileV2
    cl ⊛ LimitUsage
    cl ⊛ PlatformUsageTrackerV2
    fn ⊛ NewPlatformUsageTrackerV2() → *PlatformUsageTrackerV2
      fn ⊛ RegisterPlatformFromConfig(platformID string, limits []PlatformLimit)
      fn ⊛ RecordUsage(ctx context.Context, platformID string, tokensUsed int, costCents int)
      fn ⊛ CanMakeRequest(ctx context.Context, platformID string, estimatedTokens int) → (bool, time.Duration)
      fn ⊛ GetRemainingUsage(platformID string) → map[string]interface{}
      fn ⊛ UpdateCreditBalance(platformID string, remainingCents int)
      fn ⊛ HasPlatform(platformID string) → bool
      fn ⊛ AllPlatformStatus() → map[string]map[string]interface{}
      fn ⊛ PersistToDatabase(ctx context.Context, db Querier)
      fn ⊛ LoadFromDatabase(ctx context.Context, db Querier)

## governor/internal/runtime/research_action.go
research_action.go [561L]
  deps: path/filepath, log, context, github.com/vibepilot/governor/internal/db, os, sync, encoding/json, fmt
  exports: NewResearchActionApplier
  API:
    cl ⊛ ResearchActionApplier
    cl ⊛ ModelAction
    fn ⊛ NewResearchActionApplier(configDir string, database db.Database) → *ResearchActionApplier
      fn ⊛ ApplyResearchAction(ctx context.Context, suggestionType string, details map[string]interface{}) → (string, error)
    cl connectorsFile
    fn modelDataToProfile(data map[string]interface{}) → ModelProfile
    fn mapDataToConnector(data map[string]interface{}) → ConnectorConfig

## governor/internal/runtime/router.go
router.go [808L]
  deps: context, sort, sync/atomic, encoding/json, github.com/vibepilot/governor/internal/db, log
  exports: EstimateTokens, NewRouter
  API:
    fn ⊛ EstimateTokens(content string, role string) → int
    cl ⊛ Router
    fn ⊛ NewRouter(cfg *Config, database db.Database, usageTracker *UsageTracker) → *Router
    cl ⊛ RoutingRequest
    cl ⊛ RoutingResult
      fn ⊛ SelectRouting(ctx context.Context, req RoutingRequest) → (*RoutingResult, error)
    cl candidate
    cl scored
    cl ⊛ PlatformDestination
      fn ⊛ GetConnector(id string) → *ConnectorConfig
      fn ⊛ GetFallbackAction() → string
    cl ⊛ LegacyRoutingRequest
    cl ⊛ LegacyRoutingResult
      fn ⊛ SelectDestination(ctx context.Context, req LegacyRoutingRequest) → (*LegacyRoutingResult, error)
      fn ⊛ GetAvailableConnectors() → []string
      fn ⊛ GetAvailableModelCount() → int

## governor/internal/runtime/session.go
session.go [286L]
  deps: time, fmt, context, strings, encoding/json
  exports: WithTimeout, NewSession, NewSessionFactory
  API:
    if ⊛ ConnectorRunner
    if ⊛ HealthChecker
    cl ⊛ Session
    ty ⊛ SessionOption
    fn ⊛ WithTimeout(d time.Duration) → SessionOption
    fn ⊛ NewSession(id, agentID string, conn ConnectorRunner, connID, prompt string, opts ...SessionOption) → *Session
    cl ⊛ SessionResult
      fn ⊛ Run(ctx context.Context, input map[string]any) → (*SessionResult, error)
      fn ⊛ Compact(ctx context.Context, result *SessionResult, taskID string)
    cl ⊛ SessionFactory
    if ⊛ SessionCompactor
    fn ⊛ NewSessionFactory(cfg *Config) → *SessionFactory
      fn ⊛ SetContextBuilder(cb *ContextBuilder)
      fn ⊛ SetCompactor(c SessionCompactor)
      fn ⊛ RegisterConnector(id string, runner ConnectorRunner)
      fn ⊛ GetConnector(id string) → (ConnectorRunner, bool)
      fn ⊛ GetConnectorConfig(id string) → *ConnectorConfig
      fn ⊛ Create(agentID string, opts ...SessionOption) → (*Session, error)
      fn ⊛ CreateWithContext(ctx context.Context, agentID string, taskType string, opts ...SessionOption) → (*Session, error)
      fn ⊛ CreateWithConnector(ctx context.Context, agentID string, taskType string, connectorID string, opts ...SessionOption) → (*Session, error)

## governor/internal/runtime/tools.go
tools.go [136L]
  deps: encoding/json, context, fmt
  exports: NewToolRegistry
  API:
    cl ⊛ ToolResult
    if ⊛ ToolExecutor
    cl ⊛ ToolRegistry
    fn ⊛ NewToolRegistry(cfg *Config) → *ToolRegistry
      fn ⊛ Register(name string, executor ToolExecutor)
      fn ⊛ HasTool(name string) → bool
      fn ⊛ ListTools() → []string
      fn ⊛ Execute(ctx context.Context, toolName string, args map[string]any) → ToolResult
    fn validateToolArgs(tool *ToolConfig, args map[string]any) → error
    fn validateParamType(name, expectedType string, value any) → error

## governor/internal/runtime/usage_tracker.go
usage_tracker.go [1004L]
  deps: time, context, encoding/json, fmt, log, sync
  exports: NewUsageTracker
  API:
    ty ⊛ ThrottleBehavior
    cl ⊛ RateLimits
    cl ⊛ RecoveryConfig
    cl ⊛ LearnedData
    cl ⊛ APIPricing
    cl ⊛ ModelProfile
    cl ⊛ UsageWindow
    cl ⊛ UsageWindows
    cl ⊛ ModelUsage
    cl ⊛ UsageTracker
    fn ⊛ NewUsageTracker(db Querier) → *UsageTracker
      fn ⊛ SetDefaults(defaults ModelProfile)
      fn ⊛ RegisterConnectorSharedLimits(connectorID string, sharedLimits RateLimits)
      fn ⊛ RegisterPlatformLimits(platformID string, limits PlatformLimitSchema)
      fn ⊛ RegisterPlatformLimitsV2(platformID string, limits []PlatformLimit)
      fn ⊛ GetPlatformTrackerV2() → *PlatformUsageTrackerV2
      fn ⊛ PlatformCanMakeRequest(ctx context.Context, platformID string, estimatedTokens int) → (bool, time.Duration)
      fn ⊛ RecordPlatformMessage(ctx context.Context, platformID string, tokensUsed int)
      fn ⊛ RecordPlatformMessageWithCost(ctx context.Context, platformID, modelID string, tokensIn, tokensOut int)
      fn ⊛ RecordPlatformUsage(ctx context.Context, platformID string, tokensUsed int, costCents int)
      fn ⊛ UpdatePlatformCredit(platformID string, remainingCents int)
      fn ⊛ RecordPlatformMessageOld(ctx context.Context, platformID string, tokensUsed int)
      fn ⊛ GetPlatformTracker() → *PlatformUsageTracker
      fn ⊛ RegisterModel(profile ModelProfile)
    cl ⊛ RequestDecision
      fn ⊛ CanMakeRequest(ctx context.Context, modelID string, estimatedTokens int) → RequestDecision
      fn ⊛ CanMakeRequestVia(ctx context.Context, modelID string, connectorID string, estimatedTokens int) → RequestDecision
      fn ⊛ RecordUsage(ctx context.Context, modelID string, tokensIn, tokensOut int) → error
      fn ⊛ RecordRateLimit(ctx context.Context, modelID string) → error
      fn ⊛ RecordConnectorCooldown(ctx context.Context, connectorID string, cooldownMins int)
      fn ⊛ RecordCompletion(ctx context.Context, modelID string, taskType string, durationSeconds float64, success bool) → error
      fn ⊛ GetModelStatus(modelID string) → (map[string]interface{}, error)
      fn ⊛ GetCooldownCountdown(modelID string) → (int, error)
      fn ⊛ ExportForDashboard() → ([]byte, error)
      fn ⊛ LoadFromDatabase(ctx context.Context) → error
      fn ⊛ PersistToDatabase(ctx context.Context)
      fn ⊛ SetCredit(ctx context.Context, modelID string, totalUSD float64) → error
      fn ⊛ GetMinuteRequestCount(ctx context.Context, modelID string) → int
      fn ⊛ GetModelLearnedScore(modelID string, taskType string) → float64
      fn ⊛ GetModelCooldownMinutes(modelID string) → int
    fn parseTime(s string) → (time.Time, error)

## governor/internal/security/leak_detector.go
leak_detector.go [69L]
  deps: log, regexp
  exports: NewLeakDetector
  API:
    cl ⊛ LeakWarning
    cl leakPattern
    cl ⊛ LeakDetector
    fn ⊛ NewLeakDetector() → *LeakDetector
      fn ⊛ Scan(output string) → (string, []LeakWarning)
    fn maskSecret(s string) → string

## governor/internal/tools/db_tools.go
db_tools.go [238L]
  deps: context, github.com/vibepilot/governor/internal/db, fmt, regexp, encoding/json
  exports: NewDBQueryTool, NewDBUpdateTool, NewDBInsertTool, NewDBRPCTool, NewMaintenanceCommandTool
  API:
    fn sanitizeFilterValue(val interface{}) → string
    fn sanitizeColumnName(name string) → string
    cl ⊛ DBQueryTool
    fn ⊛ NewDBQueryTool(database db.Database) → *DBQueryTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ DBUpdateTool
    fn ⊛ NewDBUpdateTool(database db.Database) → *DBUpdateTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ DBInsertTool
    fn ⊛ NewDBInsertTool(database db.Database) → *DBInsertTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ DBRPCTool
    fn ⊛ NewDBRPCTool(database db.Database) → *DBRPCTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ MaintenanceCommandTool
    fn ⊛ NewMaintenanceCommandTool(database db.Database) → *MaintenanceCommandTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/file_tools.go
file_tools.go [177L]
  deps: encoding/json, path/filepath, strings, context, os, fmt
  exports: NewFileReadTool, NewFileWriteTool, NewFileDeleteTool
  API:
    cl ⊛ FileReadTool
    fn ⊛ NewFileReadTool(repoPath string) → *FileReadTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ FileWriteTool
    fn ⊛ NewFileWriteTool(repoPath string) → *FileWriteTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ FileDeleteTool
    fn ⊛ NewFileDeleteTool(repoPath string) → *FileDeleteTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/git_tools.go
git_tools.go [185L]
  deps: context, encoding/json, github.com/vibepilot/governor/internal/gitree, fmt
  exports: NewGitCreateBranchTool, NewGitReadBranchTool, NewGitCommitTool, NewGitMergeTool, NewGitDeleteBranchTool, NewGitClearBranchTool
  API:
    cl ⊛ GitCreateBranchTool
    fn ⊛ NewGitCreateBranchTool(git *gitree.Gitree) → *GitCreateBranchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ GitReadBranchTool
    fn ⊛ NewGitReadBranchTool(git *gitree.Gitree) → *GitReadBranchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ GitCommitTool
    fn ⊛ NewGitCommitTool(git *gitree.Gitree) → *GitCommitTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ GitMergeTool
    fn ⊛ NewGitMergeTool(git *gitree.Gitree) → *GitMergeTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ GitDeleteBranchTool
    fn ⊛ NewGitDeleteBranchTool(git *gitree.Gitree) → *GitDeleteBranchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ GitClearBranchTool
    fn ⊛ NewGitClearBranchTool(git *gitree.Gitree) → *GitClearBranchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/registry.go
registry.go [94L]
  deps: github.com/vibepilot/governor/internal/runtime, github.com/vibepilot/governor/internal/db, net/http, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/vault, time
  exports: RegisterAll
  API:
    cl ⊛ Dependencies
    fn init()
    fn ⊛ RegisterAll(registry *runtime.ToolRegistry, deps *Dependencies)

## governor/internal/tools/sandbox_tools.go
sandbox_tools.go [245L]
  deps: bytes, path/filepath, time, context, encoding/json, strings, fmt, os, os/exec
  exports: NewSandboxTestTool, NewSandboxTestToolWithConfig, NewRunLintTool, NewRunLintToolWithTimeout, NewRunTypecheckTool, NewRunTypecheckToolWithTimeout
  API:
    cl ⊛ SandboxTestTool
    cl ⊛ SandboxConfig
    fn ⊛ NewSandboxTestTool(repoPath string, timeoutSecs int) → *SandboxTestTool
    fn ⊛ NewSandboxTestToolWithConfig(cfg *SandboxConfig) → *SandboxTestTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ RunLintTool
    fn ⊛ NewRunLintTool(repoPath string) → *RunLintTool
    fn ⊛ NewRunLintToolWithTimeout(repoPath string, timeoutSecs int) → *RunLintTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ RunTypecheckTool
    fn ⊛ NewRunTypecheckTool(repoPath string) → *RunTypecheckTool
    fn ⊛ NewRunTypecheckToolWithTimeout(repoPath string, timeoutSecs int) → *RunTypecheckTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/vault_tools.go
vault_tools.go [41L]
  deps: fmt, context, encoding/json, github.com/vibepilot/governor/internal/vault
  exports: NewVaultGetTool
  API:
    cl ⊛ VaultGetTool
    fn ⊛ NewVaultGetTool(v *vault.Vault) → *VaultGetTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/web_tools.go
web_tools.go [231L]
  deps: fmt, net/http, context, io, encoding/json, github.com/vibepilot/governor/internal/runtime, strings, net/url
  exports: NewWebSearchTool, NewWebFetchTool
  API:
    cl ⊛ WebSearchTool
    fn ⊛ NewWebSearchTool(allowlist []string, config *runtime.WebToolsConfig) → *WebSearchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ WebFetchTool
    fn ⊛ NewWebFetchTool(allowlist []string, config *runtime.WebToolsConfig) → *WebFetchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/vault/vault.go
vault.go [506L]
  deps: context, sync, time, encoding/base64, crypto/cipher, crypto/rand, encoding/json, io, os, crypto/aes, github.com/vibepilot/governor/internal/db, log, golang.org/x/crypto/pbkdf2, crypto/sha256, fmt
  exports: New, NewWithoutAudit, Encrypt, GetEnvOrVault
  API:
    cl ⊛ Vault
    cl cachedSecret
    cl ⊛ AuditEntry
    cl ⊛ SecretRecord
    fn ⊛ New(database db.Database) → *Vault
    fn ⊛ NewWithoutAudit(database db.Database) → *Vault
      fn ⊛ InitVaultKey(keyEnv string)
      fn ⊛ SetVaultKeyDirect(key string)
    fn getMachineSalt() → []byte
      fn ⊛ GetSecret(ctx context.Context, keyName string) → (string, error)
      fn ⊛ GetSecretNoCache(ctx context.Context, keyName string) → (string, error)
    fn ⊛ Encrypt(plaintext, masterKey string) → (string, error)
    fn deriveKey(password string, salt []byte) → []byte
      fn ⊛ InvalidateCache(keyName string)
      fn ⊛ InvalidateAll()
      fn ⊛ CacheStats() → map[string]interface{}
      fn ⊛ StoreSecret(ctx context.Context, keyName, plaintext string) → error
      fn ⊛ ListSecrets(ctx context.Context) → ([]string, error)
      fn ⊛ RotateKey(ctx context.Context, newMasterKey string) → (int, error)
      fn ⊛ DeleteSecret(ctx context.Context, keyName string) → error
    fn ⊛ GetEnvOrVault(ctx context.Context, v *Vault, keyName string) → string

## governor/internal/visualqa/audit.go
audit.go [484L]
  deps: encoding/base64, fmt, strings, context, encoding/json, os, time
  API:
    cl ⊛ AuditResult
    cl ⊛ AuditIssue
    fn mergeAuditIssues(visionIssues []AuditIssue, domIssues []AuditIssue) → []AuditIssue
    fn worstSeverity(issues []AuditIssue) → string
    fn domAuditScript(viewport int) → string

## governor/internal/visualqa/baseline.go
baseline.go [164L]
  deps: bytes, context, encoding/json, os/exec, time, fmt, os, path/filepath
  API:
    cl ⊛ BaselineEntry
    ty ⊛ BaselineManifest
      fn ⊛ GetBaselinePath(pageName string, viewportWidth int) → string
      fn ⊛ LoadManifest() → (BaselineManifest, error)
      fn ⊛ SaveManifest(manifest BaselineManifest) → error
      fn ⊛ SaveBaseline(ctx context.Context, pageName string, viewportWidth int, capturePath string) → error
      fn ⊛ ApproveBaseline(ctx context.Context, pageName string, viewportWidth int) → error

## governor/internal/visualqa/capture.go
capture.go [513L]
  deps: os, time, path/filepath, context, fmt, os/exec, strconv, encoding/json
  API:
    cl ⊛ CaptureResult
    cl ⊛ UIIssue
    cl ⊛ UICaptureReport

## governor/internal/visualqa/compare.go
compare.go [450L]
  deps: encoding/base64, fmt, bytes, context, os, strings, net/http, time, encoding/json, io
  API:
    cl ⊛ VisionProvider
    cl ⊛ VisionProviderState
    cl ⊛ GeminiCompareRequest
    cl ⊛ GeminiContent
    cl ⊛ GeminiPart
    cl ⊛ GeminiInlineData
    cl ⊛ GeminiCompareResponse
    cl ⊛ ORMessage
    cl ⊛ ORContent
    cl ⊛ ORImageURL
    cl ⊛ ORRequest
    cl ⊛ ORResponse
    cl visionCallResult
    fn extractJSONFromText(text string) → string
    fn indexOf(s, substr string, fromIdx ...int) → int

## governor/internal/visualqa/db.go
db.go [78L]
  deps: github.com/jackc/pgx/v5, github.com/jackc/pgx/v5/pgxpool, context, database/sql, fmt, os
  exports: NewPGXDB, NewPGXDBFromPool
  API:
    cl ⊛ PGXDB
    fn ⊛ NewPGXDB(ctx context.Context, connString string) → (*PGXDB, error)
    fn ⊛ NewPGXDBFromPool(pool *pgxpool.Pool) → *PGXDB
      fn ⊛ Close()
      fn ⊛ Exec(ctx context.Context, query string, args ...interface{}) → (interface{}, error)
      fn ⊛ Query(ctx context.Context, query string, args ...interface{}) → (pgx.Rows, error)
      fn ⊛ SQLDB() → *sql.DB
    fn fileExists(path string) → bool
    fn ensureDir(path string) → error
    fn removeDir(path string) → error

## governor/internal/visualqa/fixengine.go
fixengine.go [437L]
  deps: time, context, fmt, os/exec, os, regexp, strings, path/filepath, encoding/json
  exports: NewFixEngine
  API:
    if ⊛ FixStrategy
    cl ⊛ FixSuggestion
    cl ⊛ FixResult
    cl ⊛ FixEngine
    cl ⊛ FixConfig
    fn ⊛ NewFixEngine(config FixConfig) → *FixEngine
      fn ⊛ RegisterStrategy(s FixStrategy)
      fn ⊛ ProcessIssues(ctx context.Context, issues []UIIssue, screenshotPath string) → ([]FixSuggestion, error)
      fn ⊛ ApplyFixes(ctx context.Context, suggestions []FixSuggestion) → []FixResult
    cl ⊛ ConsoleErrorStrategy
      fn ⊛ CanFix(issue UIIssue) → bool
      fn ⊛ Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) → (*FixSuggestion, error)
      fn ⊛ Apply(ctx context.Context, suggestion *FixSuggestion) → (*FixResult, error)
    cl ⊛ LayoutBreakpointStrategy
      fn ⊛ CanFix(issue UIIssue) → bool
      fn ⊛ Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) → (*FixSuggestion, error)
      fn ⊛ Apply(ctx context.Context, suggestion *FixSuggestion) → (*FixResult, error)
    cl ⊛ VisualAnomalyStrategy
      fn ⊛ CanFix(issue UIIssue) → bool
      fn ⊛ Analyze(ctx context.Context, issue UIIssue, sourceRoot string, screenshotPath string) → (*FixSuggestion, error)
      fn ⊛ Apply(ctx context.Context, suggestion *FixSuggestion) → (*FixResult, error)
    fn extractURL(text string) → string
    fn findInSource(sourceRoot, searchStr string) → (string, int)
    fn findJSErrorSource(sourceRoot, errorMsg string) → (string, int)
    fn findCSSFiles(sourceRoot string) → []string
    fn extractMediaQueryBreakpoints(cssFilePath string) → []int

## governor/internal/visualqa/visualqa.go
visualqa.go [666L]
  deps: time, context, github.com/google/uuid, github.com/jackc/pgx/v5, encoding/json, fmt, strings, regexp
  exports: NewVisualQA
  API:
    cl ⊛ Config
    cl ⊛ InteractionDiscovery
    cl ⊛ PageConfig
    cl ⊛ ComparisonResult
    cl ⊛ Difference
    cl ⊛ RunResult
    cl ⊛ PageResult
    if ⊛ DB
    cl ⊛ VisualQA
    fn ⊛ NewVisualQA(config Config, apiKey string, db DB) → *VisualQA
      fn ⊛ Run(ctx context.Context, triggeredBy, triggerDetail string) → (RunResult, error)
      fn ⊛ GetIssuesForRun(ctx context.Context, runID string) → ([]map[string]any, error)
      fn ⊛ GetEnabled() → bool
    fn issuePatternKey(issueType, element string) → string
    fn filterKnownFalsePositives(issues []AuditIssue, falsePositives map[string]bool) → []AuditIssue
      fn ⊛ RecordIssueFeedback(ctx context.Context, runID, issueType, issueElement, issueDescription string, viewport int, verdict, userNote, patternKey string) → error
      fn ⊛ GetIssueFeedback(ctx context.Context) → ([]map[string]any, error)
      fn ⊛ GetFalsePositivePatterns(ctx context.Context) → map[string]bool
    fn dedupIssues(issues []UIIssue) → []UIIssue
    fn normalizeForDedup(desc string) → string

## governor/internal/webhooks/chat_usage.go
chat_usage.go [70L]
  deps: io, log, net/http, encoding/json
  API:
    cl chatUsageRequest

## governor/internal/webhooks/designpreview.go
designpreview.go [204L]
  deps: log, encoding/json, net/http, fmt, context
  API:
    if ⊛ DesignPreviewer
    cl ⊛ DesignPreviewRequest
    cl ⊛ DesignPreviewResponse
      fn ⊛ SetDesignPreview(previewer DesignPreviewer)

## governor/internal/webhooks/github.go
github.go [293L]
  deps: encoding/json, path/filepath, strings, github.com/vibepilot/governor/internal/db, log, context
  exports: NewGitHubWebhookHandler
  API:
    cl ⊛ GitHubWebhookHandler
    cl ⊛ GitHubPushPayload
    cl ⊛ GitHubCommit
    fn ⊛ NewGitHubWebhookHandler(database db.Database, prdDir string) → *GitHubWebhookHandler
      fn ⊛ HandlePush(ctx context.Context, body []byte)

## governor/internal/webhooks/server.go
server.go [2645L]
  deps: log, strconv, github.com/vibepilot/governor/internal/runtime, strings, crypto/sha256, os, crypto/hmac, encoding/json, time, fmt, os/exec, context, path/filepath, net/http, io, encoding/hex, github.com/vibepilot/governor/internal/reviewitems
  exports: NewServer, GetWebhookURL
  API:
    ty ⊛ CourierResultFunc
    if ⊛ VaultManager
    cl ⊛ Server
    if ⊛ DBQuerier
    ty ⊛ EventHandler
    cl ⊛ Config
    cl ⊛ Payload
    fn ⊛ NewServer(cfg *Config, router *runtime.EventRouter) → *Server
      fn ⊛ SetGitHubHandler(handler *GitHubWebhookHandler)
      fn ⊛ SetDB(db DBQuerier)
      fn ⊛ SetSSEBroker(broker *SSEBroker)
      fn ⊛ SetCourierResultFn(fn CourierResultFunc)
      fn ⊛ SetVault(v VaultManager)
      fn ⊛ SetAdminToken(token string)
      fn ⊛ SetConfigDir(dir string)
    if ⊛ CreditTracker
      fn ⊛ SetCreditTracker(tracker CreditTracker)
      fn ⊛ RegisterHandler(eventType string, handler EventHandler)
      fn ⊛ Start(ctx context.Context) → error
      fn ⊛ Shutdown(ctx context.Context) → error
    fn extractID(record map[string]any) → string
    cl tableResult
      fn ⊛ GetPort() → int
      fn ⊛ GetSSEBroker() → *SSEBroker
      fn ⊛ SetWSUpgrader(upgrader any)
      fn ⊛ SetWSPath(path string)
      fn ⊛ IsRunning() → bool
    fn ⊛ GetWebhookURL(baseURL string, port int, path string) → string
    cl compatAlert
    cl responseItem
    cl sysEntry
    fn splitLines(s string) → []string
    fn splitFields(s string) → []string
    fn unwrapRPCResult(data []byte, funcName string) → []byte

## governor/internal/webhooks/sse.go
sse.go [79L]
  deps: log, encoding/json, sync
  exports: NewSSEBroker
  API:
    cl ⊛ SSENotification
    cl ⊛ SSEBroker
    fn ⊛ NewSSEBroker() → *SSEBroker
      fn ⊛ Broadcast(table, action, id string)
      fn ⊛ Subscribe() → chan SSENotification
      fn ⊛ Unsubscribe(ch chan SSENotification)

## governor/internal/webhooks/visualqa.go
visualqa.go [216L]
  deps: time, fmt, encoding/json, context, net/http, io, log
  API:
    if ⊛ VisualQARunner
      fn ⊛ SetVisualQA(runner VisualQARunner)
    fn writeJSON(w http.ResponseWriter, statusCode int, data any)

## governor/pkg/types/types.go
types.go [121L]
  deps: time
  API:
    ty ⊛ TaskStatus
    ty ⊛ RoutingFlag
    cl ⊛ Task
    cl ⊛ PromptPacket
    cl ⊛ Constraints
    cl ⊛ TaskRun
    cl ⊛ Model
    cl ⊛ Platform
    cl ⊛ DispatchResult


