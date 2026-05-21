//go:build windows

package action

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ewxReboot      = 0x02
	ewxPowerOff    = 0x08
	ewxForceIfHung = 0x10

	shtdnReasonFlagPlanned = 0x80000000

	seShutdownName = "SeShutdownPrivilege"
)

var (
	modUser32           = windows.NewLazySystemDLL("user32.dll")
	procExitWindowsEx   = modUser32.NewProc("ExitWindowsEx")
	procLockWorkStation = modUser32.NewProc("LockWorkStation")
)

func platformRunBuiltin(verb string) error {
	switch verb {
	case "reboot":
		if err := enableShutdownPrivilege(); err != nil {
			return fmt.Errorf("enable shutdown privilege: %w", err)
		}
		return exitWindowsEx(ewxReboot | ewxForceIfHung)
	case "shutdown":
		if err := enableShutdownPrivilege(); err != nil {
			return fmt.Errorf("enable shutdown privilege: %w", err)
		}
		return exitWindowsEx(ewxPowerOff | ewxForceIfHung)
	case "lock":
		return lockWorkStation()
	default:
		return fmt.Errorf("unknown verb: %q", verb)
	}
}

func exitWindowsEx(flags uint32) error {
	r, _, err := procExitWindowsEx.Call(
		uintptr(flags),
		uintptr(shtdnReasonFlagPlanned),
	)
	if r == 0 {
		return fmt.Errorf("ExitWindowsEx: %w", err)
	}
	return nil
}

func lockWorkStation() error {
	r, _, err := procLockWorkStation.Call()
	if r == 0 {
		return fmt.Errorf("LockWorkStation: %w", err)
	}
	return nil
}

func enableShutdownPrivilege() error {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		return fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer token.Close()

	var luid windows.LUID
	name, _ := windows.UTF16PtrFromString(seShutdownName)
	err = windows.LookupPrivilegeValue(nil, name, &luid)
	if err != nil {
		return fmt.Errorf("LookupPrivilegeValue: %w", err)
	}

	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges: [1]windows.LUIDAndAttributes{
			{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED},
		},
	}

	err = windows.AdjustTokenPrivileges(token, false, &tp, uint32(unsafe.Sizeof(tp)), nil, nil)
	if err != nil {
		return fmt.Errorf("AdjustTokenPrivileges: %w", err)
	}
	if errno := windows.GetLastError(); errno == windows.ERROR_NOT_ALL_ASSIGNED {
		return fmt.Errorf("AdjustTokenPrivileges: privilege not held")
	}
	return nil
}
