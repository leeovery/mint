//go:build unix

package runner

import (
	"os"
	"syscall"
)

// terminate asks p to stop with SIGTERM.
func terminate(p *os.Process) error { return p.Signal(syscall.SIGTERM) }
