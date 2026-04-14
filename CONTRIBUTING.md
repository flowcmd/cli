# Contributing to flowcmd

Thanks for your interest in contributing!

## Reporting bugs

Open an issue with the **Bug report** template. Please include:

- `flowcmd --version` output (once releases exist) or the commit SHA
- Minimal workflow YAML that reproduces the problem
- Expected vs actual behavior
- OS and Go version

## Suggesting features

Open an issue with the **Feature request** template. Describe the problem first, then the proposed solution. Alternatives considered are welcome.

## Development setup

```sh
git clone https://github.com/flowcmd/cli && cd cli
make test      # unit + integration tests
make race      # tests with the race detector
make lint      # golangci-lint
make build     # bin/flowcmd
```

Requires Go 1.23+.

## Code conventions

- `internal/` holds all non-exported packages
- Return errors, don't panic
- Table-driven tests in `*_test.go` next to source
- Use `context.Context` for cancellation
- Keep comments to WHY, not WHAT
- Run `make lint` before pushing

## Pull requests

- Keep PRs focused — one logical change per PR
- Include tests for new behavior and bug fixes
- Update `README.md` / `CHANGELOG.md` when user-visible behavior changes
- CI must be green before review

## Running a single test

```sh
go test -race -run TestEngine_ParallelExecutesConcurrently ./internal/engine/
```

## Coverage

```sh
make cover
go tool cover -html=coverage.out
```

Current coverage: ~98%. Keep it high — new code should come with tests.
