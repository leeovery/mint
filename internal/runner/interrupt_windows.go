//go:build windows

package runner

import "os"

// statusControlCExit is STATUS_CONTROL_C_EXIT, the exit code of a console
// process ended by Ctrl-C.
const statusControlCExit = 0xC000013A

// diedOfInterrupt reports whether the child ended on Ctrl-C.
func diedOfInterrupt(state *os.ProcessState) bool {
	return uint32(state.ExitCode()) == statusControlCExit
}
