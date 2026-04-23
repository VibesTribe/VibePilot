# PLAN: Add Version Command

## Overview
Add a `version` subcommand to the governor CLI that prints the build version. The current binary has no CLI subcommand framework — it runs directly via `main()`. Version vars (`version`, `commit`, `date`) already exist at lines 27-31 of main.go. This plan adds cobra-based subcommand support and a dedicated `version` command.

## Tasks

### T001: Add Cobra CLI Framework and Version Command
**Confidence:** 0.97
**Category:** coding
**Dependencies:** none

#### Prompt Packet
```
# TASK: T001 - Add Cobra CLI Framework and Version Command

## Context
The governor binary currently runs directly via `main()` with no subcommand support. It already has version variables declared:

```go
var (
    version = "2.0.0"
    commit  = "dev"
    date    = "unknown"
)
```

We need to add cobra so the binary supports `./governor version` as a subcommand that prints the version info.

## What to Build

### 1. Add cobra dependency
Run: `cd ~/VibePilot/governor && go get github.com/spf13/cobra@latest`

### 2. Create `governor/cmd/governor/version.go`

```go
package main

import (
    "fmt"

    "github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print the governor version",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Printf("VibePilot Governor v%s\n", version)
        },
    }
}
```

Note: `version` is a package-level var already declared in main.go, so this file can reference it directly since it's the same `main` package.

### 3. Create `governor/cmd/governor/root.go`

Create a root cobra command. Move the `main()` body into a `runE` function. Keep the existing `main()` as the entry point that creates the root command and executes it.

```go
package main

import (
    "github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
    rootCmd := &cobra.Command{
        Use:   "governor",
        Short: "VibePilot Governor - sovereign AI execution engine",
        RunE:  runGovernor,
    }
    rootCmd.AddCommand(newVersionCommand())
    return rootCmd
}
```

### 4. Modify `governor/cmd/governor/main.go`

Replace the current `main()` function body. Keep the `version`, `commit`, `date` vars. Keep all other functions (getConfigDir, getEnvOrDefault, registerConnectors, setupEventHandlers). Change `main()` to:

```go
func main() {
    rootCmd := newRootCommand()
    if err := rootCmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
```

Move all the current `main()` body logic (everything from `log.Printf("VibePilot Governor...")` through `<-sigCh` and shutdown) into a new function:

```go
func runGovernor(cmd *cobra.Command, args []string) error {
    // ... all the existing main() body logic moved here ...
    return nil
}
```

### 5. Create `governor/cmd/governor/version_test.go`

```go
package main

import (
    "bytes"
    "os/exec"
    "testing"
)

func TestVersionCommand(t *testing.T) {
    // Test via cobra command directly
    cmd := newVersionCommand()
    buf := new(bytes.Buffer)
    cmd.SetOut(buf)
    cmd.SetArgs([]string{})
    cmd.Execute()

    output := buf.String()
    if !bytes.Contains([]byte(output), []byte("VibePilot")) {
        t.Errorf("Expected output to contain 'VibePilot', got: %s", output)
    }
}

func TestVersionSubcommand(t *testing.T) {
    // Test that root has version subcommand
    root := newRootCommand()
    verCmd, _, err := root.Find([]string{"version"})
    if err != nil {
        t.Fatalf("Failed to find version subcommand: %v", err)
    }
    if verCmd.Name() != "version" {
        t.Errorf("Expected command name 'version', got '%s'", verCmd.Name())
    }
}
```

## Files
- `governor/cmd/governor/version.go` - Version cobra command (NEW)
- `governor/cmd/governor/root.go` - Root cobra command (NEW)
- `governor/cmd/governor/main.go` - Simplified entry point (MODIFY)
- `governor/cmd/governor/version_test.go` - Tests (NEW)
- `governor/go.mod` - Updated with cobra dependency (MODIFY)
```

#### Expected Output
```json
{
  "task_id": "T001",
  "files_created": [
    "governor/cmd/governor/version.go",
    "governor/cmd/governor/root.go",
    "governor/cmd/governor/version_test.go"
  ],
  "files_modified": [
    "governor/cmd/governor/main.go",
    "governor/go.mod",
    "governor/go.sum"
  ],
  "tests_written": [
    "governor/cmd/governor/version_test.go"
  ],
  "verification": [
    "cd governor && go build ./cmd/governor/",
    "cd governor && go test ./cmd/governor/ -run TestVersion"
  ]
}
```
