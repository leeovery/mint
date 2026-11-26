package runner

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// gracefulStop is how a graceful ExecRunner stops a command whose context ends: SIGTERM,
// so the command can stop what it runs and clean up, then SIGKILL once grace has passed
// or killStopping is called, whichever comes first. The command stays in mint's process
// group and on its terminal, so the terminal's Ctrl-C reaches it directly, as SIGINT;
// that Ctrl-C ends the run's context too, so a command still running when mint acts on
// it gets the SIGTERM as well. Where only mint was signalled (`kill <mint>`, a closed
// terminal), the SIGTERM is all it gets.
type gracefulStop struct {
	grace time.Duration
	mu    sync.Mutex
	// running is the commands started whose runs have yet to return.
	running map[*runningCommand]struct{}
}

// runningCommand is a started command whose run has yet to return: its process, its
// output, and the end of its run's context.
type runningCommand struct {
	process *os.Process
	out     *output
	ended   <-chan struct{}
}

// stopping reports whether the command's run's context has ended.
func (c *runningCommand) stopping() bool {
	select {
	case <-c.ended:
		return true
	default:
		return false
	}
}

func newGracefulStop(grace time.Duration) *gracefulStop {
	return &gracefulStop{grace: grace, running: make(map[*runningCommand]struct{})}
}

// run runs cmd to its end, stopping it gracefully when ctx ends first, and calling the
// stop notice ctx carries once the SIGTERM is sent. Past grace, WaitDelay kills the
// command. Its output is then read to the end, for no longer than grace from the
// SIGTERM, or from its exit where it got none, since something it left running can
// hold the output open; only killStopping cuts that read short.
func (g *gracefulStop) run(ctx context.Context, cmd *exec.Cmd) error {
	out, err := ownOutput(cmd)
	if err != nil {
		return err
	}
	var terminated time.Time
	cmd.Cancel = func() error {
		if err := terminate(cmd.Process); err != nil {
			return err
		}
		terminated = time.Now()
		NoticeStop(ctx)
		return nil
	}
	cmd.WaitDelay = g.grace
	startErr := cmd.Start()
	out.started()
	if startErr != nil {
		out.drain(time.Now())
		return startErr
	}
	defer g.untrack(g.track(&runningCommand{process: cmd.Process, out: out, ended: ctx.Done()}))
	err = cmd.Wait()
	// Wait has had Cancel's result, so terminated is set where Cancel ran.
	from := terminated
	if from.IsZero() {
		from = time.Now()
	}
	out.drain(from.Add(g.grace))
	return err
}

func (g *gracefulStop) track(c *runningCommand) *runningCommand {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.running[c] = struct{}{}
	return c
}

func (g *gracefulStop) untrack(c *runningCommand) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.running, c)
}

// killStopping kills every command whose run's context has ended and whose run has yet
// to return, whether it is stopping on its SIGTERM or its output is still being read,
// and stops reading its output, so each run returns at once. It reports whether there
// were any.
func (g *gracefulStop) killStopping() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	killed := false
	for c := range g.running {
		if !c.stopping() {
			continue
		}
		_ = c.process.Kill()
		c.out.abandon()
		killed = true
	}
	return killed
}

// output is a command's stdout and stderr where they are writers of mint's, read
// through pipes the runner holds, so its wait on them can be cut short: exec's own
// copies end only at EOF, which something the command left running can hold off.
type output struct {
	reads, writes []*os.File
	copied        chan struct{}
	closeReads    sync.Once
}

// ownOutput gives cmd pipes for its stdout and stderr where they are writers of
// mint's, copying what it writes to them into those writers.
func ownOutput(cmd *exec.Cmd) (*output, error) {
	o := &output{copied: make(chan struct{})}
	var copies sync.WaitGroup
	for _, w := range []*io.Writer{&cmd.Stdout, &cmd.Stderr} {
		if _, isFile := (*w).(*os.File); *w == nil || isFile {
			continue
		}
		r, pw, err := os.Pipe()
		if err != nil {
			o.started()
			o.abandon()
			return nil, err
		}
		dst := *w
		*w = pw
		o.reads, o.writes = append(o.reads, r), append(o.writes, pw)
		copies.Go(func() { _, _ = io.Copy(dst, r) })
	}
	go func() {
		copies.Wait()
		close(o.copied)
	}()
	return o, nil
}

// started closes mint's copies of the write ends, which the command has its own of.
func (o *output) started() {
	for _, w := range o.writes {
		_ = w.Close()
	}
}

// abandon closes the read ends, ending the copies: what is still to come is lost.
func (o *output) abandon() {
	o.closeReads.Do(func() {
		for _, r := range o.reads {
			_ = r.Close()
		}
	})
}

// drain waits for the copies to reach the end of the output, until it is past until
// or the output is abandoned, then abandons what is left, so the writers are no longer
// written to once it returns.
func (o *output) drain(until time.Time) {
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-o.copied:
	case <-timer.C:
	}
	o.abandon()
	<-o.copied
}
