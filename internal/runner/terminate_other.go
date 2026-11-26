//go:build !unix

package runner

import "os"

// terminate kills p where there is no SIGTERM to send it.
func terminate(p *os.Process) error { return p.Kill() }
