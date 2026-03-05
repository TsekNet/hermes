<div align="center">
  <img src="assets/logo.png" alt="hermes logo" width="250"/>
  <h1>hermes</h1>
  <p><strong>Cross-platform web-based notification service.</strong> One binary. One web UI. Every platform.</p>

  [![codecov](https://codecov.io/gh/TsekNet/hermes/branch/main/graph/badge.svg)](https://codecov.io/gh/TsekNet/hermes)
  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
  [![GitHub Release](https://img.shields.io/github/v/release/TsekNet/hermes)](https://github.com/TsekNet/hermes/releases)
</div>

---

*hermes* [^1] renders system notifications inside a pretty [Wails v2](https://wails.io) webview instead of platform-specific toasts. Defines configs in JSON and produces consistent-looking notifications across Linux, macOS, and Windows.

[^1]: Named after *Hermes*, the Greek messenger god.

<div align="center">
<img src="assets/screenshot-windows.png" alt="Windows" width="600"/>

<details>
<summary><strong>macOS &amp; Linux screenshots</strong></summary>
<br>

**macOS**
<img src="assets/screenshot-macos.png" alt="macOS" width="600"/>

**Linux**
<img src="assets/screenshot-linux.png" alt="Linux" width="600"/>

</details></br>
</div>

> **Disclaimer:** This was created as a fun side project, not affiliated with any company.

## Install

Grab a binary from [Releases](https://github.com/TsekNet/hermes/releases) -- zero runtime dependencies, single Go binary with all the web UI code embedded.

## Quick start

```bash
# Start the service daemon
hermes serve

# Send a notification (blocks until user responds)
hermes notify '{"heading":"Restart Required","message":"Please restart.","buttons":[{"label":"Restart","value":"restart","style":"primary"}]}'

# Or use --config
hermes --config notification.json

# List active notifications
hermes list

# View notification history (inbox)
hermes inbox
hermes inbox --json

# Show a demo notification (no service needed)
hermes demo
```

### Local mode (no service)

```bash
hermes --local '{"heading":"Test","message":"Quick local test."}'
hermes --local testdata/restart-notification.json
```

### Deferrals

```bash
hermes notify '{"heading":"Restart","message":"Please restart.","deferDeadline":"24h","maxDefers":3,"buttons":[{"label":"Defer 1h","value":"defer_1h","style":"secondary"},{"label":"Restart","value":"restart","style":"primary"}]}'
```

The service tracks deferrals per notification, persisted to disk. When the user defers, the notification reappears after the specified interval — even across service restarts. After `maxDefers` or `deferDeadline`, the notification auto-actions.

## Features

| Feature | Description |
|---------|-------------|
| **Buttons & dropdowns** | Primary, secondary, danger styles. Dropdown menus for defer options. |
| **Deferrals** | User can defer N times within a deadline. State persists across restarts. |
| **Image carousel** | Embed slides/screenshots via HTTPS URLs or data URIs. Arrow key navigation. |
| **Filesystem watch** | Monitor paths for changes (e.g. wait for install receipt). UI updates live. |
| **Do Not Disturb** | Detects OS Focus/DND mode. Default: wait and retry. Also: skip or ignore. |
| **Settings URIs** | `url:ms-settings:windowsupdate` (Windows), `url:x-apple.systempreferences:...` (macOS). Platform-filtered at runtime. |
| **Countdown timer** | Auto-action after timeout. Configurable value returned to calling script. |
| **Inbox / history** | Completed notifications are persisted. View past actions via `hermes inbox` (UI or JSON). Auto-pruned by age and count. |

## Why web-based

| | Native notifications | hermes |
|-|---------------------|--------|
| **Look** | Different on every OS | Identical everywhere -- your HTML/CSS |
| **Interactivity** | Limited (approve/dismiss) | Buttons, dropdowns, countdown, links |
| **Branding** | OS-controlled | Fully yours -- colors, layout, fonts |
| **Portability** | Rewrite per platform | Single HTML/CSS/JS, ships in the binary |

## Documentation

- [Usage](docs/usage.md) -- JSON config, subcommands, flags, exit codes
- [Architecture](docs/architecture.md) -- service daemon, gRPC, deployment, packages
- [Development](docs/development.md) -- building, testing, dev workflow
- [Platforms](docs/platforms.md) -- webview engines, per-OS deployment

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License

[MIT](LICENSE)
