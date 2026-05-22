//go:build windows

package socket

import (
	"errors"
	"syscall"
)

// WSAEADDRINUSE is the Windows Sockets error for "address already in use".
const wsaeaddrinuse = syscall.Errno(10048)

func isAddrInUse(err error) bool {
	return errors.Is(err, wsaeaddrinuse)
}
