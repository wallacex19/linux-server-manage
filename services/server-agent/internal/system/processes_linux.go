//go:build linux

package system

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// pidfdSendSignal utiliza as syscalls SYS_PIDFD_OPEN (434) e SYS_PIDFD_SEND_SIGNAL (424)
// introduzidas no Linux 5.1 para eliminar condições de corrida com reutilização rápida de PIDs.
func pidfdSendSignal(pid int, sig syscall.Signal) error {
	fd, _, errno := unix.Syscall(unix.SYS_PIDFD_OPEN, uintptr(pid), 0, 0)
	if errno != 0 {
		return errno
	}
	defer unix.Close(int(fd))

	_, _, errno = unix.Syscall6(unix.SYS_PIDFD_SEND_SIGNAL, fd, uintptr(sig), 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
