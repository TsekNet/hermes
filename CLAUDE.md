# Hermes

Cross-platform notification daemon: per-user gRPC service + Wails v2 webview UI. One binary, consistent UX across Windows, macOS, Linux.

## Architecture

Read `docs/architecture.md` for the full design. Key facts for code changes:

- Service model: `hermes serve` runs per-user, gRPC over Unix domain socket. CLI and Wails UI are gRPC clients
- Config format: `docs/usage.md`. Subcommands, flags, exit codes also documented there
- Platform details: `docs/platforms.md`. Build/test workflow: `docs/development.md`

## Build and test

```bash
# Vet (Linux requires build tag)
go vet -tags webkit2_41 ./...

# Test
go test -v -race -count=1 -tags webkit2_41 ./internal/... ./cmd/...

# E2E tests (requires Playwright: go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium)
go test -v -count=1 -timeout=180s -tags "e2e,webkit2_41" ./e2e/

# Update visual regression baselines
UPDATE_GOLDEN=1 go test -v -count=1 -tags "e2e,webkit2_41" -run=Visual ./e2e/

# Build (requires wails CLI: go install github.com/wailsapp/wails/v2/cmd/wails@latest)
wails build -skipbindings -platform linux/amd64 -tags webkit2_41 \
  -ldflags "-s -w -X github.com/TsekNet/hermes/cmd.Version=dev"

# Windows (requires -windowsconsole for CLI stdout/exit codes)
wails build -skipbindings -platform windows/amd64 -windowsconsole \
  -ldflags "-s -w -X github.com/TsekNet/hermes/cmd.Version=dev"
```

Linux CI needs `libgtk-3-dev libwebkit2gtk-4.1-dev`. The `-tags webkit2_41` flag is Linux-only (GTK/WebKit compat). The `-windowsconsole` flag is Windows-only (without it, stdout is lost and exit codes are always 0).

## Platform-specific code

When modifying platform-specific code (`_windows.go`, `_unix.go`, `_darwin.go`, `_linux.go`), always verify implementations exist for all three OSes. Project uses both suffix-based naming and `//go:build` directives.

## Proto/gRPC

- Source: `proto/hermes.proto`. Generated files live in `proto/` (Go package alias: `hermespb`)
- Regenerate: `protoc --go_out=. --go-grpc_out=. --go_opt=paths=source_relative --go-grpc_opt=paths=source_relative proto/hermes.proto`
- Never renumber or remove existing proto fields, only deprecate and add new ones

## Conventions

- `go.mod` has a local `replace` directive for development, do not commit changes to it unless intentional
- Release: tag-triggered (`v*`), see `.github/workflows/release.yml`

## Agents

Four agents live in `.claude/agents/`. Review agents expect `<diff>` and `<conventions>` XML tags as input from the caller. Use them when changes touch their domain:

| Agent | Trigger files |
| --- | --- |
| `platform-reviewer` | `_windows.go`, `_unix.go`, `_darwin.go`, `_linux.go`, `build/`, `runtime.GOOS` branches |
| `proto-guardian` | `proto/`, `internal/server`, `internal/client` |
| `security-auditor` | `internal/action`, `internal/auth`, `internal/ratelimit`, `internal/server`, config changes |
| `integration-tester` | After code changes, runs `testdata/*.json` fixtures against a local binary, validates exit codes |
