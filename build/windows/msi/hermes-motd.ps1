# Hermes SSH login banner — shows pending notifications for headless sessions.
# Installed to C:\Program Files\Hermes\hermes-motd.ps1 by the MSI.
# Sourced from $PSHOME\Profile.ps1 via a guarded one-liner.

if (-not $env:SSH_CLIENT -and -not $env:SSH_CONNECTION) { return }
if ($Host.Name -ne 'ConsoleHost') { return }

$hermesExe = Join-Path $env:ProgramFiles 'Hermes\hermes.exe'
if (-not (Test-Path $hermesExe)) { return }

try {
    $raw = & $hermesExe inbox --json 2>$null
    if (-not $raw) { return }
    $entries = @($raw | ConvertFrom-Json)
    if ($entries.Count -eq 0) { return }

    $count = $entries.Count
    Write-Host "`n-- Hermes: $count pending notification(s) --"
    $take = [Math]::Min($count, 5)
    for ($i = 0; $i -lt $take; $i++) {
        $heading = $entries[$i].heading -replace '[\x00-\x1f\x7f-\x9f]', ''
        Write-Host "  * $heading"
    }
    if ($count -gt 5) { Write-Host "  ... and $($count - 5) more" }
    Write-Host "Run 'hermes.exe inbox' for details."
    Write-Host "----------------------------------------`n"
} catch {}
