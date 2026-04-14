# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-04-14

### Added
- `install.sh` (Linux / macOS) and `install.ps1` (Windows) for curl/irm-based installation; no Go toolchain required on the user's machine
- `flowcmd update` subcommand (with `--check` and `--version` flags) for in-place upgrades
- YAML workflow runner with sequential and parallel step execution
- Template interpolation across step outputs (`{{ steps.name.output }}`, `{{ steps[0].output }}`, `.output`/`.error`/`.exitcode`)
- `when` conditionals for step skipping
- `retry` with configurable attempts and delay
- Bubbletea TUI with status icons, spinner, parallel-group indicator, elapsed time, and terminal-width-aware stdout preview
- `--no-tui` plain output for CI/piping, `--dry-run` execution plan preview, `--verbose` inline stdout
- Commands: `init`, `add`, `remove`, `list`, `run`, `validate`
- Local (`./.flowcmd/`) and global (`~/.flowcmd/`) workflow scopes
- `flowcmd run <name>` resolves against local scope first, then global, with dup warning
- `flowcmd add <path-or-url>` installs from local files or http(s) URLs (1 MiB cap), with `--as` rename and `--force` overwrite
- Fail-fast parallel group cancellation via shared child context
- Forward-reference detection in template validation

[Unreleased]: https://github.com/flowcmd/cli/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/flowcmd/cli/releases/tag/v0.1.0
