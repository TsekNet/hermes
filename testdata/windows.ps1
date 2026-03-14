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

$TESTDATA = $PSScriptRoot

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
