package ptytest

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// openPty opens the pty side of a new pair, non-blocking so reads of it can time
// out, and names its terminal, as posix_openpt, grantpt, unlockpt and ptsname do.
//
// XNU numbers a pair's two sides alike, so the terminal is named from the minor of
// the side ptmx cloned: ptsname asks an ioctl for it that x/sys reaches only by a raw
// syscall, deprecated on darwin.
func openPty() (*os.File, string, error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, "", err
	}
	for _, req := range []uint{unix.TIOCPTYGRANT, unix.TIOCPTYUNLK} {
		if err := unix.IoctlSetInt(fd, req, 0); err != nil {
			_ = unix.Close(fd)
			return nil, "", err
		}
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		_ = unix.Close(fd)
		return nil, "", err
	}
	return os.NewFile(uintptr(fd), "/dev/ptmx"), fmt.Sprintf("/dev/ttys%03d", unix.Minor(uint64(st.Rdev))), nil
}
