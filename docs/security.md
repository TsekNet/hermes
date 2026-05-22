# Security

This document describes Hermes's security model, the threats it defends
against, and the boundaries where it relies on OS-level guarantees.

## Reporting Vulnerabilities

Use [GitHub private vulnerability reporting](https://github.com/TsekNet/hermes/security/advisories/new)
to disclose security issues. Do not file public issues for vulnerabilities.

## Architecture

Hermes runs as a per-user daemon communicating over a Unix domain socket.
No network listeners are created. The gRPC transport is `insecure` because
traffic never leaves the machine: the UDS provides OS-level isolation
equivalent to a loopback-only TCP socket with file permission enforcement.

| Component | Trust boundary | Access control |
|---|---|---|
| gRPC daemon (`hermes serve`) | Per-user UDS | Socket file `0600`, parent dir `0700` |
| Session token | Per-user file | Token file `0600`, parent dir `0700` |
| Wails webview (notification UI) | Embedded content only | No remote content loaded |
| Broadcast mode (SYSTEM/root) | Cross-session | Drops to target user's UID/GID via `CreateProcessAsUser` (Windows) or `syscall.Credential` (Unix) |
| bbolt database | Per-user file | DB file `0600`, parent dir `0700` |

## Authentication

The daemon generates a 32-byte cryptographically random token
(`crypto/rand`) on startup, stored at a platform-specific path with `0600`
permissions. Every gRPC call must present this token as `authorization`
metadata. Comparison uses `crypto/subtle.ConstantTimeCompare` to prevent
timing side-channels.

Any process running as the same user that can read the token file has full
access to the daemon. This is by design: the daemon serves a single user,
and same-user access is equivalent to the user acting directly.

## Input Validation

### Config payloads

| Control | Implementation |
|---|---|
| Size limit | 64 KB max (`MaxConfigSize`), enforced before parsing |
| Format | YAML parser (`gopkg.in/yaml.v3`), size limit mitigates expansion attacks |
| Text sanitization | `html.EscapeString` via `SanitizeText()` on all user-visible strings |
| MOTD sanitization | Strips ASCII control chars (< 0x20), DEL (0x7F), ESC (0x1B), and C1 controls (0x80-0x9F) |

### Button and action values

| Prefix | Behavior | Validation |
|---|---|---|
| `uri:` | Opens via OS default handler | Scheme allowlist: `https`, `http`, `ms-settings`, `x-apple.systempreferences`, `slack`, `msteams`, `companyportal`, `codex` |
| `action:` | Runs built-in verb | Verb allowlist: `reboot`, `shutdown`, `lock` |
| `cmd:` | **Rejected** | Explicitly blocked at validation and dispatch |
| `defer_Xh/d/m/s` | Schedules re-show | Regex-validated duration |
| Plain value | Returned to caller | Checked against config's known values via `HasValue()` |

The `cmd:` prefix is rejected at three layers: config validation, action
dispatch, and Wails binding response handling. Shell execution from config
input is not permitted at any point in the pipeline.

### Filesystem paths

Watch paths (`watch_paths`) must be absolute and clean (`filepath.IsAbs` +
`filepath.Clean` comparison). Path traversal via `..` components is rejected.
Maximum of 5 watch paths per notification.

### Image URIs

Images must be `https:` URLs or `data:image/` base64 URIs. HTTP is rejected.
SVG data URIs are rejected to prevent script execution in the webview.
Maximum of 5 images per notification.

## Rate Limiting

The gRPC `Notify` RPC is rate-limited: burst of 5, refill of 1 per second.
The rate limiter runs after the auth interceptor, so unauthenticated
requests are rejected before consuming rate limit tokens. A hard cap of
10 concurrent active notifications (`MaxActiveNotifications`) prevents
resource exhaustion.

## Broadcast Mode

When running as SYSTEM (Windows) or root (Unix), `hermes notify` broadcasts
to all active user sessions. The broadcast writes a temporary config file
to a directory with `0711` permissions and config file with `0644`
permissions, then launches child processes as each target user.

The child processes connect to each user's own daemon using their own
session token. The broadcast parent never accesses user daemons directly.

## Webview Security

The Wails webview renders embedded frontend assets compiled into the binary.
No remote content is loaded. The frontend uses `textContent` (not
`innerHTML`) for all user-provided text, preventing XSS even if HTML
escaping is bypassed.

The `Respond()` Wails binding validates that the value is a known config
value (`HasValue()`), a valid `uri:` with an allowed scheme, or a valid
`action:` with an allowed verb before dispatching. Unknown values are
rejected with a warning log.

## Socket Lifecycle

The daemon creates its UDS in a `0700` directory. Stale socket cleanup
probes the existing socket: if a connection succeeds, another daemon is
running and startup is rejected. If the probe fails, the stale socket is
removed and a new listener is created. The brief TOCTOU window between
remove and listen is bounded by the `0700` parent directory, which
restricts access to same-user processes. The socket file itself is set to
`0600` after creation.

## Supply Chain

| Control | Implementation |
|---|---|
| Go dependencies | Pinned in `go.mod` with `go.sum` integrity verification |
| Wails CLI | Fetched at build time (`@latest`); Wails is a build tool, not a runtime dependency |
| Config payload size | 64 KB cap prevents YAML expansion attacks |
| gRPC message size | 128 KB cap (`grpc.MaxRecvMsgSize`) |

## Platform-Specific Notes

### Windows

| Aspect | Detail |
|---|---|
| Manifest | `asInvoker` execution level, no UAC elevation requested |
| Privilege drop | `CreateProcessAsUser` with user's session token |
| Shutdown privilege | `SeShutdownPrivilege` enabled via standard `AdjustTokenPrivileges` API |
| Autostart | HKLM Run key (per-machine, set by MSI installer) |

### macOS

| Aspect | Detail |
|---|---|
| Privilege drop | `syscall.Credential` with target UID/GID + `Setsid` |
| Socket path | `~/.hermes/` (not `~/Library/Application Support/`) to stay within 104-byte `sun_path` limit |

### Linux

| Aspect | Detail |
|---|---|
| Privilege drop | `syscall.Credential` with target UID/GID + `Setsid` |
| Socket path | `$XDG_RUNTIME_DIR/hermes/` (falls back to `~/.hermes/`) |
| Wayland | Forces `GDK_BACKEND=x11` for window positioning. X11 does not provide input isolation between clients on the same display. |
| Display | Cross-session launch hardcodes `DISPLAY=:0`. Multi-seat systems may route to the wrong display server. |

