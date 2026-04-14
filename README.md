<div align="center">

# flowcmd

**Deterministic workflows that call an LLM only when they need one.**

[![CI](https://github.com/flowcmd/cli/actions/workflows/ci.yml/badge.svg)](https://github.com/flowcmd/cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/flowcmd/cli.svg)](https://pkg.go.dev/github.com/flowcmd/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/flowcmd/cli)](https://goreportcard.com/report/github.com/flowcmd/cli)

</div>

---

## The problem

Most AI automation today is written in plain English. A long prompt lists every step: "read the file, look for errors, run the tests, summarize the results." A large model reads the whole thing, plans it, then executes each step.

That works, but it has a cost:

- **Tokens**: every run re-ships the full instruction set to the model, plus whatever context the steps accumulate.
- **Latency**: the model has to read, plan, and narrate each step before anything actually happens on disk.
- **Fragility**: a step that's really just `git diff` becomes a model call with all the non-determinism that implies.
- **Lock-in**: the workflow lives inside whatever agent runtime you chose; you can't easily hand it to someone else to run.

Most of the steps in a real workflow don't need a model. `git diff`, `go test`, `curl`, `jq`, `gh pr view` — these are deterministic commands that should just run. The model only needs to show up where there's actual judgment: writing a commit message, classifying an issue, summarizing a diff.

## What flowcmd does

flowcmd is a small CLI that runs **workflows written as YAML**. Each workflow is a list of steps. A step is a shell command. If a step needs an LLM, you call one from that step — any LLM, any CLI, local or cloud. The model is *in* the loop, not *around* it.

```yaml
name: Commit agent
steps:
  - name: diff
    run: git diff --cached          # plain command, fast and free

  - name: message
    run: my-llm "Write a commit msg for: {{ steps.diff.output }}"
    when: "{{ steps.diff.output != '' }}"   # only when there is something to commit

  - name: commit
    run: git commit -m "{{ steps.message.output }}"
```

That's the whole idea. Commands you already know, stitched together, with the LLM invited in only where it earns its keep.

## Why this shape

- **Every step is a real process.** If it works in your shell, it works in a step.
- **Outputs flow between steps** through simple `{{ steps.<name>.output }}` templates.
- **You choose the model.** Local (e.g. a local model server), cloud (any vendor's CLI), or your own wrapper. flowcmd doesn't care; it runs whatever binary you name.
- **Sequential by default, parallel when you say so.** Mark adjacent steps `parallel: true` and they run concurrently.
- **Deterministic unless you ask for randomness.** A workflow with no LLM steps runs the same way every time.
- **Readable as a file.** A YAML workflow is something you can review, diff, commit, and hand to a teammate.

---

## Install

```sh
go install github.com/flowcmd/cli@latest
```

Verify:

```sh
flowcmd --version
```

## Your first workflow

```sh
flowcmd init      # creates ./.flowcmd/hello.yml
flowcmd run hello
```

You'll see a live terminal view with checkmarks as each step completes. That's a workflow running.

Now open `./.flowcmd/hello.yml` and read it. Two echo steps. Add a third. Run it again. That's the whole loop.

## A workflow with an LLM in it

Pick any CLI that reads a prompt and prints a response to stdout. The examples below use a placeholder `my-llm` — substitute whichever CLI you use (a local model runner, a cloud vendor's CLI, a shell script wrapping an HTTP API, or anything else).

Save this as `./.flowcmd/commit.yml`:

```yaml
name: Commit agent
description: Generate a commit message from staged changes and commit.

steps:
  - name: diff
    description: Read staged changes
    run: git diff --cached

  - name: message
    description: Ask the model for a commit message
    run: |
      my-llm "Write a one-line commit message for these changes:
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

The `diff` step is pure shell. The `message` step calls a model — but only when there's an actual diff to describe. The `commit` step is pure shell again. The LLM touched exactly one step.

## Sharing workflows

Workflows live in two places:

| Scope | Where | When to use |
|---|---|---|
| **local** | `./.flowcmd/` | Project-specific. Commit it to the repo. |
| **global** | `~/.flowcmd/` | Personal workflows you use everywhere. |

Install one into the global scope:

```sh
flowcmd add -g ./.flowcmd/commit.yml
```

Now `flowcmd run commit` works from any directory. Same workflow, any project.

---

## Writing workflows

### The shape of a workflow

```yaml
name: Example                    # required — shown in the terminal header
description: What this does      # optional

steps:                           # required — one or more steps
  - name: step-one               # lowercase, hyphens/underscores, unique
    description: What this step does
    run: echo hello              # required — any shell command
```

A step's `run` is handed to `sh -c`, so anything you can type in a shell works: pipes, `&&`, multi-line heredocs, `$ENV_VAR`, redirections, the lot.

### Reusing outputs

Any step can read what an earlier step produced:

```yaml
steps:
  - name: hello
    run: echo "world"

  - name: echo-back
    run: echo "previous said {{ steps.hello.output }}"
```

Three properties are available on every step that has run:

| Property | What it is |
|---|---|
| `.output` | stdout, trimmed |
| `.error` | stderr |
| `.exitcode` | integer exit code |

References are by name (`steps.hello.output`) or by position (`steps[0].output`). Names are refactor-safe; positions are good for quick throwaways.

### Skipping a step

Use `when:` with a simple template expression. If it evaluates to empty, `false`, or `0`, the step is skipped (shown in gray in the terminal).

```yaml
- name: deploy
  run: ./deploy.sh
  when: "{{ steps.tests.exitcode == 0 }}"
```

Supported comparisons: `==` and `!=`. Literals in comparisons should be quoted: `'0'`, `''`.

### Running steps in parallel

Mark adjacent steps `parallel: true` and they run concurrently. The workflow waits for all of them before moving on:

```yaml
- name: lint
  run: my-linter
  parallel: true

- name: test
  run: my-test-runner
  parallel: true

- name: report
  run: echo "both done"       # runs after both finish
```

If any step in a parallel group fails, the others are cancelled (their processes are killed) and the workflow stops.

### Retrying a flaky step

```yaml
- name: flaky
  run: ./sometimes-fails.sh
  retry:
    attempts: 3          # try up to 3 times
    delay: 2s            # wait 2s between attempts
```

Only non-zero exits trigger a retry. If the step still fails after all attempts, the workflow stops.

### Calling any LLM

flowcmd has no opinion about which model you call. A step just runs a command. Anything that reads a prompt and writes a response works:

```yaml
# a local model runner
- name: classify
  run: my-local-llm "Classify this issue: {{ steps.body.output }}"

# a vendor CLI
- name: summarize
  run: my-cloud-llm "Summarize: {{ steps.diff.output }}"

# your own wrapper script
- name: review
  run: ./scripts/llm.sh "Review this code: {{ steps.files.output }}"

# a raw HTTP call
- name: explain
  run: |
    curl -s https://my-endpoint/v1/complete \
      -d "{\"prompt\": \"Explain: {{ steps.error.output }}\"}" \
      | jq -r '.text'
```

Swap models by changing one line. No lock-in.

### Passing large context

Some prompts need the contents of multiple files. Build the context in a step, then reference it:

```yaml
- name: context
  run: cat src/*.go

- name: review
  run: |
    my-llm "Review this Go code for bugs:
    {{ steps.context.output }}"
```

Anything your shell can do, a step can do — read files, pipe through `jq`, grep for patterns, base64-encode, whatever.

---

## Commands

### `flowcmd init`

Create `./.flowcmd/hello.yml` — a minimal starter workflow.

```sh
flowcmd init
flowcmd init --force   # overwrite if present
```

### `flowcmd run <name-or-path>`

Run a workflow. The argument is a **name** (looked up in scopes) or a **path** (used as-is):

```sh
flowcmd run hello                      # by name
flowcmd run ./workflows/hello.yml      # by path

flowcmd run hello --no-tui             # plain text (CI, logs, pipes)
flowcmd run hello --dry-run            # print the plan, don't run
flowcmd run hello --verbose            # show full stdout of every step
```

When you pass a name, flowcmd looks in local scope first (`./.flowcmd/<name>.yml`), then global scope (`~/.flowcmd/<name>.yml`). If both exist, local wins and a warning is printed.

### `flowcmd add <path-or-url>`

Install a workflow into a scope. Accepts local files or `http(s)://` URLs (up to 1 MiB).

```sh
flowcmd add ./my-workflow.yml               # into ./.flowcmd/
flowcmd add -g ./my-workflow.yml            # into ~/.flowcmd/ (global)
flowcmd add https://example.com/flow.yml    # fetch from a URL
flowcmd add ./my.yml --as deploy            # save under a different name
flowcmd add ./my.yml --force                # overwrite existing
```

Every added workflow is validated before being written. A broken YAML never replaces a working one on disk.

### `flowcmd remove <name>`

Delete a workflow from a scope.

```sh
flowcmd remove hello        # from ./.flowcmd/
flowcmd remove -g hello     # from ~/.flowcmd/
flowcmd rm hello            # alias
```

### `flowcmd list`

Show installed workflows. Global first, local second.

```sh
flowcmd list       # both scopes
flowcmd list -g    # global only
flowcmd list -l    # local only
```

### `flowcmd validate <file.yml>`

Parse and schema-check a workflow without running it. Exits non-zero on any error — good for pre-commit hooks and CI.

```sh
flowcmd validate ./.flowcmd/commit.yml
```

---

## What flowcmd does not do

Worth being explicit:

- **It does not ship an LLM.** flowcmd is a runner. It executes commands. If a command happens to call a model, that model comes from your system, not from flowcmd.
- **It does not manage secrets.** Steps inherit your shell environment. Use `.env` files or your OS keychain, the same way any other tool would.
- **It does not run in the cloud.** This is a local CLI. You can invoke it from CI pipelines if you want.
- **It does not keep state between runs.** Each invocation is independent. Steps communicate through templates *within* a run, not across runs.
- **It does not interpret step output.** Your step ran, it printed something, flowcmd captured it. What it means is up to the next step.

## A few worked examples

Copy any of these into `./.flowcmd/` and run them. Substitute `my-llm` with whichever LLM CLI you use.

### Pre-push check

```yaml
name: Pre-push
steps:
  - name: lint
    run: my-linter
    parallel: true
  - name: test
    run: my-test-runner
    parallel: true
  - name: ok
    run: echo "ready to push"
```

### PR review

```yaml
name: PR review
steps:
  - name: diff
    run: git diff origin/main...HEAD

  - name: review
    run: |
      my-llm "Review this diff. List bugs, missing tests, unclear names:
      {{ steps.diff.output }}"

  - name: show
    run: echo "{{ steps.review.output }}"
```

### Flake investigator

```yaml
name: Flake hunt
steps:
  - name: run
    run: my-test-runner --run TestThing --count=10

  - name: analyze
    when: "{{ steps.run.exitcode != 0 }}"
    run: |
      my-llm "This test output looks flaky. What might cause it?
      {{ steps.run.error }}"
```

### Issue triage

```yaml
name: Triage
steps:
  - name: body
    run: gh issue view "$ISSUE" --json body -q .body

  - name: classify
    run: |
      my-llm "Classify as bug/feature/question/duplicate:
      {{ steps.body.output }}"

  - name: label
    run: gh issue edit "$ISSUE" --add-label "{{ steps.classify.output }}"
```

### Release notes

```yaml
name: Release notes
steps:
  - name: log
    run: git log --oneline $(git describe --tags --abbrev=0)..HEAD

  - name: draft
    run: |
      my-llm "Write human-readable release notes from this commit log:
      {{ steps.log.output }}"

  - name: save
    run: echo "{{ steps.draft.output }}" > NOTES.md
```

---

## Validation and error behavior

- `name`, `steps`, and each step's `name` + `run` are required.
- Step names match `^[a-z][a-z0-9_-]*$`, max 64 chars, unique within the workflow.
- Template references must point to **earlier** steps. Forward references are caught at validation time, not at runtime.
- On any step failure, the workflow stops. If the failed step is in a parallel group, its siblings are cancelled.
- On a template resolution error (e.g. referencing a step that hasn't run), the workflow stops with a clear message pointing to the template.

## Frequently asked

**Why YAML?**
Because a workflow is a list of steps, and YAML is the shortest honest way to write a list of steps that humans can read and diff. If you need a real scripting language, put it in the `run:` of a step.

**Is there state between runs?**
No. Each run is fresh. If you need persistent state, write it to a file in one run and read it in the next.

**Can a step read stdin?**
Steps run with stdin pointed at `/dev/null`. If you need interactivity, run the interactive command directly from your shell. flowcmd is for automation.

**Does it run on Windows?**
The binary cross-compiles. Steps use `sh -c`, so they assume a POSIX shell. WSL works.

**What if I want the model to drive the whole workflow?**
Then flowcmd isn't what you want — you want an agent runtime. flowcmd's whole premise is that the workflow drives and the model is a tool the workflow reaches for. That's a deliberate trade.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

Quick dev loop:

```sh
make test     # unit + integration tests
make race     # with race detector
make lint     # golangci-lint
make build    # binary at bin/flowcmd
```

Report a bug: [open an issue](https://github.com/flowcmd/cli/issues/new?template=bug_report.md).
Suggest a feature: [feature request](https://github.com/flowcmd/cli/issues/new?template=feature_request.md).

## License

[MIT](LICENSE) © The flowcmd authors.
