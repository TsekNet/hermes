#!/usr/bin/env bash
# Pop every testdata notification for manual validation.
# Works on native Linux (hermes) and WSL (hermes.exe via interop).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

HERMES=""
if grep -qiE '(microsoft|wsl)' /proc/version 2>/dev/null; then
    # WSL: use the Windows binary
    if command -v hermes.exe &>/dev/null; then
        HERMES="hermes.exe"
    elif [[ -x "$REPO_DIR/build/bin/hermes.exe" ]]; then
        HERMES="$REPO_DIR/build/bin/hermes.exe"
    else
        echo "hermes.exe not found in PATH or build/bin/. Run 'wails build -platform windows/amd64' first." >&2
        exit 1
    fi
else
    # Native Linux
    if command -v hermes &>/dev/null; then
        HERMES="hermes"
    elif [[ -x "$REPO_DIR/build/bin/hermes" ]]; then
        HERMES="$REPO_DIR/build/bin/hermes"
    else
        echo "hermes not found in PATH or build/bin/. Run 'wails build' first." >&2
        exit 1
    fi
fi

CONFIGS=(
    simple-notification.json
    restart-notification.json
    update-notification.json
    defer-with-dropdown.json
    short-defer-restart.json
    short-defer-deadline.json
    image-carousel.json
    install-with-watch.json
    priority-critical.json
    escalation-restart.json
    action-chaining.json
    quiet-hours.json
    localized-restart.json
    workflow-step1-eula.json
    workflow-step2-update.json
)

total=${#CONFIGS[@]}
i=0

for config in "${CONFIGS[@]}"; do
    i=$((i + 1))
    path="$SCRIPT_DIR/$config"
    if [[ ! -f "$path" ]]; then
        echo "[$i/$total] SKIP: $config (file not found)"
        continue
    fi

    echo -e "\033[36m[$i/$total] $config\033[0m"
    "$HERMES" --local "$path" || true
    echo ""
done

echo -e "\033[32mDone: $total notifications shown.\033[0m"
