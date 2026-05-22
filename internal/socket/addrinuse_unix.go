//go:build !windows

package socket

import (
	"errors"
	"syscall"
)

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
