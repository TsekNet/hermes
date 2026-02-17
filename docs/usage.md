# Usage

## Service mode (recommended)

Start the per-user service daemon, then submit notifications via CLI. The service manages the full lifecycle: UI launch, deferrals, and deadlines.

```bash
# Start the per-user service daemon (see architecture.md for autostart setup)
hermes serve

# Send a notification (blocks until user responds)
hermes notify '{"heading":"Restart Required","message":"Please restart."}'

# Or use --config flag on root command
hermes --config notification.json

# List active notifications
hermes list

# Cancel a notification
hermes cancel <notification-id>
```

## Local mode

For testing or single-session use, render directly without the service:

```bash
hermes --local '{"heading":"Test","message":"Local test."}'
hermes --local notification.json
echo '{"heading":"..."}' | hermes --local
```

## JSON config

hermes accepts a single JSON object with these fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `heading` | string | **yes** | | Bold heading text |
| `message` | string | **yes** | | Body text below the heading |
| `buttons` | array | no | `[]` | Button definitions (see below) |
| `timeout` | int | no | `300` | Seconds until auto-action |
| `timeoutValue` | string | no | `""` | Value returned on timeout |
| `escValue` | string | no | `""` | Value returned on ESC (defaults to `timeoutValue`) |
| `title` | string | no | `Notification` | Small uppercase label at the top |
| `accentColor` | string | no | `#D4AF37` | Theme accent color (hex) |
| `helpUrl` | string | no | `""` | "Need help?" link URL |
| `id` | string | no | auto-generated | Unique notification ID for the service |
| `deferDeadline` | string | no | `""` | Max deferral window (e.g., `"24h"`, `"7d"`) |
| `maxDefers` | int | no | `0` | Max number of deferrals (0 = unlimited) |

### Button format

**Simple button:**

```json
{"label": "Restart Now", "value": "restart", "style": "primary"}
```

**Dropdown button** (fly-out menu above the button):

```json
{
  "label": "Defer",
  "style": "secondary",
  "dropdown": [
    {"label": "1 Hour", "value": "defer_1h"},
    {"label": "4 Hours", "value": "defer_4h"},
    {"label": "1 Day", "value": "defer_1d"}
  ]
}
```

**Styles:** `primary` (accent color), `secondary` (dark with border), `danger` (red).

### Deferral config

When using the service daemon, configure deferrals to control how long and how many times a user can defer:

```json
{
  "heading": "System Restart Required",
  "message": "Your computer needs to restart to apply security updates.",
  "timeout": 300,
  "timeoutValue": "restart",
  "deferDeadline": "24h",
  "maxDefers": 5,
  "buttons": [
    {"label": "Defer 1h", "value": "defer_1h", "style": "secondary"},
    {"label": "Defer 4h", "value": "defer_4h", "style": "secondary"},
    {"label": "Restart Now", "value": "restart", "style": "primary"}
  ]
}
```

Defer values must match the pattern `defer_Xh`, `defer_Xd`, `defer_Xm` or `defer_Xs` (hours, days, minutes, seconds). The service parses these to schedule re-notification. Deferral state is persisted to disk so notifications survive service restarts (see [architecture.md](architecture.md#persistence)).

When `maxDefers` is reached or `deferDeadline` has passed, hermes automatically hides any buttons (or dropdown options) that trigger a deferral. If a button has no other action (e.g. it was purely a defer button), it is removed entirely. This forces the user to choose a non-deferral action (e.g. "Restart Now") or let the timeout expire.

## Subcommands

| Command | Description |
|---------|-------------|
| `hermes serve` | Start the gRPC service daemon |
| `hermes notify [config]` | Send notification to service (blocks for result) |
| `hermes list` | List active notifications |
| `hermes cancel <id>` | Cancel an active notification |
| `hermes demo` | Show a demo notification |
| `hermes version` | Print version, build date, Go, and OS info |

## Flags

| Flag | Scope | Description |
|------|-------|-------------|
| `--config <json>` | root | JSON config (file path or inline) — routes to service |
| `--local` | root | Render locally in current session (skip service) |
| `--port <int>` | serve, notify, list, cancel | gRPC port (default: 4770) |
| `--db <path>` | serve | Bolt database path (default: platform-specific, see [architecture.md](architecture.md#persistence)) |
| `--help` | all | Print help |

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | User chose an action (response on stdout) or dismissed (empty stdout) |
| `1` | Error (bad config, validation, or launch failure) |
| `200` | User deferred (response on stdout, starts with `defer`) |
| `202` | Timeout (countdown expired, auto-actioned per config) |

**Detecting dismissals:** Exit `0` with **empty stdout** means the user dismissed
the notification (ESC / window close) without choosing an action. Scripts should
check both the exit code and stdout content.

## Input methods

hermes auto-detects how you're providing the config:

### File path

```bash
hermes notify restart-notification.json
hermes --config restart-notification.json
```

### Inline JSON

```bash
hermes notify '{"heading":"Update","message":"Please restart."}'
```

### Stdin pipe

```bash
echo '{"heading":"Update","message":"Please restart."}' | hermes notify
```

### PowerShell (recommended: pipe via stdin)

PowerShell 5.1 strips inner double quotes when passing strings to native
executables. Piping via stdin avoids this entirely:

```powershell
$config = @'
{
  "heading": "System Restart Required",
  "message": "Your computer needs to restart.",
  "timeout": 300,
  "timeoutValue": "restart",
  "deferDeadline": "24h",
  "maxDefers": 3,
  "buttons": [
    {"label": "Defer 1h", "value": "defer_1h", "style": "secondary"},
    {"label": "Restart Now", "value": "restart", "style": "primary"}
  ]
}
'@

$config | & hermes.exe notify
```

## Example templates

See `testdata/` for ready-to-use JSON configs:

- `restart-notification.json` — Restart with defer dropdown
- `update-notification.json` — Software update with defer
- `simple-notification.json` — Simple one-button acknowledgment
- `defer-with-dropdown.json` — VPN disconnect with defer dropdown
- `short-defer-restart.json` — Short deferral (2m deadline, 3 max) for quick lifecycle testing
- `short-defer-deadline.json` — Very short deadline (1m) for testing auto-action
