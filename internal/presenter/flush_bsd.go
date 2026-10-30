//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package presenter

import (
	"os"

	"golang.org/x/sys/unix"
)

// fread is FREAD from <sys/fcntl.h>, which TIOCFLUSH takes to flush what is queued
// for reading: tcflush(fd, TCIFLUSH) spelled as the BSDs implement it.
const fread = 0x1

// flushInput discards the input the terminal f has queued and nothing has read.
func flushInput(f *os.File) {
	_ = unix.IoctlSetPointerInt(int(f.Fd()), unix.TIOCFLUSH, fread)
}
