<#
.SYNOPSIS
    Capture screenshots of every hermes notification and the inbox.

.DESCRIPTION
    Launches each JSON config from testdata/ via hermes --local, captures the
    window to a PNG, then populates history via hermes serve + notify and
    captures the inbox. Requires a built hermes.exe (wails build).

.PARAMETER HermesExe
    Path to hermes.exe. Defaults to build\bin\hermes.exe relative to repo root.

.PARAMETER TestData
    Path to testdata directory. Defaults to testdata\ relative to repo root.

.PARAMETER OutDir
    Directory for captured PNGs. Defaults to assets\examples\ relative to repo root.

.EXAMPLE
    .\assets\examples\screenshot.ps1
    .\assets\examples\screenshot.ps1 -HermesExe C:\bin\hermes.exe -TestData C:\configs
#>
param(
    [string]$HermesExe,
    [string]$TestData,
    [string]$OutDir
)

$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path "$PSScriptRoot\..\..").Path
if (-not $HermesExe) { $HermesExe = Join-Path $repoRoot 'build\bin\hermes.exe' }
if (-not $TestData)  { $TestData  = Join-Path $repoRoot 'testdata' }
if (-not $OutDir)    { $OutDir    = $PSScriptRoot }

if (-not (Test-Path $HermesExe)) {
    Write-Error "hermes.exe not found at $HermesExe - run 'wails build' first"
    return
}

# ── Win32 helpers ──

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms
Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Drawing;
using System.Drawing.Imaging;
using System.Text;

public class HermesCapture {
    public delegate bool EnumWindowsProc(IntPtr hWnd, IntPtr lParam);

    [DllImport("user32.dll")] public static extern bool EnumWindows(EnumWindowsProc cb, IntPtr lParam);
    [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, StringBuilder sb, int max);
    [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint pid);
    [DllImport("dwmapi.dll")] public static extern int DwmGetWindowAttribute(IntPtr hwnd, int attr, out RECT rect, int size);

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int L, T, R, B; }

    public static IntPtr FindWindowByPid(uint targetPid) {
        IntPtr found = IntPtr.Zero;
        EnumWindows((hWnd, lParam) => {
            if (!IsWindowVisible(hWnd)) return true;
            uint pid;
            GetWindowThreadProcessId(hWnd, out pid);
            if (pid == targetPid) {
                StringBuilder sb = new StringBuilder(256);
                GetWindowText(hWnd, sb, 256);
                if (sb.Length > 0) { found = hWnd; return false; }
            }
            return true;
        }, IntPtr.Zero);
        return found;
    }

    public static bool Capture(IntPtr hwnd, string path) {
        RECT r;
        // DWMWA_EXTENDED_FRAME_BOUNDS = 9
        if (DwmGetWindowAttribute(hwnd, 9, out r, Marshal.SizeOf(typeof(RECT))) != 0)
            GetWindowRect(hwnd, out r);
        int w = r.R - r.L, h = r.B - r.T;
        if (w < 10 || h < 10) return false;
        using (var bmp = new Bitmap(w, h, PixelFormat.Format32bppArgb)) {
            using (var g = Graphics.FromImage(bmp))
                g.CopyFromScreen(r.L, r.T, 0, 0, new Size(w, h));
            bmp.Save(path, ImageFormat.Png);
        }
        return true;
    }
}
"@ -ReferencedAssemblies System.Drawing

function Invoke-Capture([System.Diagnostics.Process]$proc, [string]$path) {
    Start-Sleep -Seconds 4
    $proc.Refresh()
    $hwnd = $proc.MainWindowHandle
    if ($hwnd -eq [IntPtr]::Zero) {
        Start-Sleep -Seconds 2
        $proc.Refresh()
        $hwnd = $proc.MainWindowHandle
    }
    if ($hwnd -eq [IntPtr]::Zero) {
        $hwnd = [HermesCapture]::FindWindowByPid([uint32]$proc.Id)
    }
    if ($hwnd -ne [IntPtr]::Zero) {
        [HermesCapture]::SetForegroundWindow($hwnd) | Out-Null
        Start-Sleep -Milliseconds 500
        return [HermesCapture]::Capture($hwnd, $path)
    }
    return $false
}

# ── Phase 1: individual notification screenshots ──

Stop-Process -Name hermes -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

$configs = Get-ChildItem -Path $TestData -Filter '*.json' | Sort-Object Name
Write-Host "Capturing $($configs.Count) notifications from $TestData"

foreach ($cfg in $configs) {
    $name = $cfg.BaseName
    Write-Host -NoNewline "  [$name] "
    $p = Start-Process -FilePath $HermesExe -ArgumentList '--local', "`"$($cfg.FullName)`"" -PassThru

    if (Invoke-Capture $p (Join-Path $OutDir "$name.png")) {
        Write-Host 'OK'
    } else {
        Write-Host 'FAILED'
    }

    Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
}

# ── Phase 1b: hero screenshot (about + defer + "Need help?") ──

$hero_config = @{
    heading      = 'Hermes'
    message      = "A cross-platform notification framework for IT teams.`n`nPowered by Go and Wails v2, Hermes displays rich notifications in a native webview with support for buttons, deferrals, image carousels, and more."
    title        = 'IT Department'
    accent_color  = '#D4A843'
    timeout      = 300
    timeout_value = 'defer_1h'
    esc_value     = 'defer_1h'
    help_url      = 'https://github.com/TsekNet/hermes'
    buttons      = @(
        @{ label = 'Defer'; style = 'secondary'; dropdown = @(
            @{ label = '1 Hour';  value = 'defer_1h' },
            @{ label = '4 Hours'; value = 'defer_4h' },
            @{ label = '1 Day';   value = 'defer_1d' }
        )},
        @{ label = 'Get Started'; value = 'start'; style = 'primary' }
    )
}

$hero_tmp = Join-Path $env:TEMP 'hermes-hero.json'
$hero_json = $hero_config | ConvertTo-Json -Depth 5
[System.IO.File]::WriteAllText($hero_tmp, $hero_json)

Write-Host -NoNewline '  [hero] '
$p = Start-Process -FilePath $HermesExe -ArgumentList '--local', "`"$hero_tmp`"" -PassThru

$hero_out = Join-Path $OutDir '..\hero.png'
if (Invoke-Capture $p $hero_out) {
    Write-Host 'OK'
} else {
    Write-Host 'FAILED'
}

Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
Remove-Item $hero_tmp -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# ── Phase 2: inbox with diverse history ──

Write-Host "`nPopulating inbox history..."

$dbPath = Join-Path $env:LOCALAPPDATA 'hermes\hermes.db'
Remove-Item $dbPath -Force -ErrorAction SilentlyContinue

$daemon = Start-Process -FilePath $HermesExe -ArgumentList 'serve' -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 3

$inboxConfigs = @(
    @{ heading='System Restart Required';    message='Your computer needs to restart to apply security updates.';           title='IT Department';       accent_color='#D4A843'; timeout=3; timeout_value='restart';    buttons=@(@{label='Restart Now';    value='restart';    style='primary'}) },
    @{ heading='VPN Disconnecting';          message='Your VPN session will be disconnected for network maintenance.';      title='Network Ops';         accent_color='#D4A843'; timeout=3; timeout_value='disconnect'; buttons=@(@{label='Disconnect Now'; value='disconnect'; style='danger'})  },
    @{ heading='Critical Security Patch';    message='A zero-day vulnerability has been patched. Immediate restart required.'; title='Security Ops';     accent_color='#FF0000'; timeout=3; timeout_value='restart';    buttons=@(@{label='Restart Now';    value='restart';    style='danger'})  },
    @{ heading='Software Update Available';  message='A new version of the corporate VPN is ready to install.';             title='IT Department';       accent_color='#D4A843'; timeout=3; timeout_value='install';    buttons=@(@{label='Install Now';    value='install';    style='primary'}) },
    @{ heading='End User License Agreement'; message='Please review and accept the updated EULA.';                          title='IT Department';       accent_color='#76B900'; timeout=3; timeout_value='accepted';   buttons=@(@{label='Accept';         value='accepted';   style='primary'}) },
    @{ heading='Maintenance Window';         message='Scheduled maintenance begins in 30 minutes.';                         title='IT Department';       accent_color='#D4A843'; timeout=3; timeout_value='dismiss';    buttons=@(@{label='Got it';         value='dismiss';    style='primary'}) },
    @{ heading='Security Agent Install';     message='Click Install to begin.';                                             title='Platform Eng';        accent_color='#D4A843'; timeout=3; timeout_value='timeout';    buttons=@(@{label='Install';        value='install';    style='primary'}) },
    @{ heading='VPN Disconnected';           message='Your VPN connection was dropped. Reconnect to access corporate resources.'; title='IT Department';  accent_color='#76B900'; timeout=3; timeout_value='dismiss';    buttons=@(@{label='Reconnect VPN';  value='reconnect';  style='primary'}) }
)

$tmpDir = Join-Path $env:TEMP 'hermes-inbox'
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

$i = 0
foreach ($n in $inboxConfigs) {
    $i++
    $json = $n | ConvertTo-Json -Compress -Depth 5
    $tmpFile = Join-Path $tmpDir "notif_$i.json"
    [System.IO.File]::WriteAllText($tmpFile, $json)

    Write-Host "  Submitting $i/$($inboxConfigs.Count): $($n.heading)"
    Start-Process -FilePath $HermesExe -ArgumentList 'notify', '--config', $tmpFile -WindowStyle Hidden -Wait
    Start-Sleep -Seconds 5
}

Write-Host '  Opening inbox...'
Start-Sleep -Seconds 2
$inbox = Start-Process -FilePath $HermesExe -ArgumentList 'inbox' -PassThru

if (Invoke-Capture $inbox (Join-Path $OutDir 'inbox.png')) {
    Write-Host '  inbox.png OK'
} else {
    Write-Host '  inbox.png FAILED'
}

Stop-Process -Id $inbox.Id  -Force -ErrorAction SilentlyContinue
Stop-Process -Id $daemon.Id -Force -ErrorAction SilentlyContinue
Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "`nDone. Screenshots saved to $OutDir"
