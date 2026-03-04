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
```

---

## Local mode

For testing or single-session use, render directly without the service:

```bash
hermes --local '{"heading":"Test","message":"Local test."}'
hermes --local notification.json
echo '{"heading":"..."}' | hermes --local
```

---

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
| `images` | array | no | `[]` | HTTPS URLs or `data:image/` URIs for a carousel (max 20, no SVG data URIs) |
| `watchPaths` | array | no | `[]` | Filesystem paths to monitor for changes (max 10, no `..` traversal) |
| `dnd` | string | no | `"respect"` | Do Not Disturb behavior: `"respect"`, `"ignore"`, or `"skip"` |

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

Defer values must match the pattern `defer_Xh`, `defer_Xd`, `defer_Xm` or `defer_Xs` (hours, days, minutes, seconds). The service parses these to schedule re-notification. Deferral state is persisted to disk so notifications survive service restarts (see [Architecture — Persistence](architecture.md#persistence)).

When `maxDefers` is reached or `deferDeadline` has passed, hermes automatically hides any buttons (or dropdown options) that trigger a deferral. If a button has no other action (e.g. it was purely a defer button), it is removed entirely. This forces the user to choose a non-deferral action (e.g. "Restart Now") or let the timeout expire.

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
  "watchPaths": [
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

---

## Subcommands

| Command | Description |
|---------|-------------|
| `hermes serve` | Start the gRPC service daemon |
| `hermes notify [config]` | Send notification to service (blocks for result) |
| `hermes list` | List active notifications |
| `hermes cancel <id>` | Cancel an active notification |
| `hermes demo` | Show a demo notification |
| `hermes version` | Print version, build date, Go, and OS info |

---

## Flags

| Flag | Scope | Description |
|------|-------|-------------|
| `--config <json>` | root | JSON config (file path or inline) — routes to service |
| `--local` | root | Render locally in current session (skip service) |
| `--port <int>` | serve, notify, list, cancel | gRPC port (default: 4770) |
| `--db <path>` | serve | Bolt database path (default: platform-specific, see [Architecture](architecture.md#persistence)) |
| `--help` | all | Print help |

---

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | User chose an action (response on stdout) or dismissed (empty stdout) |
| `1` | Error (bad config, validation, or launch failure) |
| `200` | User deferred (response on stdout, starts with `defer`) |
| `202` | Timeout (countdown expired, auto-actioned per config) |

**Detecting dismissals:** Exit `0` with **empty stdout** means the user dismissed the notification (ESC / window close) without choosing an action. Scripts should check both the exit code and stdout content.

---

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

PowerShell 5.1 strips inner double quotes when passing strings to native executables. Piping via stdin avoids this entirely:

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

---

## Example templates

See `testdata/` for ready-to-use JSON configs:

- `restart-notification.json` — Restart with defer dropdown
- `update-notification.json` — Software update with defer
- `simple-notification.json` — Simple one-button acknowledgment
- `defer-with-dropdown.json` — VPN disconnect with defer dropdown
- `short-defer-restart.json` — Short deferral (2m deadline, 3 max) for quick lifecycle testing
- `short-defer-deadline.json` — Very short deadline (1m) for testing auto-action
- `image-carousel.json` — Multi-slide image carousel with placeholder images
- `install-with-watch.json` — Filesystem watch for install receipt validation
