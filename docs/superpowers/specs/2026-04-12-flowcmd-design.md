# FlowCmd Design Spec

## Context

Today's AI workflow tools force a binary choice: either everything goes through an LLM (Claude Code, Copilot agents) — making deterministic tasks slow and expensive — or you use traditional task runners (Make, Taskfile) with zero AI awareness. FlowCmd bridges this gap: a CLI tool that executes declarative YAML workflows where shell scripts and LLM calls are first-class citizens. You pay for AI only when you need judgment.

## Product Vision

**"Taskfile for the AI era."** A single Go binary that runs YAML-defined workflows with sequential and parallel steps, template variable interpolation, conditionals, retry logic, and a beautiful TUI showing real-time execution status.

## Target Users

- Individual developers automating workflows (commit, deploy, test, review)
- DevOps/platform teams building shared pipelines with LLM integration
- AI/ML engineers needing orchestration beyond pure NLP prompts

## Business Model

Open-source (MIT/Apache). Community-driven growth.

---

## Architecture

### Project Structure

```
flowcmd/
├── cmd/                    # Cobra CLI commands
│   ├── root.go             # Root command, global flags
│   ├── run.go              # flowcmd run <workflow.yml>
│   ├── validate.go         # flowcmd validate <workflow.yml>
│   └── list.go             # flowcmd list [dir]
├── internal/
│   ├── parser/             # YAML parsing + schema validation
│   │   └── parser.go
│   ├── engine/             # Workflow execution engine
│   │   ├── engine.go       # Main orchestrator
│   │   ├── step.go         # Step execution (shell via exec.Command)
│   │   └── parallel.go     # Parallel step group execution
│   ├── template/           # {{ }} interpolation engine
│   │   └── template.go
│   ├── tui/                # Terminal UI (charmbracelet/bubbletea)
│   │   ├── model.go        # Bubbletea model
│   │   └── styles.go       # Lipgloss styles
│   └── types/              # Shared types
│       └── types.go
├── main.go
├── go.mod
├── Makefile
├── CLAUDE.md
└── README.md
```

### Dependencies

| Dependency | Purpose |
|-----------|---------|
| `spf13/cobra` | CLI framework (includes viper for config) |
| `gopkg.in/yaml.v3` | YAML parsing |
| `charmbracelet/bubbletea` | TUI framework |
| `charmbracelet/lipgloss` | TUI styling |
| `charmbracelet/bubbles` | TUI components (spinner, etc.) |

---

## YAML Workflow Schema

```yaml
name: Commit Agent                    # Required. Workflow display name
description: Analyzes changes and...  # Optional. Shown in TUI header

steps:                                # Required. List of steps
  - name: identify-changes            # Required. ^[a-z][a-z0-9_-]*$ max 64 chars
    description: Get staged changes   # Optional. Shown in TUI
    run: git diff --cached            # Required. Shell command to execute

  - name: generate-message
    description: AI-generated commit message
    run: |
      claude -p "Generate a commit message for these changes: {{ steps.identify-changes.output }}"
    retry:                            # Optional
      attempts: 2                     # Default: 1
      delay: 5s                       # Default: 0s

  - name: commit
    description: Commit with generated message
    run: git commit -m "{{ steps.generate-message.output }}"
    when: "{{ steps.identify-changes.output != '' }}"  # Optional conditional

  - name: lint
    description: Run linter
    run: golangci-lint run
    parallel: true                    # Runs concurrently with adjacent parallel steps

  - name: test
    description: Run tests
    run: go test ./...
    parallel: true
```

### Schema Rules

1. **Steps are sequential by default.** Only steps with `parallel: true` run concurrently.
2. **Adjacent parallel steps form a group.** When a non-parallel step follows parallel steps, execution waits for all parallel steps to complete before continuing.
3. **Step names** must match `^[a-z][a-z0-9_-]*$`, max 64 characters. Enforced by validator.
4. **Step names must be unique** within a workflow.
5. **`run`** is a shell command executed via `sh -c` (or user's default shell).
6. **`when`** is a simple expression. If it evaluates to false/empty, the step is skipped.
7. **`retry`** re-executes the step on non-zero exit code, up to `attempts` times with `delay` between.

---

## Template Engine

### Variable Access

Two syntaxes supported:

| Syntax | Example | Description |
|--------|---------|-------------|
| Named | `{{ steps.identify-changes.output }}` | Recommended. Refactor-safe. |
| Numeric | `{{ steps[0].output }}` | Positional. 0-indexed. |

### Available Properties

| Property | Type | Description |
|----------|------|-------------|
| `.output` | string | Captured stdout (trimmed) |
| `.error` | string | Captured stderr |
| `.exitcode` | int | Exit code (0 = success) |

### Template Resolution

- Templates are resolved **at execution time**, not parse time
- A template referencing a step that hasn't run yet is a validation error
- A template referencing a non-existent step name is a validation error
- Output is trimmed of leading/trailing whitespace

---

## Execution Engine

### Flow

1. **Parse** — Load and parse YAML file
2. **Validate** — Check schema, step name constraints, template references, detect cycles
3. **Build execution plan** — Group adjacent parallel steps, create ordered execution list
4. **Execute** — Run steps sequentially; parallel groups run concurrently via goroutines
5. **Template interpolation** — Resolve `{{ }}` expressions at each step's execution time
6. **Output capture** — Capture stdout/stderr per step, store in step result
7. **Conditional evaluation** — Evaluate `when` expressions, skip if false
8. **Retry logic** — On failure, retry based on `retry` config
9. **TUI update** — Update terminal display after each step state change

### Error Handling

- **Step failure (non-zero exit):** Workflow stops immediately (fail-fast). Parallel group: cancel remaining, wait for running steps.
- **Template resolution failure:** Workflow stops with clear error message pointing to the template.
- **Retry exhaustion:** Step marked as failed, workflow stops.
- **Skipped steps:** `when` evaluating to false marks step as skipped (shown in TUI as gray with skip indicator).

### Parallel Execution

Adjacent `parallel: true` steps are collected into a group. The group executes all steps concurrently using goroutines with a `sync.WaitGroup`. If any step in the group fails, remaining steps are allowed to complete (no cancellation within group), but the workflow stops after the group.

---

## TUI Design

### Layout

```
 Commit Agent
 Analyzes changes and commits with AI-generated message

 ✓ identify-changes    Get staged changes              2.1s
   ╰─ 3 files changed, 47 insertions(+), 12 deletions(-)
 ✓ generate-message    AI-generated commit message      4.3s
   ╰─ fix: resolve null pointer in auth middleware
 ● commit              Commit with generated message     ...
 ○ lint                Run linter
 ○ test                Run tests
```

### Status Indicators

| State | Symbol | Color |
|-------|--------|-------|
| Pending | ○ | Gray |
| In Progress | ● (spinner) | Orange/Yellow |
| Completed | ✓ | Green |
| Failed | ✗ | Red |
| Skipped | ○ (skip) | Dim gray |

### Features

- Workflow name and description shown as header
- Each step shows: status icon, name, description, elapsed time
- Last line of stdout shown below completed steps (truncated to terminal width)
- Failed steps show stderr in red below the step
- `--verbose` flag shows full stdout/stderr inline
- Parallel steps shown with a visual grouping indicator

### Library

Built with **charmbracelet/bubbletea** (Elm-architecture TUI framework) and **charmbracelet/lipgloss** (styling). Spinner from **charmbracelet/bubbles**.

---

## CLI Commands

### `flowcmd run <file.yml> [flags]`

Execute a workflow.

| Flag | Description | Default |
|------|-------------|---------|
| `--verbose, -v` | Show full step output inline | false |
| `--dry-run` | Validate and show execution plan, don't execute | false |
| `--no-tui` | Plain text output (for CI/piping) | false |

### `flowcmd validate <file.yml>`

Validate a workflow file. Checks:
- YAML syntax
- Schema compliance (required fields, types)
- Step name constraints and uniqueness
- Template reference validity (referenced steps exist and precede current step)

Exit code 0 = valid, non-zero = invalid with error details.

Uses the **same parser/validator** as `flowcmd run` — single code path.

### `flowcmd list [dir]`

List all `.yml` workflow files in a directory (default: current dir). Shows name, description, and step count.

---

## CLAUDE.md Configuration

The project will include a `CLAUDE.md` for AI-assisted development:

```markdown
# FlowCmd

Go CLI tool for executing YAML-defined workflows with sequential/parallel steps and LLM integration.

## Build & Test
- `go build -o flowcmd ./...` to build
- `go test ./...` to run all tests
- `golangci-lint run` to lint

## Architecture
- `cmd/` — Cobra CLI commands
- `internal/parser/` — YAML parsing and validation
- `internal/engine/` — Workflow execution engine
- `internal/template/` — Template interpolation
- `internal/tui/` — Bubbletea TUI
- `internal/types/` — Shared type definitions

## Conventions
- Use `internal/` for non-exported packages
- Error handling: return errors, don't panic
- Tests: table-driven tests, test files next to source
- Use `context.Context` for cancellation in engine
```

---

## Verification Plan

### Manual Testing

1. Create a simple 3-step sequential workflow, run with `flowcmd run`
2. Create a workflow with `parallel: true` steps, verify concurrent execution
3. Test `when` conditionals — verify step skipping
4. Test `retry` — create a step that fails then succeeds
5. Test template interpolation — verify `steps.name.output` and `steps[0].output`
6. Test `flowcmd validate` on valid and invalid files
7. Test TUI rendering — verify colors, status indicators, timing
8. Test `--verbose`, `--dry-run`, `--no-tui` flags
9. Test error cases: invalid YAML, missing step reference, circular template ref

### Automated Tests

- Unit tests for parser, template engine, and validator
- Integration tests for engine with real shell commands
- TUI tests using bubbletea's test utilities

### End-to-End

Run the commit agent example from the original vision document as the acceptance test.
