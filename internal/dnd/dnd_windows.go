//go:build windows

package dnd

import (
	"syscall"
	"unsafe"
)

var (
	shell32                      = syscall.NewLazyDLL("shell32.dll")
	procQueryUserNotificationSt  = shell32.NewProc("SHQueryUserNotificationState")
)

// QUERY_USER_NOTIFICATION_STATE values from shellapi.h.
const (
	qunsNotPresent          = 1
	qunsBusy                = 2
	qunsRunningD3DFullScreen = 3
	qunsPresentationMode    = 4
	qunsAcceptsNotifications = 5
	qunsQuietTime           = 6
	qunsApp                 = 7
)

func active() bool {
	var state uint32
	hr, _, _ := procQueryUserNotificationSt.Call(uintptr(unsafe.Pointer(&state)))
	if hr != 0 {
		return false // API failed; fail-open
	}
	return state != qunsAcceptsNotifications
}
