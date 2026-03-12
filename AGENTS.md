# AGENTS.md — Project Conventions for Coding Agents

> This file describes project structure, conventions, and development workflow
> for AI coding agents working on this codebase.

---

## Project Overview

LINE event-wall scraper and notification system.
Fetches LINE promotional activities via JSON API, parses detail pages for
keywords, and sends daily task summaries via Discord and Email.

**Language**: Go (Golang)
**Architecture**: `golang-standards/project-layout`

---

## Directory Structure

| Path                 | Purpose                                                             |
| -------------------- | ------------------------------------------------------------------- |
| `cmd/scheduler/`     | CLI entry point: `main.go` + `cli/` sub-package                     |
| `cmd/scheduler/cli/` | Cobra subcommands (`root.go`, `init.go`, `sync.go`, etc.)           |
| `cmd/bot/`           | Discord Bot WebSocket Gateway entry point                           |
| `internal/`          | Private application logic (see design docs for module details)      |
| `config/`            | Runtime configuration files (`config.yaml`, `channel_mapping.yaml`) |
| `data/`              | SQLite database file (`line_tasks.db`, tracked in Git)              |
| `docs/requirements/` | Requirements specification                                          |
| `docs/design/`       | Detailed design documents (per development phase)                   |
| `docs/guides/`       | Detailed guides for CI/CD and maintenance                           |
| `logs/`              | Test reports, coverage output, execution logs                       |
| `.local-dev/`        | Temporary development files (not committed)                         |

---

## Toolchain (mise)

This project uses [mise](https://mise.jdx.dev/) for toolchain and task management.

```bash
# Install dependencies
mise install

# Common tasks
mise run test    # Run tests with coverage → logs/coverage.out
mise run lint    # Run golangci-lint v2
mise run fmt     # Format with gofumpt
mise run build   # Build binary → bin/scheduler
```

## Run Application

```bash
# Init DB
./bin/scheduler init --config ./config/config.yaml

# Sync remote line events
./bin/scheduler sync --config ./config/config.yaml

# Notify tomorrow's tasks
./bin/scheduler notify --config ./config/config.yaml

# Notify specified date tasks
./bin/scheduler notify --config ./config/config.yaml --date {YYYY-MM-DD}
```

---

## Code Style

- **Formatter**: `gofumpt` (stricter than `gofmt`)
- **Linter**: `golangci-lint` v2 (config in `.golangci.yml`)
- **Imports**: Group order: stdlib → 3rd-party → local
- **Comments**: All code comments in **English**
- **Error wrapping**: Always use `fmt.Errorf("failed to <action>: %w", err)`
- **Context**: All I/O functions take `ctx context.Context` as first parameter

---

## Testing Conventions

- **Table-Driven Tests**: All unit tests use `cases := []struct{...}` pattern
- **Subtests**: Use `t.Run(tc.name, ...)` for each case
- **Mocking**: External dependencies (HTTP, DB) are defined as interfaces
- **Coverage target**: ≥ 90%
- **Coverage output**: `logs/coverage.out`
- **No third-party test frameworks**: Use stdlib `testing` + `net/http/httptest`

---

## Integration / End-to-End Test Conventions

- **Test scripts**: Use go `txtar`
- **Test file path**:
  - Integration: `tests/integration/`
  - End-to-End: `tests/e2e/`
- **Test fixture**:
  - API payloads: `tests/fixture/api/`
  - configs: `tests/fixture/config/`
  - db sql: `tests/fixture/db/`
  - html parsing: `tests/fixture/html/`
- **Test plan and cases**: `docs/test/test-plan.md`
- **Test output**:
  - `logs/integration_test_report.log`
  - `logs/e2e_test_report.log`
- **Test helpers**: `tests/helpers/testmain_test.go`

---

## Database Conventions

- **Database file**: `data/line_tasks.db`
- **Driver**: `modernc.org/sqlite` (pure Go, no CGO)
- **Schema**: Managed via `internal/storage/schema.go`

---

## Git Conventions

- **Commit messages**: English, following [Conventional Commits](https://www.conventionalcommits.org/)
  - `feat: add activity sync L1 hash`
  - `fix: handle empty API response`
  - `test: add storage upsert test cases`
  - `docs: update design-part1`
- **Do NOT modify** `git config --global`

---

## Important Notes

- `data/line_tasks.db` is tracked in Git (commit-back mechanism via GitHub Actions)
- Config values support `${ENV_VAR}` expansion (e.g., `${DISCORD_BOT_TOKEN}`)
- CLI uses `cobra` framework; each subcommand is a separate file in `cmd/scheduler/cli/`
- API response JSON structure should be verified via `curl` before implementing parsers
- SQLite uses `modernc.org/sqlite` (pure Go, no CGO required)
