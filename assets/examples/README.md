# Notification Examples

Captured from [`testdata/`](../../testdata) configs via [`screenshot.ps1`](screenshot.ps1).

---

### Simple notification

Single-button acknowledgement with countdown timer.

![simple-notification](simple-notification.png)

### Software update

Two-button layout with defer and primary action.

![update-notification](update-notification.png)

### Defer with dropdown

Dropdown menu offering multiple deferral durations, plus a "Need help?" link.

![defer-with-dropdown](defer-with-dropdown.png)

### System restart

Defer dropdown, countdown timer, and help link — the most common IT pattern.

![restart-notification](restart-notification.png)

### Short defer with restart

Aggressive deferral window (seconds) with a hard deadline.

![short-defer-restart](short-defer-restart.png)

### Short defer with deadline

Countdown to forced action with limited deferrals.

![short-defer-deadline](short-defer-deadline.png)

### Image carousel

Embedded images with left/right navigation above the message body.

![image-carousel](image-carousel.png)

### Install with watch path

Monitors a file path and auto-dismisses when the install completes.

![install-with-watch](install-with-watch.png)

### Critical priority

Red accent, no defer option — highest priority overrides Do Not Disturb.

![priority-critical](priority-critical.png)

### Escalation ladder

Progressive urgency: accent color and timeout change after repeated deferrals.

![escalation-restart](escalation-restart.png)

### Localization

Heading and message swap to the user's locale (ja, de, es, fr, ko, zh).

![localized-restart](localized-restart.png)

### Quiet hours

Delivery is suppressed during a configured daily window (e.g. 22:00 -- 07:00).

![quiet-hours](quiet-hours.png)

### Action chaining

Button clicks trigger follow-up actions (`cmd:` or `url:` prefixes).

![action-chaining](action-chaining.png)

### Workflow step 1 — EULA

First step in a dependency chain. Step 2 waits until this is accepted.

![workflow-step1-eula](workflow-step1-eula.png)

### Workflow step 2 — Update

Blocked by `dependsOn: accept-eula`. Only shown after step 1 completes.

![workflow-step2-update](workflow-step2-update.png)

### Notification history (inbox)

Scrollable history of past notifications with outcome badges.

![inbox](inbox.png)
