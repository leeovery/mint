//go:build unix

package presenter

import (
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

// stopSignals is SIGTSTP, the stop the shell sends at Ctrl-Z, watched, and what stops
// this process in its place.
func stopSignals() (<-chan os.Signal, func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, unix.SIGTSTP)
	return c, stopProcess
}

// stopProcess stops this process alone, as the shell's Ctrl-Z would have, and returns
// once fg resumes it: its children took the terminal's SIGTSTP themselves. It raises
// SIGSTOP, not SIGTSTP. Once SIGTSTP is watched the Go runtime keeps its own handler
// for it, signal.Reset or not, and that handler drops a SIGTSTP nothing watches, so a
// re-raised one would never stop the process (tried and proven); the shell reports a
// SIGSTOP as stopped all the same.
func stopProcess() { _ = unix.Kill(os.Getpid(), unix.SIGSTOP) }
