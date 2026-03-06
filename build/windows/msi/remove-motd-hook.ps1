foreach ($dir in @($PSHOME, (Join-Path $env:ProgramFiles 'PowerShell\7'))) {
    $prof = Join-Path $dir 'Profile.ps1'
    if (-not (Test-Path $prof)) { continue }
    $lines = Get-Content -Path $prof -Encoding UTF8 | Where-Object { $_ -notmatch '^\s*if \(Test-Path .+hermes-motd\.ps1.+# Hermes-MOTD\s*$' }
    Set-Content -Path $prof -Value $lines -Encoding UTF8
}
