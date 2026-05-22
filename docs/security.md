# Security

## Reporting Vulnerabilities

Use [GitHub private vulnerability reporting](https://github.com/TsekNet/hermes/security/advisories/new)
to disclose security issues. Do not file public issues for vulnerabilities.

## Security Model

Hermes is a per-user daemon. The trust boundary is the OS user account:
any process running as the same user has equivalent access to the daemon,
its database, and its socket. This is by design.

For the full architecture, transport, authentication, rate limiting, and
capacity controls, see [Architecture](architecture.md#grpc-transport).

For input validation rules (config fields, action prefixes, URI schemes,
image constraints, watch paths), see [Usage](usage.md#config-format).

For broadcast and privilege drop behavior, see
[Broadcast](broadcast.md).

## Design Decisions

| Decision | Rationale |
|---|---|
| No TLS on the Unix domain socket | UDS traffic is kernel-mediated and never leaves the machine. File permissions (`0600` socket, `0700` parent dir) enforce access. TLS would add overhead for zero security gain. |
| No scope differentiation on the auth token | Same-user processes can already `kill` the daemon, `rm` the socket, or write to the database. Read-only scopes would be security theater. |
| `cmd:` prefix rejected at three layers | Config validation, action dispatch, and Wails binding response handling all independently reject `cmd:`. Shell execution from config input is not permitted. |
| Webview renders embedded assets only | No remote content is loaded. The frontend uses `textContent` (not `innerHTML`) for all user-provided text. |
| Broadcast temp config uses restricted directory | The temp directory is `0711` (traversable but not listable), the config file inside is `0644`. This limits exposure on multi-user systems while allowing cross-session child processes to read the config. |
| MOTD sanitizer strips ANSI and C1 controls | Headings flow into SSH login banners. Stripping ESC (0x1B) and C1 (0x80-0x9F) prevents terminal escape injection in addition to the standard ASCII control character filter. |
| Watch paths must be absolute and clean | `filepath.IsAbs` + `filepath.Clean` comparison prevents path traversal. The naive `..` substring check was insufficient. |
| Wails `Respond()` validates against config | Unknown values are rejected before dispatch, preventing the webview from triggering actions not defined in the notification config. |
| Constant-time token comparison | `crypto/subtle.ConstantTimeCompare` prevents timing side-channels on the session token. |
