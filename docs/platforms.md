# Platform Details

Platform-specific behavior: webview engines and deployment. See **[Architecture](architecture.md)** for the service daemon design, gRPC protocol, and window positioning.

---

## Webview engines

| Platform | Engine |
|----------|--------|
| Windows | WebView2 (Edge/Chromium) |
| macOS | WKWebView (Safari) |
| Linux | WebKitGTK |

---

## Deployment

The `hermes serve` daemon runs **per-user** in the desktop session. Run the installer; it handles placement and autostart:

| Platform | Install | Autostart |
|----------|---------|-----------|
| Windows | **hermes.msi** | HKLM Run key at logon. |
| Linux | `sudo dpkg -i hermes.deb` | systemd user unit + profile.d; starts on next login. |
| macOS | **hermes.pkg** / **hermes-arm64.pkg** | LaunchAgent in `/Library/LaunchAgents`. |

See **[Architecture — Deployment](architecture.md#deployment)** for detail.

---

## SSH-only users

Users who connect via SSH without a desktop session won't see the Wails UI. The installers include a login banner that shows pending notification summaries on shell login:

| Platform | Mechanism |
|----------|-----------|
| Linux | `/etc/profile.d/hermes-motd.sh` (installed by .deb) |
| macOS | `/etc/profile.d/hermes-motd.sh` (installed by .pkg; postinstall ensures zsh sources profile.d) |
| Windows | Guarded one-liner in `$PSHOME\Profile.ps1` and `$env:ProgramFiles\PowerShell\7\Profile.ps1` (installed by MSI) |

The banner only appears for SSH sessions (detected via `$SSH_CLIENT` / `$SSH_TTY` on Unix, `$env:SSH_CLIENT` / `$env:SSH_CONNECTION` on Windows). It runs `hermes inbox --json` and prints a summary. Silent when there are no pending notifications. Run `hermes inbox` for full details.
