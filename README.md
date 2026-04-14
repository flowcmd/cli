<div align="center">

# flowcmd

**Taskfile for the AI era.**
Run YAML workflows where shell commands and LLM calls are first-class steps.

[![CI](https://github.com/flowcmd/cli/actions/workflows/ci.yml/badge.svg)](https://github.com/flowcmd/cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/flowcmd/cli.svg)](https://pkg.go.dev/github.com/flowcmd/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/flowcmd/cli)](https://goreportcard.com/report/github.com/flowcmd/cli)

</div>

```sh
# install
go install github.com/flowcmd/cli@latest

# scaffold a workflow
flowcmd init

# run it
flowcmd run hello
```

That's it. You just ran your first workflow. Keep reading to write a real one.

---

## Table of contents

- [Why flowcmd](#why-flowcmd)
- [Your first workflow](#your-first-workflow) — 2-minute tutorial
- [Core concepts](#core-concepts) — scope, resolution, templates
- [Command reference](#command-reference)
- [Cookbook](#cookbook) — real-world recipes
- [Workflow schema](#workflow-schema) — full YAML reference
- [FAQ](#faq)
- [Contributing](#contributing)

---

## Why flowcmd

Today's AI automation forces a choice:

- **All-LLM orchestration** (Claude Code, agent frameworks) — the model drives every step. Slow, expensive, non-deterministic for work that's really just `git diff`.
- **Classic task runners** (Make, Taskfile, just) — fast and deterministic, but zero AI awareness. Shelling out to `claude -p` works but there's no state between steps.

**flowcmd is the third option.** Declarative YAML. Shell steps run as shell. LLM steps are just steps that happen to call an LLM. Outputs flow between them through template references. You pay for AI only where you actually need judgment.

```yaml
steps:
  - name: diff
    run: git diff --cached            # cheap, deterministic

  - name: message
    run: claude -p "Commit msg: {{ steps.diff.output }}"   # LLM only where needed

  - name: commit
    run: git commit -m "{{ steps.message.output }}"        # deterministic again
```

---

## Your first workflow

A 2-minute tutorial. By the end you have a working commit agent.

### 1. Install

```sh
go install github.com/flowcmd/cli@latest
```

Verify:

```sh
flowcmd --version
```

### 2. Scaffold

```sh
flowcmd init
```

Creates `./.flowcmd/hello.yml`, a two-step sample. Run it:

```sh
flowcmd run hello
```

You'll see a live TUI with checkmarks as each step completes.

### 3. Write a real one

Create `./.flowcmd/commit.yml`:

```yaml
name: Commit agent
description: Generate a commit message from staged changes and commit.

steps:
  - name: diff
    description: Read staged changes
    run: git diff --cached

  - name: message
    description: Ask the LLM for a commit message
    run: |
      claude -p "Write a one-line commit message for:
      {{ steps.diff.output }}"
    when: "{{ steps.diff.output != '' }}"

  - name: commit
    description: Commit with the generated message
    run: git commit -m "{{ steps.message.output }}"
    when: "{{ steps.diff.output != '' }}"
```

Run it:

```sh
flowcmd run commit
```

Done. You've just replaced a multi-turn AI agent with a deterministic workflow that only invokes the LLM where judgment is needed.

### 4. Share it

Install it globally so every project can use it:

```sh
flowcmd add -g ./.flowcmd/commit.yml
```

Now `flowcmd run commit` works from any directory.

---

## Core concepts

### Scope

Workflows live in two places:

| Scope | Directory | Meaning |
|---|---|---|
| **local** | `./.flowcmd/` | Project-specific workflows. Check into git. |
| **global** | `~/.flowcmd/` | Personal workflows shared across all your projects. |

### Resolution

`flowcmd run <arg>` behaves differently based on what `<arg>` looks like:

- **Contains `/`, `\`, `.yml`, or `.yaml`** → treated as a **file path**. Used as-is.
- **Bare name** → resolved through scopes:
  1. Look in `./.flowcmd/<name>.yml` (local wins).
  2. Fall back to `~/.flowcmd/<name>.yml`.
  3. If both have it, local is used and a warning is printed to stderr.
  4. If neither has it, exit with an error listing both scopes.

### Templates

Any step can reference the output of any earlier step.

| Syntax | Example | Notes |
|---|---|---|
| By name | `{{ steps.diff.output }}` | Preferred. Refactor-safe. |
| By index | `{{ steps[0].output }}` | Positional, 0-indexed. |
| Property | `.output`, `.error`, `.exitcode` | stdout (trimmed), stderr, exit code |
| Comparison | `{{ steps.x.output != '' }}` | For `when:` conditionals. Supports `==` and `!=`. |

Templates resolve at execution time. Forward references (to steps that haven't run yet) are caught at validation time — `flowcmd validate` will reject them.

### Execution model

- Steps run **sequentially** by default.
- Adjacent steps with `parallel: true` form a **concurrent group** — they run at the same time, and the workflow waits for all of them before moving on.
- On failure anywhere: workflow stops. If it's inside a parallel group, siblings are cancelled via `context.Context` (their running processes are killed).
- On `when:` evaluating to false/empty: the step is skipped (gray `○` in the TUI).
- `retry: { attempts: N, delay: Ts }` re-runs a step on non-zero exit, with a delay between attempts.

---

## Command reference

Every command below is copy-pasteable. Flags mirror across commands where they mean the same thing (`-g` is always "global scope").

### `flowcmd init`

Bootstrap a `.flowcmd/` directory in the current project with a starter workflow.

```sh
flowcmd init              # creates ./.flowcmd/hello.yml
flowcmd init --force      # overwrite if present
```

### `flowcmd run <name-or-path>`

Execute a workflow.

```sh
flowcmd run hello                      # by name (resolves through scopes)
flowcmd run ./.flowcmd/hello.yml       # by file path
flowcmd run hello --no-tui             # plain output (CI / logs / pipes)
flowcmd run hello --dry-run            # print execution plan, do not run
flowcmd run hello --verbose            # show full stdout of every step
```

| Flag | Default | Effect |
|---|---|---|
| `-v, --verbose` | false | Stream full stdout of every step. |
| `--dry-run` | false | Validate and show the execution plan. Nothing runs. |
| `--no-tui` | false | Plain text output. Use in CI or when piping. |

### `flowcmd add <path-or-url>`

Install a workflow into a scope. Accepts local files or `http(s)://` URLs.

```sh
flowcmd add ./my-workflow.yml                         # local scope
flowcmd add -g ./my-workflow.yml                      # global scope
flowcmd add https://example.com/flow.yml              # from a URL
flowcmd add ./my.yml --as deploy                      # save under a different name
flowcmd add ./my.yml --force                          # overwrite existing
```

Every added workflow is validated before being written. A broken YAML will never replace a working one on disk.

| Flag | Effect |
|---|---|
| `-g, --global` | Install to `~/.flowcmd/`. Default is `./.flowcmd/`. |
| `-f, --force` | Overwrite if the destination name already exists. |
| `--as <name>` | Save under a different filename (extension optional). |

### `flowcmd remove <name>`

Delete a workflow from a scope.

```sh
flowcmd remove hello         # from ./.flowcmd/
flowcmd remove -g hello      # from ~/.flowcmd/
flowcmd rm hello             # alias
```

### `flowcmd list`

Show installed workflows. Global first, local second.

```sh
flowcmd list          # both scopes
flowcmd list -g       # global only
flowcmd list -l       # local only
```

Sample output:

```
global
- commit — Commit agent [3 steps]
- deploy — Deploy pipeline [5 steps]

local
- ci — Pre-push checks [4 steps]
```

### `flowcmd validate <file.yml>`

Parse and schema-check a workflow file without running it. Exits non-zero on any error — suitable for pre-commit hooks and CI.

```sh
flowcmd validate .flowcmd/commit.yml
# ✓ Commit agent valid (3 steps)
```

Validates:
- YAML syntax
- Required fields (`name`, `steps[].name`, `steps[].run`)
- Step name regex (`^[a-z][a-z0-9_-]*$`, max 64 chars, unique)
- Template references point to earlier, real steps
- Retry config sanity

---

## Cookbook

Real-world workflows. Drop any of these into `./.flowcmd/` or `~/.flowcmd/`.

### Commit agent

Stage changes, generate a message, commit.

```yaml
name: Commit agent
steps:
  - name: diff
    run: git diff --cached
  - name: message
    run: claude -p "Commit msg for: {{ steps.diff.output }}"
    when: "{{ steps.diff.output != '' }}"
  - name: commit
    run: git commit -m "{{ steps.message.output }}"
    when: "{{ steps.diff.output != '' }}"
```

### Pre-push gate

Lint and test in parallel, then allow push.

```yaml
name: Pre-push
steps:
  - name: lint
    run: golangci-lint run
    parallel: true
  - name: test
    run: go test -race ./...
    parallel: true
  - name: ok
    run: echo "ready to push"
```

### PR review

Pull the diff against main, have an LLM review it, print the result.

```yaml
name: PR review
steps:
  - name: diff
    run: git diff origin/main...HEAD
  - name: review
    run: |
      claude -p "Review this diff. List bugs, missing tests, unclear names.
      {{ steps.diff.output }}"
  - name: show
    run: echo "{{ steps.review.output }}"
```

### Flaky test investigator

Run a test 10 times; if it ever fails, ask the LLM why.

```yaml
name: Flake hunt
steps:
  - name: run
    run: go test -run TestThing -count=10 ./...
    retry:
      attempts: 1
  - name: analyze
    when: "{{ steps.run.exitcode != 0 }}"
    run: |
      claude -p "This Go test output suggests a flake. Theories?
      {{ steps.run.error }}"
```

### Deploy gate

Build, test, and only deploy if both pass.

```yaml
name: Deploy
steps:
  - name: build
    run: make build
    parallel: true
  - name: test
    run: make test
    parallel: true
  - name: deploy
    run: ./bin/deploy
```

### Issue triage

Read an issue body from stdin, classify it with the LLM.

```yaml
name: Triage
steps:
  - name: body
    run: gh issue view $ISSUE_NUMBER --json body -q .body
  - name: classify
    run: |
      claude -p "Classify as bug/feature/question/duplicate:
      {{ steps.body.output }}"
  - name: label
    run: gh issue edit $ISSUE_NUMBER --add-label "{{ steps.classify.output }}"
```

---

## Workflow schema

Full reference for `workflow.yml`.

### Top-level

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | ✓ | Human-readable workflow name. Shown in the TUI header. |
| `description` | string | | Optional one-line description. |
| `steps` | list | ✓ | At least one step. |

### Step

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | ✓ | Matches `^[a-z][a-z0-9_-]*$`, max 64 chars, unique in the workflow. |
| `run` | string | ✓ | Shell command, executed via `sh -c`. Multi-line via `\|`. Supports templates. |
| `description` | string | | Shown in the TUI next to the step name. |
| `when` | string | | Template expression. If it evaluates to empty/`false`/`0`, the step is skipped. |
| `parallel` | bool | | When true, this step joins an adjacent parallel group. |
| `retry` | object | | `{ attempts: N, delay: <duration> }`. On non-zero exit, retries up to `N` times with `delay` between. |

### Template expressions

Wrapped in `{{ ... }}`. Resolved at each step's execution time.

| Form | Result |
|---|---|
| `{{ steps.<name>.<prop> }}` | Value of `<prop>` on named step. |
| `{{ steps[<n>].<prop> }}` | Value of `<prop>` on 0-indexed step. |
| `{{ <left> == <right> }}` | `"true"` or `"false"`. Literals are quoted: `''`, `"x"`. |
| `{{ <left> != <right> }}` | Negated equality. |

Properties: `output` (trimmed stdout), `error` (stderr), `exitcode`.

### Complete example

```yaml
name: Full example
description: Showcases every feature.

steps:
  - name: hello
    description: A simple echo
    run: echo "hello"

  - name: reuse
    description: Reference a previous step
    run: echo "previous said {{ steps.hello.output }}"

  - name: skipped-if-empty
    when: "{{ steps.hello.output != '' }}"
    run: echo "ran"

  - name: flaky
    run: /bin/sh -c "if [ -f /tmp/ok ]; then echo done; else touch /tmp/ok; exit 1; fi"
    retry:
      attempts: 3
      delay: 500ms

  - name: parallel-a
    run: sleep 0.2 && echo a
    parallel: true

  - name: parallel-b
    run: sleep 0.2 && echo b
    parallel: true

  - name: wrap-up
    run: echo "all {{ steps[5].output }} and {{ steps[4].output }}"
```

---

## FAQ

**Why YAML and not a real scripting language?**
Because declarative workflows are reviewable, diffable, and portable. If you want a real language, you can always `run: ./my-script.sh`. flowcmd orchestrates; it doesn't compete with your shell.

**Does flowcmd call any LLM by itself?**
No. flowcmd only runs shell commands. LLMs enter the picture when *you* write a step that calls one (`claude -p "..."`, `openai ...`, `curl ...`). flowcmd is the orchestrator, not the model.

**How does this compare to GitHub Actions / CircleCI / Temporal?**
Those run in the cloud and target CI/CD or durable execution. flowcmd is a local CLI — think "Taskfile that knows about LLMs" not "Temporal for AI." You can *call* flowcmd from a CI pipeline if you want.

**How does it compare to LangChain / Claude Code / agent frameworks?**
Those put an LLM in control of every step. flowcmd inverts that: the workflow controls the steps; the LLM is summoned only where needed. You get determinism by default and intelligence on demand.

**Is there a secrets story?**
flowcmd runs in your shell, so it inherits your environment. Use `.env` files or your OS keychain, same as any other tool. Nothing is captured, logged, or uploaded.

**What if a step needs interactive input?**
Steps run with stdin attached to `/dev/null`. If you need interactivity, run the interactive tool directly — flowcmd is for automation.

**Does it work on Windows?**
The Go binary cross-compiles. Commands using `sh -c` assume a POSIX shell. WSL works. Native cmd.exe / PowerShell support is not a current goal.

**How do I use my own LLM / a local model?**
Any CLI that reads a prompt and writes to stdout works. `ollama run llama3`, `llm "..." -m claude-3-5-sonnet`, a shell script wrapping an API call — anything.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

Quick dev loop:

```sh
make test      # unit + integration tests
make race      # with race detector
make lint      # golangci-lint
make build     # binary at bin/flowcmd
```

Coverage: ~98%. New code should come with tests.

Report bugs: [open an issue](https://github.com/flowcmd/cli/issues/new?template=bug_report.md).
Suggest features: [feature request](https://github.com/flowcmd/cli/issues/new?template=feature_request.md).

---

## License

[MIT](LICENSE) © The flowcmd authors.
