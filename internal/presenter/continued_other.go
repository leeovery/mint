//go:build !unix

package presenter

import "os"

// continuedSignals is no signal where the platform stops and resumes no process from
// the shell.
func continuedSignals() (<-chan os.Signal, func()) { return nil, nil }
