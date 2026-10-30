//go:build unix

package presenter

import (
	"os"
	"os/signal"

	"golang.org/x/sys/unix"
)

// continuedSignals is SIGCONT, which the shell sends a process it resumes, and the
// stop that ends it.
func continuedSignals() (<-chan os.Signal, func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, unix.SIGCONT)
	return c, func() { signal.Stop(c) }
}
