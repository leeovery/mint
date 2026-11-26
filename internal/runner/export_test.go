package runner

import (
	"os"
	"time"
)

// NewGracefulExecRunnerWatching is NewGracefulExecRunner watching again, in place of
// the process's signals, for the further interrupt that ends the wait.
func NewGracefulExecRunnerWatching(grace time.Duration, again <-chan os.Signal) *ExecRunner {
	watch := func() (<-chan os.Signal, func()) { return again, func() {} }
	return &ExecRunner{graceful: &gracefulStop{grace: grace, again: watch}}
}
