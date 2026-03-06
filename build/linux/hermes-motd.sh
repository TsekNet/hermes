#!/bin/sh
# Hermes SSH login banner — shows pending notifications for headless sessions.
# Installed to /etc/profile.d/hermes-motd.sh by the .deb (Linux) and .pkg (macOS).

[ -t 1 ] || return 0
[ -n "$SSH_CLIENT" ] || [ -n "$SSH_TTY" ] || return 0
command -v hermes >/dev/null 2>&1 || return 0

_hermes_json=$(timeout 2 hermes inbox --json 2>/dev/null) || return 0
_hermes_headings=$(printf '%s' "$_hermes_json" | sed -n 's/.*"heading"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | sed 's/[[:cntrl:]]//g')
_hermes_count=$(printf '%s\n' "$_hermes_headings" | grep -c .)
[ "$_hermes_count" -gt 0 ] 2>/dev/null || { unset _hermes_json _hermes_headings _hermes_count; return 0; }

printf '\n-- Hermes: %d pending notification(s) --\n' "$_hermes_count"
printf '%s\n' "$_hermes_headings" | head -5 | while IFS= read -r _h; do printf '  * %s\n' "$_h"; done
[ "$_hermes_count" -gt 5 ] && printf '  ... and %d more\n' $((_hermes_count - 5))
printf "Run 'hermes inbox' for details.\n"
printf '----------------------------------------\n\n'

unset _hermes_json _hermes_headings _hermes_count
