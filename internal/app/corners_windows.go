//go:build windows

package app

import (
	"syscall"
	"unsafe"

	"github.com/google/deck"
	"golang.org/x/sys/windows"
)

var (
	modGdi32               = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRoundRectRgn = modGdi32.NewProc("CreateRoundRectRgn")

	modUser32         = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW   = modUser32.NewProc("FindWindowW")
	procSetWindowRgn  = modUser32.NewProc("SetWindowRgn")
	procGetWindowRect = modUser32.NewProc("GetWindowRect")
)

const cornerEllipse = 20

func applyRoundedCorners(title string) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}

	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		deck.Warningf("rounded corners: FindWindowW(%q) returned 0", title)
		return
	}

	var r struct{ Left, Top, Right, Bottom int32 }
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ret == 0 {
		return
	}

	w := uintptr(r.Right - r.Left)
	h := uintptr(r.Bottom - r.Top)

	hrgn, _, _ := procCreateRoundRectRgn.Call(0, 0, w+1, h+1, cornerEllipse, cornerEllipse)
	if hrgn == 0 {
		return
	}

	ok, _, _ := procSetWindowRgn.Call(hwnd, hrgn, 1)
	if ok == 0 {
		deck.Warningf("rounded corners: SetWindowRgn failed")
	}
}
