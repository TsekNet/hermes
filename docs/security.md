# Security

## Reporting Vulnerabilities

Use [GitHub private vulnerability reporting](https://github.com/TsekNet/hermes/security/advisories/new)
to disclose security issues. Do not file public issues for vulnerabilities.

## Trust Boundary

The trust boundary is the OS user account. The daemon, socket, token,
and database are all per-user with `0600`/`0700` permissions. Any process
running as that user has equivalent access. Cross-user access requires
root or a privilege escalation bug in the OS.

## Attack Surface

| Vector | What stops you | Defense in depth |
|---|---|---|
| **Network probe** | No TCP/UDP listeners. gRPC runs over a Unix domain socket only. Invisible to port scanners. | Socket file `0600`, parent dir `0700`. |
| **Unauthorized IPC** | 32-byte `crypto/rand` token required on every RPC. Token file is `0600`. | `crypto/subtle.ConstantTimeCompare` prevents timing side-channels. Auth interceptor runs before rate limiter. |
| **Notification spam** | Token-bucket rate limiter on `Notify` RPC (burst 5, refill 1/s). Hard cap of 10 concurrent active notifications. | Offline queue also deduplicates by notification ID. |
| **Malicious config payload** | 64 KB size cap enforced before YAML parsing (mitigates expansion attacks). gRPC caps at 128 KB (`MaxRecvMsgSize`). | Required fields validated. Unknown fields ignored by YAML unmarshaler. |
| **XSS in notification text** | `html.EscapeString` applied unconditionally to all user-visible text (heading, message, button labels). | Frontend uses `textContent`, not `innerHTML`. Even if escaping is bypassed, the rendering path is safe. |
| **Terminal injection via MOTD** | `sanitize()` strips all ASCII controls (< 0x20), DEL (0x7F), ESC (0x1B), and C1 controls (0x80-0x9F). | MOTD output is plaintext only, no formatting sequences pass through. |
| **Shell injection via button values** | `cmd:` prefix rejected at config validation, action dispatch, and Wails binding. Three independent layers. | `uri:` uses a scheme allowlist (not denylist). `action:` uses a verb allowlist. `Respond()` validates against `HasValue()` before dispatching. |
| **Path traversal via watch_paths** | Must be absolute (`filepath.IsAbs`). Must be clean (`filepath.Clean` round-trip comparison). Max 5 paths. | Filesystem watcher operates read-only (inotify/kqueue/ReadDirectoryChanges). |
| **SVG script execution in webview** | SVG data URIs explicitly rejected in image validation. | Only `https:` URLs and `data:image/` raster URIs are accepted. Max 5 images. |
| **Temp file snooping in broadcast mode** | Config written inside a `0711` temp directory. File is `0644` (required for cross-session child reads). Directory listing blocked. | Temp directory + file cleaned up after broadcast returns. |
| **Stale socket hijack** | Daemon probes existing socket before cleanup: live daemon detected and startup rejected. | Parent directory is `0700`, bounding the TOCTOU window to same-user processes. |
| **Privilege escalation from broadcast** | Child processes drop to target user's UID/GID via `CreateProcessAsUser` (Windows) or `syscall.Credential` + `Setsid` (Unix). | Children connect to the user's own daemon with the user's own token. Broadcast parent never accesses user daemons directly. |
| **Webview calling arbitrary backend functions** | `Respond()` rejects values not in the notification config (`HasValue()`), not matching a valid `uri:` scheme, or not matching a valid `action:` verb. | Webview content is embedded at compile time. No remote content loaded. |
| **Dependency supply chain** | `go.sum` provides integrity verification for all Go dependencies. | Wails is a build-time tool, not a runtime dependency. |

## Cross-References

| Topic | Location |
|---|---|
| Architecture, transport, auth, rate limiting | [Architecture](architecture.md#grpc-transport) |
| Config fields, action prefixes, URI schemes | [Usage](usage.md#config-format) |
| Broadcast, privilege drop, session enumeration | [Broadcast](broadcast.md) |
| Platform-specific deployment, webview engines | [Platforms](platforms.md) |
