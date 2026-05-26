# Usage

## Service mode (recommended)

Start the per-user service daemon, then submit notifications via CLI. The service manages the full lifecycle: UI launch, deferrals, and deadlines.

```bash
# Start the per-user service daemon with system tray icon
hermes serve

# Start without tray icon (headless, SSH-only, CI)
hermes serve --no-tray

# Send a notification (blocks until user responds)
hermes notify '{"heading":"Restart Required","message":"Please restart."}'

# Or use --config flag on root command
hermes --config notification.json

# List active notifications
hermes list

# Cancel a notification
hermes cancel <notification-id>

# View notification history
hermes history

# Print history as JSON
hermes history --json
```

---

## History (notification history)

The history view shows all notifications: active items that need your attention appear at the top with inline action buttons, completed notifications appear below in a grayed-out style. Clicking an inline button (e.g. "Restart Now") sends the response to the service and immediately grays out the card. The history view polls for updates every 3 seconds, so actions taken in popup windows are reflected automatically.

```bash
hermes history              # Opens the history UI
hermes history --json       # Prints history as JSON to stdout
hermes history --db my.db   # Read directly from a bolt DB file (skip service)
```

The history view connects to the running service via gRPC. If the service is unreachable, it falls back to reading the bolt database directly. History is auto-pruned on service startup: records older than 30 days or exceeding 50 entries are removed.

### App launcher shortcut

Platform installers create a **Hermes Notifications** shortcut so users can find the history view without knowing the CLI:

| Platform | Location | Search keywords |
|---|---|---|
| Windows | Start Menu > Hermes > Hermes Notifications | "hermes", "notifications" |
| macOS | /Applications/Hermes Notifications.app | Spotlight: "hermes", "notifications" |
| Linux | GNOME Activities / KDE krunner | "hermes", "notifications", "alerts", "history" |

On Windows, the shortcut uses `conhost --headless` to suppress the console window that would otherwise appear because the binary is built with `-windowsconsole` for CLI compatibility.

---

## Local mode

For testing or single-session use, render directly without the service:

```bash
hermes --local '{"heading":"Test","message":"Local test."}'
hermes --local notification.json
echo '{"heading":"..."}' | hermes --local
```

---

## Config format

hermes accepts a single JSON or YAML config with these fields:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `heading` | string | **yes** | | Bold heading text |
| `message` | string | **yes** | | Body text below the heading |
| `buttons` | array | no | `[]` | Button definitions (see below) |
| `timeout` | int | no | `300` | Seconds until auto-action |
| `timeout_value` | string | no | `""` | Value returned on timeout |
| `esc_value` | string | no | `""` | Value returned on ESC (defaults to `timeout_value` when set, otherwise empty) |
| `title` | string | no | `IT Department` | Small uppercase label at the top |
| `accent_color` | string | no | `#D4A843` | Theme accent color (hex) |
| `help_url` | string | no | `""` | "Need help?" link URL |
| `id` | string | no | auto-generated | Unique notification ID for the service |
| `defer_deadline` | string | no | `""` | Max deferral window (e.g., `"24h"`, `"7d"`) |
| `max_defers` | int | no | `0` | Max number of deferrals (0 = unlimited) |
| `images` | array | no | `[]` | HTTPS URLs or `data:image/` URIs for a carousel (max 5, no SVG data URIs) |
| `watch_paths` | array | no | `[]` | Filesystem paths to monitor for changes (max 5, no `..` traversal) |
| `dnd` | string | no | `"respect"` | Do Not Disturb behavior: `"respect"`, `"ignore"`, or `"skip"` |
| `priority` | int | no | `5` | Delivery priority (0-10). Higher = shown first in queue drain |
| `escalation` | array | no | `[]` | Progressive urgency steps applied after repeated deferrals (see below) |
| `result_actions` | object | no | `{}` | Maps response values to automatic actions (max 10 entries; action chaining, see below) |
| `quiet_hours` | object | no | `null` | Time-based delivery suppression (see below) |
| `heading_localized` | object | no | `{}` | Locale → heading text map for i18n |
| `message_localized` | object | no | `{}` | Locale → message text map for i18n |
| `depends_on` | string | no | `""` | ID of notification that must complete first (sequential workflows) |

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

### URI buttons

Button values prefixed with `uri:` open the URI in the default system handler instead of closing the notification. Only schemes on the allowlist are permitted. The canonical list lives in [`internal/action/action.go`](../internal/action/action.go) (`allowedSchemes` variable). Schemes not on this list are rejected at validation time.

Platform-specific schemes (e.g. `ms-settings:`) are delegated to the OS default handler. A `ms-settings:` button on macOS will fail gracefully since the OS has no handler registered.

```json
{"label": "Windows Update", "value": "uri:ms-settings:windowsupdate", "style": "primary"}
{"label": "Software Update", "value": "uri:x-apple.systempreferences:com.apple.Software-Update-Settings.extension", "style": "primary"}
{"label": "FileVault", "value": "uri:x-apple.systempreferences:com.apple.preference.security?FileVault", "style": "secondary"}
```

macOS pane IDs follow the pattern `com.apple.preference.<name>` or `com.apple.<Name>-Settings.extension`. Append `?Anchor` for sub-panes (e.g. `?FileVault`, `?Privacy_AllFiles`). Linux has no standard settings URI scheme.

### Action buttons

Button values prefixed with `action:` execute a built-in verb. No shell is invoked, no user input reaches the argument vector. The canonical verb list lives in [`internal/action/action.go`](../internal/action/action.go) (`validVerbs` variable). Platform implementations are in the `builtin_*.go` files in the same package.

```json
{"label": "Restart Now", "value": "action:reboot", "style": "primary"}
{"label": "Lock Screen", "value": "action:lock", "style": "secondary"}
```

Shell execution from config is not supported. To run arbitrary commands on a notification result, read the response value from stdout in the calling script and dispatch externally. Only one primary button per notification is recommended, the Enter key triggers the first primary button.

### Deferral config

When using the service daemon, configure deferrals to control how long and how many times a user can defer:

```json
{
  "heading": "System Restart Required",
  "message": "Your computer needs to restart to apply security updates.",
  "timeout": 300,
  "timeout_value": "restart",
  "defer_deadline": "24h",
  "max_defers": 5,
  "buttons": [
    {"label": "Defer 1h", "value": "defer_1h", "style": "secondary"},
    {"label": "Defer 4h", "value": "defer_4h", "style": "secondary"},
    {"label": "Restart Now", "value": "restart", "style": "primary"}
  ]
}
```

Defer values must match the pattern `defer_Xh`, `defer_Xd`, `defer_Xm` or `defer_Xs` (hours, days, minutes, seconds). The service parses these to schedule re-notification. Deferral state is persisted to disk so notifications survive service restarts (see [Architecture — Persistence](architecture.md#persistence)).

When `max_defers` is reached or `defer_deadline` has passed, hermes automatically hides any buttons (or dropdown options) that trigger a deferral. If a button has no other action (e.g. it was purely a defer button), it is removed entirely. This forces the user to choose a non-deferral action (e.g. "Restart Now") or let the timeout expire.

### Image carousel

Embed images (documentation slides, screenshots, diagrams) in the notification. The window auto-sizes taller when images are present. Multiple images render as a carousel with arrow navigation and keyboard support (left/right arrow keys).

```json
{
  "heading": "macOS 15.3 Update",
  "message": "Review the changes below, then click Update.",
  "images": [
    "https://intranet.example.com/slides/macos-15.3-overview.png",
    "https://intranet.example.com/slides/macos-15.3-timeline.png",
    "data:image/png;base64,iVBORw0KGgo..."
  ],
  "buttons": [
    {"label": "Update Now", "value": "update", "style": "primary"},
    {"label": "Defer 4h", "value": "defer_4h", "style": "secondary"}
  ]
}
```

Images must be `https://` URLs or `data:image/` URIs (no SVG). Maximum 5 per notification. Recommended dimensions: **380x220px** (matches the carousel track). Images are scaled with `object-fit: contain`, so larger images work but may have letterboxing.

### Filesystem watch

Monitor filesystem paths for changes during the notification. When a watched path is created, modified, or deleted, the notification UI updates with the event. This is useful for validating installations (e.g. watch for a receipt file to appear after the user clicks "Install").

```json
{
  "heading": "Installing Security Agent",
  "message": "Click Install, then wait for confirmation.",
  "watch_paths": [
    "/var/db/receipts/com.example.agent.plist",
    "/Library/Application Support/SecurityAgent/version.txt"
  ],
  "timeout": 600,
  "buttons": [
    {"label": "Install", "value": "uri:https://intranet.example.com/install", "style": "primary"}
  ]
}
```

The notification footer shows "Monitoring filesystem..." initially, and a shimmer animation sweeps across the accent bar to indicate active watching. When a watched file event fires (e.g. "create: version.txt"), the shimmer stops permanently and the footer updates with the event. The accent bar continues shrinking as the countdown timer progresses. The frontend receives events via the Wails `fs:event` event channel, so custom frontends can also subscribe.

If a watched path doesn't exist yet, hermes watches its parent directory to catch creation events.

### Do Not Disturb

hermes detects the OS Do Not Disturb / Focus mode on all platforms and adjusts notification behavior accordingly. The `dnd` field controls what happens when DND is active:

| Mode | Behavior |
|------|----------|
| `"respect"` (default) | Wait and retry every 60 seconds until DND is off, then show the notification. Deadlines are still enforced while waiting. |
| `"ignore"` | Show the notification immediately regardless of DND. Use for critical security alerts. |
| `"skip"` | Silently complete with value `"dnd_active"` (exit code 0). The calling script can detect this and decide what to do. |

```json
{
  "heading": "Security Update Required",
  "message": "Critical vulnerability patch. This alert overrides Do Not Disturb.",
  "dnd": "ignore",
  "buttons": [
    {"label": "Update Now", "value": "update", "style": "primary"}
  ]
}
```

**Platform detection:**

| Platform | Method |
|----------|--------|
| Windows | `SHQueryUserNotificationState` Win32 API (detects Focus Assist, fullscreen apps, presentation mode, quiet hours) |
| macOS | `defaults read com.apple.controlcenter "NSStatusItem Visible FocusModes"` (Monterey+), falls back to `doNotDisturb` pref for older versions |
| Linux | GNOME `gsettings` (`show-banners`), KDE D-Bus `Inhibited` property, Xfce `xfconf` (`/do-not-disturb`). Other DEs (Sway, Hyprland, i3) are not yet supported — DND detection returns false (fail-open). |

Detection is fail-open: if the API call fails or the platform is unsupported, hermes assumes DND is off and shows the notification.

### Escalation ladder

Define progressive urgency that mutates the notification each time the user defers past a threshold:

```json
{
  "heading": "Restart Required",
  "message": "Security updates need a restart.",
  "max_defers": 5,
  "defer_deadline": "24h",
  "escalation": [
    {
      "after_defers": 2,
      "timeout": 120,
      "accent_color": "#FF6600",
      "message_suffix": "\n\nThis action is required soon."
    },
    {
      "after_defers": 4,
      "timeout": 60,
      "accent_color": "#FF0000",
      "message_suffix": "\n\nFINAL NOTICE: Action required immediately."
    }
  ]
}
```

After 2 deferrals: timeout shortens to 120s, accent turns orange, warning appended. After 4: timeout 60s, accent red, final notice. The highest matching threshold wins.

### Action chaining

Map user responses to automatic follow-up actions. The action runs server-side after the notification completes:

```json
{
  "buttons": [
    {"label": "Restart Now", "value": "restart", "style": "primary"},
    {"label": "Open Wiki", "value": "wiki", "style": "secondary"}
  ],
  "result_actions": {
    "restart": "action:reboot",
    "wiki": "uri:https://wiki.example.com/vpn"
  }
}
```

Supported prefixes: `uri:` (opens in system handler, see [`allowedSchemes`](../internal/action/action.go)) and `action:` (built-in verb, see [`validVerbs`](../internal/action/action.go)). Actions also fire on timeout if `timeout_value` matches a key (e.g. `"timeout:restart"` matches `"restart"`).

### Quiet hours

Suppress notifications during specified hours. The service delays delivery until the window ends:

```json
{
  "quiet_hours": {
    "start": "22:00",
    "end": "07:00",
    "timezone": "America/Los_Angeles"
  }
}
```

Overnight ranges (start > end) are supported. Timezone defaults to local if omitted. Deadlines are still enforced — a notification past its deadline auto-actions even during quiet hours.

### Localization

Provide translated heading and message text. The resolved locale selects the best match:

```json
{
  "heading": "Restart Required",
  "heading_localized": {
    "ja": "再起動が必要です",
    "de": "Neustart erforderlich",
    "es": "Reinicio requerido"
  },
  "message": "Please restart to apply updates.",
  "message_localized": {
    "ja": "アップデートを適用するため再起動してください。",
    "de": "Ihr Computer muss neu gestartet werden."
  }
}
```

Locale resolution order: `--locale` flag > `HERMES_LOCALE` env > `LANG` env > `"en"` fallback.

Demo: `hermes --locale ja --local --config testdata/examples/localized-restart.json`

### Priority

Control delivery order with `priority` (0=low, 10=critical, default 5). Higher priority notifications are delivered first during offline queue drain:

```json
{"heading": "Critical Patch", "priority": 10, "dnd": "ignore"}
{"heading": "Training Reminder", "priority": 3}
```

### Notification dependencies

Create multi-step workflows where notification B waits for notification A:

```json
{"id": "accept-eula", "heading": "Accept EULA", ...}
{"id": "apply-update", "depends_on": "accept-eula", "heading": "Install Update", ...}
```

The second notification is held in `waiting_on_dependency` state until the first completes. Submit both to the service — the manager handles sequencing automatically.

---

## Subcommands

| Command | Description |
|---------|-------------|
| `hermes serve` | Start the gRPC service daemon (with system tray icon when a display server is available) |
| `hermes notify [config]` | Send notification to service (blocks for result). Broadcasts when run as SYSTEM/root ([details](broadcast.md)). |
| `hermes list` | List active notifications with numbered positions (#, ID, heading, state, defers, deadline) |
| `hermes list --json` | List active notifications as JSON array (for scripting) |
| `hermes show <id or #>` | Show full notification details and available actions |
| `hermes respond <id or #> [value]` | Submit a response for a notification. Interactive picker when no value and TTY. |
| `hermes cancel <id or #>` | Cancel an active notification |
| `hermes history` | View notification history (opens GUI, or prints summary if headless) |
| `hermes history --json` | Print notification history as JSON to stdout |
| `hermes install` | Configure MOTD hook and launch daemon in active user sessions (when elevated). Called by package postinstall. |
| `hermes uninstall` | Remove MOTD hook. Called by package removal scripts. |
| `hermes stop` | Graceful daemon shutdown (gRPC then fallback kill) |
| `hermes motd` | Print pending notification summary for SSH login banners (called by profile.d scripts) |
| `hermes motd --oneline` | Print "N pending" or nothing (scriptable pending-count check, 100ms timeout) |
| `hermes demo` | Show a demo notification |
| `hermes version` | Print version, build date, Go, and OS info |

---

## Flags

| Flag | Scope | Description |
|------|-------|-------------|
| `--config <path or json>` | root | config file or inline JSON/YAML — routes to service |
| `--local` | root | Render locally in current session (skip service) |
| `--locale <code>` | root | Override locale for localized notifications (e.g. `ja`, `de`) |
| `--no-tray` | serve | Disable the system tray icon (default: auto-detect display server) |
| `--db <path>` | serve, history, motd | Bolt database path (default: platform-specific, see [Architecture](architecture.md#persistence)) |
| `--json` | list, history | Machine-readable JSON output |
| `--oneline` | motd | Print "N pending" or nothing (scriptable pending-count check, 100ms timeout) |
| `--help` | all | Print help |

---

## Terminal CLI (SSH-only users)

For headless environments where no GUI is available, the terminal CLI provides full notification interaction. Commands that take an ID also accept a `#` position from `hermes list`:

```bash
# List active notifications (numbered)
hermes list

# See full details by position number
hermes show 1

# Respond by position number
hermes respond 1 restart

# Respond interactively (TTY only, numbered menu)
hermes respond 1

# Cancel by position number
hermes cancel 1

# Full IDs also work
hermes show abc123def456

# Scriptable pending-count check (100ms timeout, never blocks)
hermes motd --oneline
```

### hermes respond exit codes

| Code | Meaning |
|------|---------|
| `0` | Choice accepted (non-defer value) |
| `1` | Error: not found, invalid value, service down, no TTY and no value |
| `200` | Choice accepted (defer value) |

### Automation example

```bash
ids=$(hermes list --json | jq -r '.[] | select(.state == "awaiting_response") | .id')
for id in $ids; do
  hermes respond "$id" restart
  echo "exit: $?"
done
```

### Discovery

`hermes install` configures the MOTD hook so `hermes motd` runs on SSH login. Users see pending notifications at login and can run `hermes list` to check at any time. The `--oneline` flag provides a scriptable pending-count check with a 100ms hard deadline: on any error, timeout, or daemon down, it prints nothing and exits 0.

---

## Exit codes

Exit code constants are defined in [`internal/exitcodes/exitcodes.go`](../internal/exitcodes/exitcodes.go). Key values:

| Code | Meaning |
|------|---------|
| `0` | User chose an action (response on stdout) or dismissed (empty stdout) |
| `1` | Error (bad config, validation, launch failure, or at capacity) |
| `200` | User deferred (response on stdout, starts with `defer`) |
| `202` | Timeout (countdown expired, auto-actioned per config) |
| `203` | Queued (service unreachable, notification saved for later delivery; stdout: `queued`) |

**Detecting dismissals:** Exit `0` with **empty stdout** means the user dismissed the notification (ESC / window close) without choosing an action. Scripts should check both the exit code and stdout content.

---

## Input methods

hermes auto-detects how you're providing the config (JSON and YAML files are both supported):

### File path

```bash
hermes notify restart-notification.json
hermes --config restart-notification.json
```

### YAML file

```bash
hermes notify restart-notification.yml
hermes --config restart-notification.yaml
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

PowerShell 5.1 strips inner double quotes when passing strings to native executables. Piping via stdin avoids this entirely:

```powershell
$config = @'
{
  "heading": "System Restart Required",
  "message": "Your computer needs to restart.",
  "timeout": 300,
  "timeout_value": "restart",
  "defer_deadline": "24h",
  "max_defers": 3,
  "buttons": [
    {"label": "Defer 1h", "value": "defer_1h", "style": "secondary"},
    {"label": "Restart Now", "value": "restart", "style": "primary"}
  ]
}
'@

$config | & hermes.exe notify
```

---

## Example templates

See `testdata/examples/` for ready-to-use configs (JSON and YAML):

- `restart-notification.json` — Restart with defer dropdown
- `update-notification.json` — Software update with defer
- `simple-notification.json` — Simple one-button acknowledgment
- `defer-with-dropdown.json` — VPN disconnect with defer dropdown
- `short-defer-restart.json` — Short deferral (2m deadline, 3 max) for quick lifecycle testing
- `short-defer-deadline.json` — Very short deadline (1m) for testing auto-action
- `image-carousel.json` — Multi-slide image carousel with placeholder images
- `install-with-watch.json` — Filesystem watch for install receipt validation
- `escalation-restart.json` — Escalation ladder: soft → firm → mandatory after repeated deferrals
- `action-chaining.json` — Result actions: user response triggers automatic follow-up
- `quiet-hours.json` — Time-based delivery suppression (22:00–07:00)
- `localized-restart.json` — Multi-language restart (ja, de, es, fr, ko, zh)
- `priority-critical.json` — Priority 10 critical alert (ignores DND, no defer)
- `workflow-step1-eula.json` — Dependency chain step 1: accept EULA
- `workflow-step2-update.json` — Dependency chain step 2: install update (waits for EULA)
