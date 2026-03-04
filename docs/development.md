# Development

How to build, test, and iterate on notifications for each platform.

---

## Prerequisites

| Tool | Install |
|------|---------|
| [Go](https://go.dev/) | `brew install go` / `winget install GoLang.Go` / `apt install golang` |
| [Wails CLI](https://wails.io) | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| [protoc](https://grpc.io/docs/protoc-installation/) | Only needed if you edit `proto/hermes.proto` |

Platform-specific webview requirements:

| Platform | Webview engine | Dev dependency |
|----------|---------------|----------------|
| Windows | WebView2 | Included with Windows 10+ (Edge) |
| macOS | WKWebView | Included with macOS |
| Linux | WebKitGTK | `apt install libwebkit2gtk-4.1-dev` |

---

## Building

### macOS / Linux

```bash
# macOS
wails build

# Linux (requires webkit2_41 build tag for libwebkit2gtk-4.1)
wails build -tags webkit2_41
```

### Windows

```bash
wails build -windowsconsole
```

> **`-windowsconsole` is required for CLI output.** Without it, Wails produces a GUI-subsystem executable (no attached console). `--help` prints nothing, stdout is lost, and `$LASTEXITCODE` is always 0 in PowerShell.
>
> However, for a silent background notification agent (no popping up black console windows), you might prefer **omitting** `-windowsconsole`. In that case, you cannot rely on stdout for the user's choice — you must use the service daemon (gRPC) or file-based IPC if you need the result.

### Cross-compile from Linux

```bash
wails build -platform windows/amd64 -windowsconsole
```

---

## Dev mode (live reload)

Wails has a dev mode that hot-reloads the frontend and rebuilds Go on change:

```bash
wails dev
```

This opens a webview that auto-refreshes when you edit HTML/CSS/JS in `frontend/`. Go changes trigger a rebuild. Useful for iterating on notification layout and styling.

---

## Testing notifications

### Quick iteration loop

The fastest way to iterate is `--local` mode — no service required, just renders the UI directly:

```bash
# Inline JSON
hermes --local '{"heading":"Test","message":"Quick test."}'

# From a file (edit, save, re-run)
hermes --local testdata/restart-notification.json

# Pipe from stdin
echo '{"heading":"Test","message":"Piped."}' | hermes --local
```

### With the service daemon

Start the service in one terminal, send notifications from another:

```bash
# Terminal 1: start service
hermes serve

# Terminal 2: send notifications
hermes notify testdata/restart-notification.json
hermes list
hermes cancel <id>
```

### Platform-specific testing

#### Windows

Build and copy the binary to a Windows machine (or use cross-compile):

```powershell
# Quick test
& .\hermes.exe --local '{"heading":"Test","message":"Windows test."}'

# From file
& .\hermes.exe --local .\testdata\restart-notification.json

# Pipe via stdin (recommended for complex JSON)
$config = @'
{
  "heading": "System Restart Required",
  "message": "Your computer needs to restart.",
  "timeout": 300,
  "timeoutValue": "restart",
  "buttons": [
    {"label": "Defer 1h", "value": "defer_1h", "style": "secondary"},
    {"label": "Restart Now", "value": "restart", "style": "primary"}
  ]
}
'@
$config | & .\hermes.exe --local

# Check exit code
Write-Host "Exit: $LASTEXITCODE"
```

> **WSL Tip:** You can run the Windows binary directly from WSL if you copy it to a Windows path (e.g. `C:\Temp`).
>
> ```bash
> # Build (cross-compile)
> wails build -platform windows/amd64 -windowsconsole
>
> # Copy to Windows temp
> cp build/bin/hermes.exe /mnt/c/Temp/hermes.exe
>
> # Run via powershell.exe
> powershell.exe -Command "& 'C:\Temp\hermes.exe' --local '{\"heading\":\"WSL Test\",\"message\":\"Hello from WSL\"}'"
> ```

> PowerShell 5.1 strips inner double quotes when passing strings to native executables. Always pipe via stdin for complex JSON.

#### macOS

```bash
# Build natively
wails build

# Test
./build/bin/hermes --local testdata/restart-notification.json

# Test with service
./build/bin/hermes serve &
./build/bin/hermes notify testdata/restart-notification.json
```

Notifications appear in the top-right corner (matching macOS notification behavior).

#### Linux

Requires a display server (X11 or Wayland with XWayland):

```bash
# Build
wails build

# Test (needs DISPLAY set)
./build/bin/hermes --local testdata/restart-notification.json

# On Wayland, hermes auto-sets GDK_BACKEND=x11 for window positioning
```

Notifications appear in the top-right corner.

#### Linux (headless / WSL)

WSL doesn't have a display server by default. Options:

1. **WSLg** (Windows 11) — GUI apps work out of the box. Just run `hermes --local ...`.
2. **VcXsrv / X410** — Install an X server on Windows, set `export DISPLAY=:0`.
3. **Service-only testing** — Skip the UI, test the gRPC lifecycle:

```bash
hermes serve &
hermes notify '{"heading":"Test","message":"WSL test."}' &
hermes list
hermes cancel <id>
```

---

## Testing the JSON config

Use the bundled templates in `testdata/` as starting points:

| File | Scenario |
|------|----------|
| `restart-notification.json` | Restart with defer dropdown |
| `update-notification.json` | Software update with defer |
| `simple-notification.json` | One-button acknowledgment |
| `defer-with-dropdown.json` | VPN disconnect with defer menu |
| `short-defer-restart.json` | Short deferral (2m deadline, 3 max) for quick lifecycle testing |
| `short-defer-deadline.json` | Very short deadline (1m) for testing auto-action |
| `image-carousel.json` | Multi-slide image carousel |
| `install-with-watch.json` | Filesystem watch for install receipt validation |

Edit a template, run it, tweak, repeat:

```bash
# Edit
vim testdata/restart-notification.json

# Test
hermes --local testdata/restart-notification.json

# Check what the user chose
echo "User chose: $(hermes --local testdata/restart-notification.json)"
echo "Exit code: $?"
```

---

## Testing deferrals

Deferrals require the service daemon. State is persisted to a local bbolt database, so notifications survive service restarts.

```bash
hermes serve &

# Notification with 5-minute deadline and max 2 defers
hermes notify '{
  "heading": "Restart Required",
  "message": "Restart to apply updates.",
  "deferDeadline": "5m",
  "maxDefers": 2,
  "timeoutValue": "restart",
  "buttons": [
    {"label": "Defer 1m", "value": "defer_1m", "style": "secondary"},
    {"label": "Restart", "value": "restart", "style": "primary"}
  ]
}'

# Watch the notification lifecycle
watch hermes list
```

After 2 defers or 5 minutes (whichever comes first), the notification auto-actions with `timeoutValue`.

### Testing persistence across restarts

```bash
# 1. Start service, send a deferrable notification, defer it
hermes serve &
hermes notify '{"heading":"Persist Test","message":"Defer me.","deferDeadline":"1h","buttons":[{"label":"Defer 5m","value":"defer_5m","style":"secondary"},{"label":"OK","value":"ok","style":"primary"}]}' &
# Click "Defer 5m" in the UI

# 2. Kill the service
kill %1

# 3. Restart — the deferred notification reappears immediately
hermes serve
```

Override the database path with `--db` for isolated testing:

```bash
hermes serve --db /tmp/hermes-test.db
```

---

## Running tests

```bash
# All internal package tests (no display server needed)
# On Linux, add -tags webkit2_41 for WebKitGTK 4.1 compatibility
go test -race -tags webkit2_41 ./internal/...

# Specific package
go test -race -tags webkit2_41 ./internal/manager/   # includes persistence tests
go test -race -tags webkit2_41 ./internal/store/      # bbolt store tests
go test -race -tags webkit2_41 ./internal/config/
go test -race -tags webkit2_41 ./internal/server/

# Vet
go vet -tags webkit2_41 ./...
```

> **Note:** The `-tags webkit2_41` flag is only required on Linux (Ubuntu 24.04+) where `libwebkit2gtk-4.1-dev` replaces the older `4.0` package. On macOS and Windows, the flag is harmless but unnecessary.

---

## Regenerating protobuf code

Only needed if you edit `proto/hermes.proto`:

```bash
protoc --go_out=. --go-grpc_out=. proto/hermes.proto
```

Requires `protoc-gen-go` and `protoc-gen-go-grpc`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

---

## Modifying the frontend

The frontend lives in `frontend/` — plain HTML/CSS/JS, no build step:

| File | Purpose |
|------|---------|
| `index.html` | Notification layout |
| `style.css` | Dark theme, CSS custom properties (`--accent`) |
| `main.js` | Countdown timer, button handling, Wails bindings |

The accent color is set dynamically from the JSON config's `accentColor` field. To preview different themes:

```bash
hermes --local '{"heading":"Blue","message":"Test","accentColor":"#0078D4"}'
hermes --local '{"heading":"Red","message":"Test","accentColor":"#E74C3C"}'
hermes --local '{"heading":"Green","message":"Test","accentColor":"#27AE60"}'
```
