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
    path="$SCRIPT_DIR/examples/$config"
    if [[ ! -f "$path" ]]; then
        echo "[$i/$total] SKIP: $config (file not found)"
        continue
    fi

    echo -e "\033[36m[$i/$total] $config\033[0m"
    "$HERMES" --local "$path" || true
    echo ""
done

echo -e "\033[32mDone: $total notifications shown.\033[0m"

# --- System tray icon tests ---
echo ""
echo -e "\033[33m=== System Tray Icon Tests ===\033[0m"
echo ""

echo -e "\033[36m[tray 1/4] hermes serve (tray auto-detect)\033[0m"
echo "  Starting service with tray icon auto-detection..."
echo "  Expected: tray icon appears if \$DISPLAY or \$WAYLAND_DISPLAY is set."
echo "  Verify: icon in system tray, tooltip says 'Hermes', right-click shows menu."
echo "  Press Ctrl+C to stop, then press Enter to continue."
"$HERMES" serve &
SERVE_PID=$!
read -r
kill "$SERVE_PID" 2>/dev/null; wait "$SERVE_PID" 2>/dev/null
echo ""

echo -e "\033[36m[tray 2/4] hermes serve --no-tray\033[0m"
echo "  Starting service without tray icon..."
echo "  Expected: no tray icon, service runs headless."
echo "  Check stderr for 'tray disabled' log message."
echo "  Press Ctrl+C to stop, then press Enter to continue."
"$HERMES" serve --no-tray &
SERVE_PID=$!
read -r
kill "$SERVE_PID" 2>/dev/null; wait "$SERVE_PID" 2>/dev/null
echo ""

echo -e "\033[36m[tray 3/4] Tray pending count\033[0m"
echo "  Start 'hermes serve' in another terminal, then run:"
echo "    hermes notify '{\"heading\":\"Test\",\"message\":\"Check tray count.\"}'"
echo "  Expected: tray tooltip updates to '1 pending notification'."
echo "  Dismiss the notification, tooltip should return to 'no pending'."
echo "  Press Enter when done."
read -r

echo -e "\033[36m[tray 4/4] Tray 'Open History' menu item\033[0m"
echo "  With 'hermes serve' running, right-click the tray icon."
echo "  Click 'Open History'."
echo "  Expected: history window opens showing notification history."
echo "  Press Enter when done."
read -r

echo -e "\033[32mTray tests complete.\033[0m"
