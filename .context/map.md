# VibePilot Code Map
# Generated: 2026-04-21T04:11:07Z | Commit: e5e22005
# Auto-generated. Run build.sh to regenerate.

## governor/cmd/cleanup/main.go
main.go [65L]
  deps: os/signal, fmt, log, github.com/vibepilot/governor/internal/db, time, context, syscall, os
  API:
    fn main()

## governor/cmd/encrypt_secret/main.go
main.go [26L]
  deps: os, fmt, github.com/vibepilot/governor/internal/vault
  API:
    fn main()

## governor/cmd/governor/adapters.go
adapters.go [36L]
  deps: context, github.com/vibepilot/governor/internal/core, github.com/vibepilot/governor/internal/db, encoding/json
  API:
    cl dbCheckpointAdapter
      fn ⊛ RPC(ctx context.Context, fn string, args map[string]any) → (json.RawMessage, error)
      fn ⊛ Save(ctx context.Context, taskID string, checkpoint core.Checkpoint) → error
      fn ⊛ Load(ctx context.Context, taskID string) → (*core.Checkpoint, error)
      fn ⊛ Delete(ctx context.Context, taskID string) → error

## governor/cmd/governor/handlers_council.go
handlers_council.go [582L]
  deps: log, context, github.com/vibepilot/governor/internal/runtime, time, fmt, github.com/vibepilot/governor/internal/db, errors, github.com/vibepilot/governor/internal/gitree, encoding/json, os, sync
  exports: NewCouncilHandler
  API:
    cl ⊛ CouncilHandler
    fn ⊛ NewCouncilHandler(database *db.DB, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, cfg *runtime.Config, git *gitree.Gitree, usageTracker *runtime.UsageTracker, ) → *CouncilHandler
      fn ⊛ Register(router *runtime.EventRouter)
    fn mapStrAny(m map[string]interface{}) → map[string]any
    fn setupCouncilHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, )

## governor/cmd/governor/handlers_maint.go
handlers_maint.go [337L]
  deps: github.com/vibepilot/governor/internal/gitree, fmt, github.com/vibepilot/governor/internal/runtime, github.com/vibepilot/governor/internal/db, log, context, encoding/json, time
  exports: NewMaintenanceHandler
  API:
    cl ⊛ MaintenanceHandler
    fn ⊛ NewMaintenanceHandler(database *db.DB, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, cfg *runtime.Config, git *gitree.Gitree, usageTracker *runtime.UsageTracker, ) → *MaintenanceHandler
      fn ⊛ Register(router *runtime.EventRouter)
    fn setupMaintenanceHandler(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, )

## governor/cmd/governor/handlers_plan.go
handlers_plan.go [576L]
  deps: github.com/vibepilot/governor/internal/db, context, os, time, encoding/json, log, fmt, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/runtime, path/filepath
  API:
    fn setupPlanHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, )
    fn handlePlanCreated(ctx context.Context, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, event runtime.Event, )
    fn runPlanReview(ctx context.Context, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, plan map[string]any, )
    fn handlePlanReview(ctx context.Context, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, usageTracker *runtime.UsageTracker, event runtime.Event, )
    fn setPlanError(ctx context.Context, database *db.DB, planID string, reason string)

## governor/cmd/governor/handlers_research.go
handlers_research.go [446L]
  deps: sync, time, github.com/vibepilot/governor/internal/runtime, encoding/json, log, github.com/vibepilot/governor/internal/db, fmt, context
  exports: NewResearchHandler
  API:
    cl ⊛ ResearchHandler
    fn ⊛ NewResearchHandler(database *db.DB, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, cfg *runtime.Config, usageTracker *runtime.UsageTracker, ) → *ResearchHandler
      fn ⊛ Register(router *runtime.EventRouter)
    fn setupResearchHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, usageTracker *runtime.UsageTracker, )

## governor/cmd/governor/handlers_task.go
handlers_task.go [1317L]
  deps: github.com/vibepilot/governor/internal/runtime, encoding/json, os, context, github.com/vibepilot/governor/internal/gitree, time, fmt, github.com/vibepilot/governor/internal/connectors, log, strings, github.com/vibepilot/governor/internal/core, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/security
  exports: NewTaskHandler
  API:
    cl ⊛ TaskHandler
    if vaultProvider
    fn ⊛ NewTaskHandler(database *db.DB, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, git *gitree.Gitree, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, cfg *runtime.Config, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v vaultProvider, ) → *TaskHandler
      fn ⊛ Register(router *runtime.EventRouter)
    cl costResult
    fn isRateLimitError(err error) → bool
    fn setupTaskHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v vaultProvider, )
    fn unlockDependentsByTaskNumber(ctx context.Context, database *db.DB, completedTaskNumber string)

## governor/cmd/governor/handlers_testing.go
handlers_testing.go [604L]
  deps: strings, time, path/filepath, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/runtime, context, github.com/vibepilot/governor/internal/db, log, fmt, os/exec, encoding/json, os, bytes
  exports: NewTestingHandler
  API:
    cl ⊛ TestingHandler
    fn ⊛ NewTestingHandler(database *db.DB, factory *runtime.SessionFactory, pool *runtime.AgentPool, connRouter *runtime.Router, git *gitree.Gitree, cfg *runtime.Config, worktreeMgr *gitree.WorktreeManager, usageTracker *runtime.UsageTracker, ) → *TestingHandler
      fn ⊛ Register(router *runtime.EventRouter)
    fn setupTestingHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, connRouter *runtime.Router, git *gitree.Gitree, worktreeMgr *gitree.WorktreeManager, usageTracker *runtime.UsageTracker, )
    fn filesToPackagePaths(files []any) → []string

## governor/cmd/governor/helpers.go
helpers.go [100L]
  deps: encoding/json, github.com/vibepilot/governor/internal/db, log, context
  API:
    fn getString(m map[string]any, key string) → string
    fn getStringOr(m map[string]any, key, def string) → string
    fn parseBool(data []byte) → bool
    fn truncateID(id string) → string
    fn truncateOutput(output string) → string
    fn extractCouncilReviews(plan map[string]any) → []map[string]any
    fn recordModelSuccess(ctx context.Context, database *db.DB, modelID, taskType string, durationSeconds float64)
    fn recordModelFailure(ctx context.Context, database *db.DB, modelID, taskID, failureType string)

## governor/cmd/governor/main.go
main.go [392L]
  deps: github.com/vibepilot/governor/internal/memory, os, github.com/vibepilot/governor/internal/tools, os/signal, github.com/vibepilot/governor/internal/dag, log, time, github.com/vibepilot/governor/internal/runtime, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/mcp, github.com/vibepilot/governor/internal/security, github.com/vibepilot/governor/internal/db, syscall, github.com/vibepilot/governor/internal/webhooks, encoding/json, path/filepath, github.com/vibepilot/governor/internal/connectors, github.com/vibepilot/governor/internal/realtime, context, github.com/vibepilot/governor/internal/core, github.com/vibepilot/governor/internal/vault
  API:
    fn main()
    fn getConfigDir() → string
    fn getEnvOrDefault(key, defaultVal string) → string
    fn registerConnectors(factory *runtime.SessionFactory, cfg *runtime.Config, v *vault.Vault, repoPath string)
    fn setupEventHandlers(ctx context.Context, router *runtime.EventRouter, factory *runtime.SessionFactory, pool *runtime.AgentPool, database *db.DB, cfg *runtime.Config, toolRegistry *runtime.ToolRegistry, connRouter *runtime.Router, git *gitree.Gitree, stateMachine *core.StateMachine, checkpointMgr *core.CheckpointManager, leakDetector *security.LeakDetector, usageTracker *runtime.UsageTracker, worktreeMgr *gitree.WorktreeManager, courierRunner *connectors.CourierRunner, v *vault.Vault)

## governor/cmd/governor/recovery.go
recovery.go [384L]
  deps: encoding/json, log, fmt, context, github.com/vibepilot/governor/internal/core, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/runtime, time
  API:
    fn getRecoveryConfig(cfg *runtime.Config) → RecoveryConfig
    fn runStartupRecovery(ctx context.Context, database *db.DB, cfg RecoveryConfig)
    fn runProcessingRecovery(ctx context.Context, database *db.DB, cfg *runtime.Config)
    fn recoverStaleProcessing(ctx context.Context, database *db.DB, table string, timeout int)
    fn runCheckpointRecovery(ctx context.Context, database *db.DB, cfg *runtime.Config, checkpointMgr *core.CheckpointManager)
    fn recoverPendingResources(ctx context.Context, database *db.DB)
    fn runStartupRehydration(ctx context.Context, database *db.DB, router *runtime.EventRouter)

## governor/cmd/governor/smoke.go
smoke.go [252L]
  deps: time, io, fmt, bytes, encoding/json, log, net/http, os
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
startup_validate.go [503L]
  deps: fmt, os/exec, os, strings, context, path/filepath, encoding/json, time, log
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
validation.go [395L]
  deps: regexp, github.com/vibepilot/governor/internal/gitree, context, os, path/filepath, encoding/json, log, strings, fmt, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/runtime, strconv
  API:
    cl ⊛ TaskData
    cl ⊛ ValidationError
      fn ⊛ Error() → string
    cl ⊛ ValidationFailedError
      fn ⊛ Error() → string
    fn validateTasks(tasks []TaskData, cfg *runtime.ValidationConfig) → *ValidationFailedError
    fn createTasksFromApprovedPlan(ctx context.Context, database *db.DB, plan map[string]any, cfg *runtime.ValidationConfig, repoPath string, git *gitree.Gitree) → error
    fn parseTasksFromPlanMarkdown(content string) → ([]TaskData, error)
    fn parseTaskSection(section string) → (TaskData, error)
    fn extractSection(body, heading string) → string

## governor/cmd/migrate_vault/main.go
main.go [197L]
  deps: golang.org/x/crypto/pbkdf2, crypto/rand, crypto/cipher, crypto/sha256, fmt, encoding/json, encoding/base64, crypto/aes, bytes, io, net/http, log, os
  API:
    fn main()
    cl ⊛ Secret
    fn fetchSecrets(baseURL, serviceKey string) → ([]Secret, error)
    fn updateSecret(baseURL, serviceKey, keyName, encryptedValue string) → error
    fn decryptOld(encrypted, masterKey string) → (string, error)
    fn encryptNew(plaintext, masterKey string) → (string, error)

## governor/internal/connectors/courier.go
courier.go [258L]
  deps: bytes, encoding/json, sync, time, context, fmt, io, net/http
  exports: NewCourierRunner
  API:
    if ⊛ CourierDB
    cl courierWaiter
    cl ⊛ CourierRunner
    fn ⊛ NewCourierRunner(githubToken, githubRepo string, db CourierDB, timeoutSecs int) → *CourierRunner
      fn ⊛ Run(ctx context.Context, prompt string, timeout int) → (string, int, int, error)
      fn ⊛ NotifyResult(taskID string, result *TaskRunResult)
    cl ⊛ TaskRunResult
    fn min(a, b int) → int

## governor/internal/connectors/runners.go
runners.go [497L]
  deps: context, io, strings, bufio, encoding/json, net/http, os/exec, github.com/vibepilot/governor/internal/runtime, github.com/vibepilot/governor/internal/vault, bytes, time, fmt
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
    fn parseGeminiResponse(body []byte) → (string, int, int)
    fn parseOpenAIResponse(body []byte) → (string, int, int)
    cl ⊛ VaultAdapter
    fn ⊛ NewVaultAdapter(v *vault.Vault) → *VaultAdapter
      fn ⊛ GetSecret(ctx context.Context, keyName string) → (string, error)

## governor/internal/core/analyst.go
analyst.go [116L]
  deps: context, fmt, time, encoding/json
  exports: NewAnalyst
  API:
    cl ⊛ Analyst
    if ⊛ DBInterface
    cl ⊛ AnalysisResult
    fn ⊛ NewAnalyst(sm *StateMachine, db DBInterface, checkpointMgr *CheckpointManager) → *Analyst
      fn ⊛ RunDailyAnalysis(ctx context.Context) → (*AnalysisResult, error)

## governor/internal/core/checkpoint.go
checkpoint.go [143L]
  deps: context, encoding/json, time, fmt
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
state.go [302L]
  deps: time, fmt, sync, encoding/json, context
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
  deps: encoding/json, path/filepath, os/exec, os, strings, time, context, fmt
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
  deps: fmt, strings, time, log, context, sync
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
  deps: os, path/filepath, fmt, sync
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

## governor/internal/db/rpc.go
rpc.go [229L]
  deps: encoding/json, fmt, sync, context
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
  deps: context, fmt, time, encoding/json
  API:
      fn ⊛ RecordStateTransition(ctx context.Context, entityType string, entityID string, fromState string, toState string, reason string, metadata map[string]any) → error
      fn ⊛ RecordPerformanceMetric(ctx context.Context, metricType string, entityID string, duration time.Duration, success bool, metadata map[string]any) → error
      fn ⊛ GetLatestState(ctx context.Context, entityType string, entityID string) → (toState string, reason string, createdAt time.Time, err error)
      fn ⊛ ClearProcessingAndRecordTransition(ctx context.Context, table string, id string, fromState string, toState string, reason string) → error

## governor/internal/db/supabase.go
supabase.go [285L]
  deps: regexp, time, io, net/http, strings, encoding/json, bytes, net/url, context, fmt
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

## governor/internal/gitree/gitree.go
gitree.go [499L]
  deps: fmt, encoding/json, regexp, bytes, os/exec, context, time, log, os, path/filepath, strings
  exports: New
  API:
    fn isValidBranchName(name string) → bool
    cl ⊛ Gitree
    cl ⊛ Config
    fn ⊛ New(cfg *Config) → *Gitree
      fn ⊛ CreateBranch(ctx context.Context, branchName string) → error
      fn ⊛ CreateBranchFrom(ctx context.Context, branchName, sourceBranch string) → error
      fn ⊛ CommitOutput(ctx context.Context, branchName string, output interface{}) → error
      fn ⊛ ReadBranchOutput(ctx context.Context, branchName string) → ([]string, error)
      fn ⊛ MergeBranch(ctx context.Context, sourceBranch, targetBranch string) → error
      fn ⊛ DeleteBranch(ctx context.Context, branchName string) → error
      fn ⊛ ClearBranch(ctx context.Context, branchName string) → error
      fn ⊛ CreateModuleBranch(ctx context.Context, sliceID string) → error
      fn ⊛ CommitAndPush(ctx context.Context, filePath, message string) → error
      fn ⊛ Pull(ctx context.Context) → error

## governor/internal/gitree/worktree.go
worktree.go [411L]
  deps: log, regexp, strings, path/filepath, context, os, fmt, time
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

## governor/internal/hello/hello.go
hello.go [6L]
  exports: Greet
  API:
    fn ⊛ Greet(name string) → string

## governor/internal/maintenance/maintenance.go
maintenance.go [346L]
  deps: os, strings, github.com/vibepilot/governor/pkg/types, github.com/vibepilot/governor/internal/gitree, log, fmt, context, path/filepath, github.com/vibepilot/governor/internal/db
  exports: New
  API:
    ty ⊛ RiskLevel
    ty ⊛ ChangeType
    cl ⊛ Change
    cl ⊛ ExecutionResult
    cl ⊛ Maintenance
    cl ⊛ Config
    fn ⊛ New(cfg *Config, database *db.DB, git *gitree.Gitree) → *Maintenance
      fn ⊛ ClassifyRisk(change *Change) → RiskLevel
      fn ⊛ RequiresSandbox(change *Change) → bool
      fn ⊛ Execute(ctx context.Context, task *types.Task, packet *types.PromptPacket, output interface{}) → (*ExecutionResult, error)
      fn ⊛ CheckApprovalChain(ctx context.Context, change *Change) → error
      fn ⊛ IsSystemChange(change *Change) → bool
      fn ⊛ RepoPath() → string

## governor/internal/maintenance/sandbox.go
sandbox.go [165L]
  deps: os/exec, context, os, path/filepath, fmt, time, io, log
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
  deps: log, fmt, os, time, strings, encoding/json, path/filepath
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
  deps: github.com/vibepilot/governor/internal/runtime, encoding/json, context, fmt
  exports: NewMCPToolExecutor
  API:
    cl ⊛ MCPToolExecutor
    fn ⊛ NewMCPToolExecutor(registry *Registry, toolName string) → *MCPToolExecutor
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
      fn ⊛ RegisterToolsInRegistry(toolRegistry *runtime.ToolRegistry)

## governor/internal/mcp/governor_server.go
governor_server.go [211L]
  deps: os, github.com/vibepilot/governor/internal/runtime, encoding/json, fmt, log, strconv, github.com/mark3labs/mcp-go/mcp, github.com/mark3labs/mcp-go/server, context
  exports: NewGovernorServer
  API:
    cl ⊛ GovernorServer
    fn ⊛ NewGovernorServer(registry *runtime.ToolRegistry, config *runtime.Config, cfg runtime.GovernorMCPConfig) → *GovernorServer
      fn ⊛ Start(ctx context.Context) → error
      fn ⊛ Shutdown()
    fn jsonEscape(s string) → string

## governor/internal/mcp/registry.go
registry.go [253L]
  deps: fmt, encoding/json, github.com/mark3labs/mcp-go/mcp, time, context, github.com/mark3labs/mcp-go/client, github.com/mark3labs/mcp-go/client/transport, log, github.com/vibepilot/governor/internal/runtime, sync
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
  deps: time, github.com/vibepilot/governor/internal/db, github.com/vibepilot/governor/internal/runtime, context, encoding/json, fmt, strings
  exports: NewCompactor
  API:
    cl ⊛ SessionSummary
    cl ⊛ Compactor
    fn ⊛ NewCompactor(dbClient *db.DB) → *Compactor
      fn ⊛ Compact(ctx context.Context, result *runtime.SessionResult, taskID string) → (*SessionSummary, error)
      fn ⊛ CompactSession(ctx context.Context, result *runtime.SessionResult, taskID string)
      fn ⊛ GetRecentSummaries(ctx context.Context, agentID string, limit int) → ([]SessionSummary, error)
      fn ⊛ BuildCompactionContext(ctx context.Context, taskID string, maxSummaries int) → (string, error)

## governor/internal/memory/service.go
service.go [276L]
  deps: fmt, encoding/json, github.com/vibepilot/governor/internal/db, time, context
  exports: New
  API:
    cl ⊛ Config
    cl ⊛ Rule
    cl ⊛ MemoryService
    fn ⊛ New(database *db.DB, cfg Config) → *MemoryService
      fn ⊛ StoreShortTerm(ctx context.Context, sessionID, agentType string, contextData map[string]any) → error
      fn ⊛ GetShortTerm(ctx context.Context, sessionID string) → (map[string]any, error)
      fn ⊛ StoreProjectState(ctx context.Context, projectID, key string, value map[string]any) → error
      fn ⊛ GetProjectState(ctx context.Context, projectID, key string) → (map[string]any, error)
      fn ⊛ StoreRule(ctx context.Context, category, ruleText, source string, priority int) → error
      fn ⊛ GetRulesByCategory(ctx context.Context, category string) → ([]Rule, error)
      fn ⊛ GetRulesByPriority(ctx context.Context, minPriority int) → ([]Rule, error)
      fn ⊛ CleanExpired(ctx context.Context) → error

## governor/internal/realtime/client.go
client.go [648L]
  deps: sync, time, github.com/vibepilot/governor/internal/runtime, net/url, context, encoding/json, fmt, github.com/coder/websocket, net/http, log
  exports: NewClient
  API:
    cl ⊛ Client
    cl ⊛ Subscription
    cl ⊛ Config
    cl phoenixMessage
    cl postgresChangesPayload
    cl postgresChangesConfig
    cl channelResponse
    cl ⊛ ChangeEvent
    fn ⊛ NewClient(cfg *Config, router *runtime.EventRouter) → *Client
      fn ⊛ Connect() → error
      fn ⊛ SubscribeToTable(table string) → error
      fn ⊛ SubscribeToTableWithFilter(table, event, filter string) → error
      fn ⊛ SubscribeToAllTables() → error
      fn ⊛ Close() → error
      fn ⊛ IsConnected() → bool
    fn mustMarshal(v interface{}) → json.RawMessage
    fn extractID(record map[string]interface{}) → string

## governor/internal/runtime/config.go
config.go [1175L]
  deps: sync, encoding/json, fmt, context, os, path/filepath, log
  exports: LoadConfig
  API:
    cl ⊛ SystemConfig
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
      fn ⊛ GetDatabaseKey() → string
      fn ⊛ GetRealtimeURL() → string
      fn ⊛ GetVaultKey() → string
      fn ⊛ GetProtectedBranches() → []string
      fn ⊛ GetRepoPath() → string
      fn ⊛ GetGitTimeout() → int
      fn ⊛ GetDefaultMergeTarget() → string
      fn ⊛ GetBranchNamePattern() → string
      fn ⊛ GetGitTimeoutSeconds() → int
      fn ⊛ GetRemoteName() → string
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
      fn ⊛ GetDefaultCLIArgs() → []string

## governor/internal/runtime/connector_tracker.go
connector_tracker.go [283L]
  deps: sync, encoding/json, time, context, log
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
context_builder.go [201L]
  deps: strings, fmt, context, encoding/json
  exports: NewContextBuilder
  API:
    if ⊛ RPCQuerier
    if ⊛ MCPToolLister
    cl ⊛ MCPToolInfo
    cl ⊛ ContextBuilder
    fn ⊛ NewContextBuilder(db RPCQuerier) → *ContextBuilder
      fn ⊛ SetMCPRegistry(registry MCPToolLister)
      fn ⊛ BuildPlannerContext(ctx context.Context, projectType string) → (string, error)
      fn ⊛ BuildSupervisorContext(ctx context.Context, taskType string) → (string, error)
      fn ⊛ BuildTesterContext(ctx context.Context, taskType string) → (string, error)
      fn ⊛ GetRoutingHeuristic(ctx context.Context, taskType string) → (modelID string, action map[string]any)
      fn ⊛ GetProblemSolution(ctx context.Context, failureType, taskType string) → (solutionType string, solutionModel string, details map[string]any)

## governor/internal/runtime/decision_escape_fix.go
decision_escape_fix.go [16L]
  API:
    fn unescapePlanContent(s string) → string

## governor/internal/runtime/decision.go
decision.go [444L]
  deps: strings, encoding/json
  exports: ParseResearchReview, ParseSupervisorDecision, ParseCouncilVote, ParsePlannerOutput, ParseTestResults, ParseInitialReview, ParseTaskRunnerOutput, CategorizeFailure
  API:
    cl ⊛ Issue
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
    fn ⊛ ParsePlannerOutput(output string) → (*PlannerOutput, error)
    fn ⊛ ParseTestResults(output string) → (*TestResults, error)
    fn ⊛ ParseInitialReview(output string) → (*InitialReviewDecision, error)
    fn ⊛ ParseTaskRunnerOutput(output string) → (*TaskRunnerOutput, error)
    fn parseFilesArray(raw json.RawMessage) → []File
    fn extractJSON(output string) → string
    fn ⊛ CategorizeFailure(issueType string) → string
    fn sanitizeJSON(input string) → string
    fn extractPlanContent(raw string) → (planContent string, cleanJSON string)

## governor/internal/runtime/events.go
events.go [135L]
  deps: log, time, context, encoding/json, sync
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
model_loader.go [364L]
  deps: path/filepath, os, encoding/json, log, context, time, github.com/vibepilot/governor/internal/db, fmt
  exports: NewModelLoader, LoadModelsFromConfig
  API:
    cl ⊛ ModelsConfigFile
    cl ⊛ ModelLoader
    fn ⊛ NewModelLoader(configPath string, database *db.DB, tracker *UsageTracker) → *ModelLoader
      fn ⊛ Load(ctx context.Context) → error
      fn ⊛ Reload(ctx context.Context) → error
    fn ⊛ LoadModelsFromConfig(configDir string, database *db.DB, tracker *UsageTracker) → (*ModelLoader, error)
    fn loadConnectorSharedLimits(connectorsPath string, tracker *UsageTracker)
    fn loadWebPlatformLimits(connectorsPath string, tracker *UsageTracker)
      fn ⊛ GetModel(modelID string) → *ModelProfile
      fn ⊛ ListModels() → []string
      fn ⊛ GetActiveModels() → []string
      fn ⊛ GetAvailableModels(ctx context.Context) → []string
      fn ⊛ UpdateLearnedData(ctx context.Context, modelID string, learned LearnedData) → error

## governor/internal/runtime/parallel.go
parallel.go [230L]
  deps: context, fmt, log, sync/atomic, sync
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
  deps: context, sync, time, log, encoding/json
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

## governor/internal/runtime/router.go
router.go [767L]
  deps: log, context, sync/atomic, github.com/vibepilot/governor/internal/db, sort, encoding/json
  exports: NewRouter
  API:
    cl ⊛ Router
    fn ⊛ NewRouter(cfg *Config, database *db.DB, usageTracker *UsageTracker) → *Router
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
session.go [243L]
  deps: time, strings, context, encoding/json, fmt
  exports: WithTimeout, NewSession, NewSessionFactory
  API:
    if ⊛ ConnectorRunner
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
  deps: encoding/json, fmt, context
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
usage_tracker.go [786L]
  deps: context, fmt, encoding/json, log, sync, time
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
      fn ⊛ PlatformCanMakeRequest(ctx context.Context, platformID string, estimatedTokens int) → (bool, time.Duration)
      fn ⊛ RecordPlatformMessage(ctx context.Context, platformID string, tokensUsed int)
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
      fn ⊛ GetMinuteRequestCount(ctx context.Context, modelID string) → int
      fn ⊛ GetModelLearnedScore(modelID string, taskType string) → float64
      fn ⊛ GetModelCooldownMinutes(modelID string) → int

## governor/internal/security/leak_detector.go
leak_detector.go [69L]
  deps: regexp, log
  exports: NewLeakDetector
  API:
    cl ⊛ LeakWarning
    cl leakPattern
    cl ⊛ LeakDetector
    fn ⊛ NewLeakDetector() → *LeakDetector
      fn ⊛ Scan(output string) → (string, []LeakWarning)
    fn maskSecret(s string) → string

## governor/internal/tools/db_tools.go
db_tools.go [255L]
  deps: fmt, context, encoding/json, regexp, strings, github.com/vibepilot/governor/internal/db
  exports: NewDBQueryTool, NewDBUpdateTool, NewDBInsertTool, NewDBRPCTool, NewMaintenanceCommandTool
  API:
    fn sanitizeFilterValue(val interface{}) → string
    fn sanitizeColumnName(name string) → string
    cl ⊛ DBQueryTool
    fn ⊛ NewDBQueryTool(database *db.DB) → *DBQueryTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ DBUpdateTool
    fn ⊛ NewDBUpdateTool(database *db.DB) → *DBUpdateTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ DBInsertTool
    fn ⊛ NewDBInsertTool(database *db.DB) → *DBInsertTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ DBRPCTool
    fn ⊛ NewDBRPCTool(database *db.DB) → *DBRPCTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ MaintenanceCommandTool
    fn ⊛ NewMaintenanceCommandTool(database *db.DB) → *MaintenanceCommandTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/file_tools.go
file_tools.go [177L]
  deps: path/filepath, strings, context, encoding/json, fmt, os
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
  deps: fmt, github.com/vibepilot/governor/internal/gitree, context, encoding/json
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
  deps: github.com/vibepilot/governor/internal/vault, github.com/vibepilot/governor/internal/db, net/http, github.com/vibepilot/governor/internal/gitree, github.com/vibepilot/governor/internal/runtime, time
  exports: RegisterAll
  API:
    cl ⊛ Dependencies
    fn init()
    fn ⊛ RegisterAll(registry *runtime.ToolRegistry, deps *Dependencies)

## governor/internal/tools/sandbox_tools.go
sandbox_tools.go [245L]
  deps: fmt, path/filepath, time, bytes, os/exec, os, strings, context, encoding/json
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
  deps: github.com/vibepilot/governor/internal/vault, encoding/json, context, fmt
  exports: NewVaultGetTool
  API:
    cl ⊛ VaultGetTool
    fn ⊛ NewVaultGetTool(v *vault.Vault) → *VaultGetTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/tools/web_tools.go
web_tools.go [231L]
  deps: net/http, github.com/vibepilot/governor/internal/runtime, strings, net/url, encoding/json, context, io, fmt
  exports: NewWebSearchTool, NewWebFetchTool
  API:
    cl ⊛ WebSearchTool
    fn ⊛ NewWebSearchTool(allowlist []string, config *runtime.WebToolsConfig) → *WebSearchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)
    cl ⊛ WebFetchTool
    fn ⊛ NewWebFetchTool(allowlist []string, config *runtime.WebToolsConfig) → *WebFetchTool
      fn ⊛ Execute(ctx context.Context, args map[string]any) → (json.RawMessage, error)

## governor/internal/vault/vault.go
vault.go [337L]
  deps: encoding/base64, io, crypto/cipher, crypto/aes, crypto/rand, encoding/json, github.com/vibepilot/governor/internal/db, crypto/sha256, os, context, fmt, sync, time, log, golang.org/x/crypto/pbkdf2
  exports: New, NewWithoutAudit, Encrypt, GetEnvOrVault
  API:
    cl ⊛ Vault
    cl cachedSecret
    cl ⊛ AuditEntry
    cl ⊛ SecretRecord
    fn ⊛ New(database *db.DB) → *Vault
    fn ⊛ NewWithoutAudit(database *db.DB) → *Vault
    fn getMachineSalt() → []byte
      fn ⊛ GetSecret(ctx context.Context, keyName string) → (string, error)
      fn ⊛ GetSecretNoCache(ctx context.Context, keyName string) → (string, error)
    fn ⊛ Encrypt(plaintext, masterKey string) → (string, error)
    fn deriveKey(password string, salt []byte) → []byte
      fn ⊛ InvalidateCache(keyName string)
      fn ⊛ InvalidateAll()
      fn ⊛ CacheStats() → map[string]interface{}
    fn ⊛ GetEnvOrVault(ctx context.Context, v *Vault, keyName string) → string

## governor/internal/webhooks/github.go
github.go [129L]
  deps: strings, log, github.com/vibepilot/governor/internal/db, encoding/json, context
  exports: NewGitHubWebhookHandler
  API:
    cl ⊛ GitHubWebhookHandler
    cl ⊛ GitHubPushPayload
    cl ⊛ GitHubCommit
    fn ⊛ NewGitHubWebhookHandler(database *db.DB, prdDir string) → *GitHubWebhookHandler
      fn ⊛ HandlePush(ctx context.Context, body []byte)

## governor/internal/webhooks/server.go
server.go [327L]
  deps: fmt, log, crypto/hmac, encoding/json, context, strings, time, crypto/sha256, encoding/hex, net/http, github.com/vibepilot/governor/internal/runtime, io
  exports: NewServer, GetWebhookURL
  API:
    cl ⊛ Server
    ty ⊛ EventHandler
    cl ⊛ Config
    cl ⊛ Payload
    fn ⊛ NewServer(cfg *Config, router *runtime.EventRouter) → *Server
      fn ⊛ SetGitHubHandler(handler *GitHubWebhookHandler)
      fn ⊛ RegisterHandler(eventType string, handler EventHandler)
      fn ⊛ Start(ctx context.Context) → error
      fn ⊛ Shutdown(ctx context.Context) → error
    fn extractID(record map[string]any) → string
      fn ⊛ GetPort() → int
      fn ⊛ IsRunning() → bool
    fn ⊛ GetWebhookURL(baseURL string, port int, path string) → string

## governor/pkg/types/types.go
types.go [124L]
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


