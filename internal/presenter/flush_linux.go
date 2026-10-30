package presenter

import (
	"os"

	"golang.org/x/sys/unix"
)

// flushInput discards the input the terminal f has queued and nothing has read:
// tcflush(fd, TCIFLUSH).
func flushInput(f *os.File) {
	_ = unix.IoctlSetInt(int(f.Fd()), unix.TCFLSH, unix.TCIFLUSH)
}
