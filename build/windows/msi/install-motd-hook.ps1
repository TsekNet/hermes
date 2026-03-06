$line = 'if (Test-Path "$env:ProgramFiles\Hermes\hermes-motd.ps1") { . "$env:ProgramFiles\Hermes\hermes-motd.ps1" } # Hermes-MOTD'
foreach ($dir in @($PSHOME, (Join-Path $env:ProgramFiles 'PowerShell\7'))) {
    if (-not (Test-Path $dir)) { continue }
    $prof = Join-Path $dir 'Profile.ps1'
    if ((Test-Path $prof) -and (Select-String -Path $prof -Pattern 'Hermes-MOTD' -Quiet)) { continue }
    Add-Content -Path $prof -Value $line -Encoding UTF8
}
