# flowcmd

Go CLI tool for executing YAML-defined workflows with sequential/parallel steps and LLM integration.

## Build & Test
- `make build` — build `bin/flowcmd`
- `make test` — run all tests
- `make race` — run tests with the race detector
- `make lint` — `golangci-lint run`
- `go build -o bin/flowcmd .` — direct build
- `go test ./...` — direct test

## Architecture
- `cmd/` — Cobra CLI commands (root, init, add, remove, run, validate, list); `cmd/scope.go` maps local (`./.flowcmd`) and global (`~/.flowcmd`) scope dirs; `cmd/templates/` holds embedded starter workflows
- `internal/parser/` — YAML parsing and schema/template validation
- `internal/engine/` — Workflow execution engine (sequential + parallel groups, retry, when, ctx-cancel)
- `internal/template/` — `{{ steps.name.output }}` / `{{ steps[0].output }}` interpolation
- `internal/tui/` — Bubbletea TUI (icons, spinner, timing, parallel indicator)
- `internal/textutil/` — Shared text helpers
- `internal/types/` — Shared type definitions

## Conventions
- Use `internal/` for non-exported packages
- Error handling: return errors, don't panic
- Tests: table-driven, test files next to source (`*_test.go`)
- Use `context.Context` for cancellation in engine
- Event channel carries immutable `*StepResult` snapshots; never mutate a result after `setResult`

## Key Invariants
- Template references must point to earlier steps; enforced at validation time
- Step names match `^[a-z][a-z0-9_-]*$`, max 64 chars, unique within workflow
- A parallel group is a run of adjacent `parallel: true` steps; a failing step in the group cancels remaining via shared child context
