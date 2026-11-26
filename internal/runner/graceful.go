package runner

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

// gracefulStop is how a graceful ExecRunner stops a command whose context ends: SIGTERM,
// so the command can stop what it runs and clean up, then SIGKILL once grace has passed
// or the user interrupts mint again, whichever comes first. The command stays in mint's
// process group and on its terminal, so the terminal's Ctrl-C reaches it directly; the
// SIGTERM is for a run only mint was signalled to stop (`kill <mint>`, a closed
// terminal), and the backstop.
type gracefulStop struct {
	grace time.Duration
	// again watches for a further interrupt once the command has had its SIGTERM.
	again interrupts
}

// interrupts starts watching for an interrupt, until stop is called.
type interrupts func() (interrupted <-chan os.Signal, stop func())

// againSignals watches for a second Ctrl-C or SIGTERM: whoever sends one wants out. A
// second SIGHUP is no such ask, a closing terminal can send two, so it is not watched.
// Watching keeps the signal from killing mint, which then unwinds as the first one
// asked.
func againSignals() (<-chan os.Signal, func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	return c, func() { signal.Stop(c) }
}

// run runs cmd to its end, stopping it gracefully when its context ends first. Past
// grace, WaitDelay kills the command and closes its output, which something it left
// running may still hold open.
func (s *gracefulStop) run(cmd *exec.Cmd) error {
	waited := make(chan struct{})
	defer close(waited)
	cmd.Cancel = func() error {
		if err := terminate(cmd.Process); err != nil {
			return err
		}
		interrupted, stop := s.again()
		go func() {
			defer stop()
			select {
			case <-interrupted:
				_ = cmd.Process.Kill()
			case <-waited:
			}
		}()
		return nil
	}
	cmd.WaitDelay = s.grace
	return cmd.Run()
}
