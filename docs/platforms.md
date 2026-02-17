# Platform Details

Platform-specific behavior: webview engines and deployment. See **[architecture.md](architecture.md)** for the service daemon design, gRPC protocol, and window positioning.

## Webview engines

| Platform | Engine |
|----------|--------|
| Windows | WebView2 (Edge/Chromium) |
| macOS | WKWebView (Safari) |
| Linux | WebKitGTK |

## Deployment

The `hermes serve` daemon runs **per-user** in the desktop session. Install an autostart entry so it launches at login:

| Platform | Mechanism | Location |
|----------|-----------|----------|
| Windows | Registry Run key | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` |
| macOS | LaunchAgent | `~/Library/LaunchAgents/com.tseknet.hermes.plist` |
| Linux | systemd user unit | `~/.config/systemd/user/hermes.service` |

See **[architecture.md — Deployment](architecture.md#deployment)** for copy-pasteable configs.

Deployment tooling (scripts, config management, MDM profiles) drops the binary and the autostart config. The daemon itself requires no elevated privileges.
