#Requires -Version 5.1
<#
.SYNOPSIS  Pop every testdata notification for manual validation on Windows.
.DESCRIPTION
    Launches each testdata/*.json config in local mode, one at a time.
    Dismiss or interact with each notification, then the next one appears.
    Requires hermes.exe in PATH or the repo's build/bin/ directory.
.NOTES
    Exit codes: 0 = all shown, 1 = hermes not found
#>

$HERMES = $null
if (Get-Command hermes.exe -ErrorAction SilentlyContinue) {
    $HERMES = 'hermes.exe'
} elseif (Test-Path "$PSScriptRoot\..\build\bin\hermes.exe") {
    $HERMES = (Resolve-Path "$PSScriptRoot\..\build\bin\hermes.exe").Path
} else {
    Write-Error 'hermes.exe not found in PATH or build/bin/. Run "wails build" first.'
    exit 1
}

$TESTDATA = Join-Path $PSScriptRoot 'examples'

# Configs in presentation order: simple first, then features, then advanced.
$CONFIGS = @(
    'simple-notification.json'
    'restart-notification.json'
    'update-notification.json'
    'defer-with-dropdown.json'
    'short-defer-restart.json'
    'short-defer-deadline.json'
    'image-carousel.json'
    'install-with-watch.json'
    'priority-critical.json'
    'escalation-restart.json'
    'action-chaining.json'
    'quiet-hours.json'
    'localized-restart.json'
    'workflow-step1-eula.json'
    'workflow-step2-update.json'
)

$total = $CONFIGS.Count
$i = 0

foreach ($config in $CONFIGS) {
    $i++
    $path = Join-Path $TESTDATA $config
    if (-not (Test-Path $path)) {
        Write-Warning "[$i/$total] SKIP: $config (file not found)"
        continue
    }

    Write-Host "[$i/$total] $config" -ForegroundColor Cyan
    & $HERMES --local $path
    $code = $LASTEXITCODE
    Write-Host "         exit=$code" -ForegroundColor DarkGray
    Write-Host ''
}

Write-Host "Done: $total notifications shown." -ForegroundColor Green

# --- System tray icon tests ---
Write-Host ''
Write-Host '=== System Tray Icon Tests ===' -ForegroundColor Yellow
Write-Host ''

Write-Host '[tray 1/4] hermes serve (tray auto-detect)' -ForegroundColor Cyan
Write-Host '  Starting service with tray icon...'
Write-Host '  Expected: hermes icon appears in Windows notification area (system tray).'
Write-Host '  Verify: right-click icon shows menu with Open History, Pending: 0, Quit Hermes.'
Write-Host '  Press Ctrl+C to stop, then press Enter to continue.'
$job = Start-Job { & $using:HERMES serve 2>&1 }
$null = Read-Host
Stop-Job $job -ErrorAction SilentlyContinue
Remove-Job $job -ErrorAction SilentlyContinue
Write-Host ''

Write-Host '[tray 2/4] hermes serve --no-tray' -ForegroundColor Cyan
Write-Host '  Starting service without tray icon...'
Write-Host '  Expected: no tray icon, service runs headless.'
Write-Host '  Check stderr for "tray disabled" log message.'
Write-Host '  Press Ctrl+C to stop, then press Enter to continue.'
$job = Start-Job { & $using:HERMES serve --no-tray 2>&1 }
$null = Read-Host
Stop-Job $job -ErrorAction SilentlyContinue
Remove-Job $job -ErrorAction SilentlyContinue
Write-Host ''

Write-Host '[tray 3/4] Tray pending count' -ForegroundColor Cyan
Write-Host '  Start "hermes serve" in another terminal, then run:'
Write-Host '    hermes notify ''{"heading":"Test","message":"Check tray count."}'''
Write-Host '  Expected: tray tooltip updates to "1 pending notification".'
Write-Host '  Dismiss the notification, tooltip should return to "no pending".'
Write-Host '  Press Enter when done.'
$null = Read-Host

Write-Host '[tray 4/4] Tray "Open History" menu item' -ForegroundColor Cyan
Write-Host '  With "hermes serve" running, right-click the tray icon.'
Write-Host '  Click "Open History".'
Write-Host '  Expected: history window opens showing notification history.'
Write-Host '  Press Enter when done.'
$null = Read-Host

Write-Host 'Tray tests complete.' -ForegroundColor Green
