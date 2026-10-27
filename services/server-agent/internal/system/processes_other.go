//go:build !linux

package system

import "syscall"

func pidfdSendSignal(pid int, sig syscall.Signal) error {
	return syscall.ENOSYS
}
