//go:build unix

package runner

import (
	"os"
	"syscall"
)

// diedOfInterrupt reports whether the child was killed by SIGINT.
func diedOfInterrupt(state *os.ProcessState) bool {
	status, ok := state.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGINT
}
