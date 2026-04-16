# PRD: Add Version Command

## Summary
Add a `version` subcommand to the governor CLI that prints the build version.

## Requirements
- Add a `version.go` file in `governor/cmd/governor/` that registers a `version` command
- The command should print `VibePilot Governor v0.1.0` to stdout
- Register it in the root command in `main.go`
- Add a test file `version_test.go` that verifies the output contains "VibePilot"

## Acceptance Criteria
- `go build ./cmd/governor/` compiles
- `./governor version` prints `VibePilot Governor v0.1.0`
- `go test ./cmd/governor/ -run TestVersion` passes

## Files
- `governor/cmd/governor/version.go` (new)
- `governor/cmd/governor/version_test.go` (new)
- `governor/cmd/governor/main.go` (modify - add version command registration)
