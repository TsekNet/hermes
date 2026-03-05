# Usage

## Service mode (recommended)

Start the per-user service daemon, then submit notifications via CLI. The service manages the full lifecycle: UI launch, deferrals, and deadlines.

```bash
# Start the per-user service daemon (see Architecture for autostart setup)
hermes serve

# Send a notification (blocks until user responds)
hermes notify '{"heading":"Restart Required","message":"Please restart."}'

# Or use --config flag on root command
hermes --config notification.json

# List active notifications
hermes list

# Cancel a notification
hermes cancel <notification-id>

# View notification history
hermes inbox

# Print history as JSON
hermes inbox --json
```

---

## Inbox (notification history)

Completed notifications are automatically saved to the history bucket. The inbox lets you review past notifications and their outcomes.

```bash
hermes inbox              # Opens the inbox UI
hermes inbox --json       # Prints history as JSON to stdout
hermes inbox --db my.db   # Read directly from a bolt DB file (skip service)
```

The inbox connects to the running service via gRPC. If the service is unreachable, it falls back to reading the bolt database directly. History is auto-pruned on service startup: records older than 30 days or exceeding 200 entries are removed.

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
| `esc_value` | string | no | `""` | Value returned on ESC (defaults to `timeout_value`) |
| `title` | string | no | `IT Department` | Small uppercase label at the top |
| `accent_color` | string | no | `#D4A843` | Theme accent color (hex) |
| `help_url` | string | no | `""` | "Need help?" link URL |
| `id` | string | no | auto-generated | Unique notification ID for the service |
| `defer_deadline` | string | no | `""` | Max deferral window (e.g., `"24h"`, `"7d"`) |
| `max_defers` | int | no | `0` | Max number of deferrals (0 = unlimited) |
| `images` | array | no | `[]` | HTTPS URLs or `data:image/` URIs for a carousel (max 20, no SVG data URIs) |
| `watch_paths` | array | no | `[]` | Filesystem paths to monitor for changes (max 10, no `..` traversal) |
| `dnd` | string | no | `"respect"` | Do Not Disturb behavior: `"respect"`, `"ignore"`, or `"skip"` |
| `priority` | int | no | `5` | Delivery priority (0-10). Higher = shown first in queue drain |
| `escalation` | array | no | `[]` | Progressive urgency steps applied after repeated deferrals (see below) |
| `result_actions` | object | no | `{}` | Maps response values to automatic actions (action chaining, see below) |
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

### URL and settings URI buttons

Button values prefixed with `url:` open the URI in the default handler instead of closing the notification. Hermes supports platform-specific settings URIs alongside standard web URLs:

| Scheme | Platform | Example |
|--------|----------|---------|
| `https:` | All | `url:https://example.com/kb/update` |
| `http:` | All | `url:http://intranet.corp/install` |
| `ms-settings:` | Windows | `url:ms-settings:windowsupdate` |
| `x-apple.systempreferences:` | macOS | `url:x-apple.systempreferences:com.apple.Software-Update-Settings.extension` |

Settings URIs are only allowed on their native platform. A `ms-settings:` button on macOS is silently blocked (and vice versa). Include both in a shared config -- hermes filters at runtime.

```json
{"label": "Windows Update", "value": "url:ms-settings:windowsupdate", "style": "primary"}
{"label": "Software Update", "value": "url:x-apple.systempreferences:com.apple.Software-Update-Settings.extension", "style": "primary"}
{"label": "FileVault", "value": "url:x-apple.systempreferences:com.apple.preference.security?FileVault", "style": "secondary"}
```

macOS pane IDs follow the pattern `com.apple.preference.<name>` or `com.apple.<Name>-Settings.extension`. Append `?Anchor` for sub-panes (e.g. `?FileVault`, `?Privacy_AllFiles`). Linux has no standard settings URI scheme.

### Command buttons

Button values prefixed with `cmd:` execute a shell command when clicked. The command runs through the platform shell (`cmd /C` on Windows, `sh -c` on Unix). Arguments, pipes, and shell features are supported.

```json
{"label": "Restart Now", "value": "cmd:shutdown /r /t 0", "style": "primary"}
{"label": "Reboot", "value": "cmd:sudo shutdown -r now", "style": "danger"}
```

Commands are also re-executable from the inbox history view. Empty commands (`cmd:` with no argument) are blocked. Only one primary button per notification is recommended -- the Enter key triggers the first primary button.

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

Images must be `https://` URLs or `data:image/` URIs (no SVG). Maximum 20 per notification.

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
    {"label": "Install", "value": "url:https://intranet.example.com/install", "style": "primary"}
  ]
}
```

The notification footer shows "Monitoring filesystem..." initially, then updates with each event (e.g. "create: version.txt"). The frontend receives events via the Wails `fs:event` event channel, so custom frontends can also subscribe.

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
    "restart": "cmd:shutdown /r /t 60",
    "wiki": "url:https://wiki.example.com/vpn"
  }
}
```

Supported prefixes: `cmd:` (shell command) and `url:` (opens in browser). Actions also fire on timeout if `timeout_value` matches a key (e.g. `"timeout:restart"` matches `"restart"`).

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

Demo: `hermes --locale ja --local --config testdata/localized-restart.json`

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
| `hermes serve` | Start the gRPC service daemon |
| `hermes notify [config]` | Send notification to service (blocks for result) |
| `hermes list` | List active notifications |
| `hermes cancel <id>` | Cancel an active notification |
| `hermes inbox` | View notification history (opens inbox UI) |
| `hermes inbox --json` | Print notification history as JSON to stdout |
| `hermes demo` | Show a demo notification |
| `hermes version` | Print version, build date, Go, and OS info |

---

## Flags

| Flag | Scope | Description |
|------|-------|-------------|
| `--config <path or json>` | root | config file or inline JSON/YAML — routes to service |
| `--local` | root | Render locally in current session (skip service) |
| `--locale <code>` | root | Override locale for localized notifications (e.g. `ja`, `de`) |
| `--port <int>` | serve, notify, list, cancel | gRPC port (default: 4770) |
| `--db <path>` | serve, inbox | Bolt database path (default: platform-specific, see [Architecture](architecture.md#persistence)) |
| `--json` | inbox | Print history as JSON instead of opening the UI |
| `--help` | all | Print help |

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | User chose an action (response on stdout) or dismissed (empty stdout) |
| `1` | Error (bad config, validation, or launch failure) |
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

See `testdata/` for ready-to-use configs (JSON and YAML):

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
