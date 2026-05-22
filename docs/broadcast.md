# Broadcast Mode

When `hermes notify` (or `hermes '{"heading":"..."}'`) runs as **SYSTEM** (Windows) or **root** (macOS/Linux), it automatically broadcasts the notification to all active user sessions. No wrapper scripts, `Invoke-AsLoggedInUser`, or `sudo -u` needed.

---

## How it works

```
SYSTEM/root process
  └─ hermes notify '{"heading":"Restart"}'
       ├─ detects isPrivileged()
       ├─ enumerates active user sessions
       └─ for each session:
            └─ launches "hermes notify ..." as that user
                 ├─ reads user's auth token
                 └─ delivers to user's hermes serve daemon
```

Hermes reuses the same session launch machinery as `hermes install`:

| Platform | Mechanism |
|----------|-----------|
| Windows | `WTSEnumerateSessionsW` + `CreateProcessAsUser` |
| Linux | `/run/user/<uid>` scan + `SysProcAttr.Credential` |
| macOS | `/Users` scan + `SysProcAttr.Credential` |

Each child process runs in the user's context with access to the per-user auth token (`session.token`) and daemon socket.

---

## Usage

All input methods work: inline JSON, file path, stdin pipe.

```powershell
# Windows (from a SYSTEM-context script)
& "$env:ProgramFiles\Hermes\hermes.exe" notify '{"heading":"Restart Required","message":"Please restart to apply updates."}'
```

```bash
# Linux/macOS (from a root-context script)
hermes notify '{"heading":"Restart Required","message":"Please restart to apply updates."}'
```

```bash
# File path
hermes notify /etc/hermes/restart-notification.json
```

```bash
# Stdin pipe
cat notification.json | hermes notify
```

---

## Fleet integration

Fleet scripts run as SYSTEM (Windows) or root (macOS/Linux). Broadcast mode means Fleet policies and scripts can call hermes directly:

| Fleet mechanism | Example |
|-----------------|---------|
| `install_script` / `post_install_script` | Call `hermes notify` after installing software |
| Policy `run_script` | Remediation script sends a notification |
| `controls.scripts` | Admin-triggered notification to all sessions |

No `Invoke-AsLoggedInUser` or `sudo -u` wrappers are needed. Hermes handles user enumeration and context switching internally.

---

## Behavior

| Condition | Result |
|-----------|--------|
| One active user session | Notification delivered to that user |
| Multiple active sessions (e.g. RDP + console) | Delivered to each unique user once (dedup by username) |
| No active sessions | Falls back to offline queue (delivered on next login, 30-day TTL) |
| Running as non-privileged user | Normal direct delivery to local daemon (no broadcast) |

---

## Logging

Broadcast events are logged to Event Log (Windows) or syslog (macOS/Linux):

```
notification: mode=broadcast (privileged), relaunching in user sessions
session launch: started hermes serve in session 1 (jdoe)
```
