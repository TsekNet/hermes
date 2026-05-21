//go:build !windows

package tray

import _ "embed"

//go:embed icon.png
var iconData []byte
