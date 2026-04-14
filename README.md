# flowcmd

[![CI](https://github.com/flowcmd/cli/actions/workflows/ci.yml/badge.svg)](https://github.com/flowcmd/cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/flowcmd/cli.svg)](https://pkg.go.dev/github.com/flowcmd/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Taskfile for the AI era. Run YAML-defined workflows with sequential & parallel shell steps and LLM calls as first-class citizens.

## Install

```sh
go install github.com/flowcmd/cli@latest
# or
git clone https://github.com/flowcmd/cli && cd cli && make build
```

## Usage

```sh
flowcmd init                          # bootstrap ./.flowcmd/hello.yml
flowcmd add <path-or-url>             # install into ./.flowcmd
flowcmd add -g <path-or-url>          # install into ~/.flowcmd (global)
flowcmd add <path> --as <name>        # install under a different filename
flowcmd remove <name> [-g]            # delete from local (default) or global
flowcmd list                          # show both scopes (global then local)
flowcmd list -g                       # global only
flowcmd list -l                       # local only
flowcmd run hello                     # run by name (resolves through scopes)
flowcmd run ./path/to/workflow.yml    # or by file path
flowcmd run --no-tui hello            # plain output (CI / piping)
flowcmd run --dry-run hello
flowcmd run --verbose hello
flowcmd validate workflow.yml
```

### Scopes

| Scope | Directory |
|---|---|
| local | `./.flowcmd/` |
| global | `~/.flowcmd/` |

Workflow identity is the filename stem (`hello.yml` → `hello`). Names must be unique within a scope; the same name may exist in both.

### Resolution

`flowcmd run <arg>` treats `<arg>` as a **file path** when it contains `/` / `\` or ends in `.yml` / `.yaml`. Otherwise it's a **name**, resolved as:

1. `./.flowcmd/<name>.yml` (local) — if present, used.
2. `~/.flowcmd/<name>.yml` (global) — used if local has no match.
3. If both have it, **local wins** and a warning is printed to stderr.
4. If neither has it, the command exits with an error listing both scopes.

## Workflow syntax

```yaml
name: Commit Agent
description: Analyzes changes and commits with an AI-generated message.

steps:
  - name: identify-changes
    description: Get staged changes
    run: git diff --cached

  - name: generate-message
    description: AI-generated commit message
    run: |
      claude -p "Write a commit message for: {{ steps.identify-changes.output }}"
    retry:
      attempts: 2
      delay: 5s

  - name: commit
    run: git commit -m "{{ steps.generate-message.output }}"
    when: "{{ steps.identify-changes.output != '' }}"

  - name: lint
    run: golangci-lint run
    parallel: true

  - name: test
    run: go test ./...
    parallel: true
```

### Template refs

| Syntax | Example |
|---|---|
| Named | `{{ steps.identify-changes.output }}` |
| Indexed | `{{ steps[0].output }}` |
| Properties | `.output`, `.error`, `.exitcode` |
| Comparisons (in `when`) | `{{ steps.foo.output != '' }}` |

### Schema rules

- Steps run sequentially by default. Adjacent `parallel: true` steps form a concurrent group.
- Step names: `^[a-z][a-z0-9_-]*$`, max 64 chars, unique.
- `run` is executed via `sh -c`.
- `when` skips the step if it evaluates to false/empty.
- `retry.attempts` re-executes on non-zero exit; `retry.delay` waits between attempts.
- Templates resolve at execution time and must reference only earlier steps.

### Error behavior

- Any step failure → fail-fast stop.
- Parallel group failure → remaining running steps are cancelled via context.
- Template resolution error → workflow stops with a clear message.

## Develop

```sh
make test        # unit + integration tests
make race        # with race detector
make build
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

[MIT](LICENSE) © The flowcmd authors.
