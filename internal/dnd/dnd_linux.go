//go:build linux

package dnd

import (
	"os/exec"
	"strings"
)

func active() bool {
	if out, err := exec.Command("gsettings", "get", "org.gnome.desktop.notifications", "show-banners").CombinedOutput(); err == nil {
		return strings.TrimSpace(string(out)) == "false"
	}

	if out, err := exec.Command(
		"dbus-send", "--session", "--print-reply", "--dest=org.freedesktop.Notifications",
		"/org/freedesktop/Notifications",
		"org.freedesktop.DBus.Properties.Get",
		"string:org.freedesktop.Notifications", "string:Inhibited",
	).CombinedOutput(); err == nil && strings.Contains(string(out), "true") {
		return true
	}

	if out, err := exec.Command(
		"xfconf-query", "-c", "xfce4-notifyd", "-p", "/do-not-disturb",
	).CombinedOutput(); err == nil {
		return strings.TrimSpace(string(out)) == "true"
	}

	return false
}
