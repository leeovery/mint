package ptytest

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openPty opens the pty side of a new pair, non-blocking so reads of it can time
// out, and names its terminal, as posix_openpt, unlockpt and ptsname do.
func openPty() (*os.File, string, error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, "", err
	}
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		_ = unix.Close(fd)
		return nil, "", err
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		_ = unix.Close(fd)
		return nil, "", err
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), fmt.Sprintf("/dev/pts/%d", n), nil
}
