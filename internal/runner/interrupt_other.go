//go:build !unix && !windows

package runner

import "os"

// diedOfInterrupt reports false where the platform has no Ctrl-C exit to read.
func diedOfInterrupt(*os.ProcessState) bool { return false }
