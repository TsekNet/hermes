# Test Data

JSON configs for every notification type. Used by unit tests and the manual validation scripts below.

## Manual validation scripts

Each script pops every config in sequence. Dismiss or interact with each notification, then the next one appears automatically. Requires a `wails build` binary (not plain `go build`).

### Windows (PowerShell)

```powershell
.\testdata\windows.ps1
```

### Linux / WSL

Auto-detects WSL and uses `hermes.exe` via interop, or `hermes` on native Linux.

```bash
./testdata/linux.sh
```

### macOS

```bash
./testdata/mac.sh
```

## Building the binary

The scripts look for `hermes` (or `hermes.exe`) in `PATH`, then fall back to `build/bin/`. Build with:

```bash
# Native (current OS)
wails build

# Cross-compile for Windows from Linux/macOS
wails build -platform windows/amd64

# Cross-compile for macOS (universal) from Linux
wails build -platform darwin/universal
```

## Screenshots

See [examples/](examples/) for captured screenshots of every notification type.
