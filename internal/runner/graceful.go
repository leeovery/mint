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
// group and on its terminal, so the terminal's Ctrl-C reaches it directly; the SIGTERM
// is for a run only mint was signalled to stop (`kill <mint>`, a closed terminal), and
// the backstop.
type gracefulStop struct {
	grace time.Duration
	mu    sync.Mutex
	// stopping is the commands sent SIGTERM whose runs have yet to return.
	stopping map[*stoppingCommand]struct{}
}

// stoppingCommand is a command sent SIGTERM, and its output.
type stoppingCommand struct {
	process *os.Process
	out     *output
}

func newGracefulStop(grace time.Duration) *gracefulStop {
	return &gracefulStop{grace: grace, stopping: make(map[*stoppingCommand]struct{})}
}

// run runs cmd to its end, stopping it gracefully when ctx ends first, and calling the
// stop notice ctx carries once the SIGTERM is sent. Past grace, WaitDelay kills the
// command. Its output is then read for no longer than grace from the SIGTERM, or from
// its exit where it exited before ctx ended, since something it left running can hold
// the output open; an exit before ctx ended stops that read when ctx ends.
func (g *gracefulStop) run(ctx context.Context, cmd *exec.Cmd) error {
	out, err := ownOutput(cmd)
	if err != nil {
		return err
	}
	stopping := &stoppingCommand{out: out}
	var terminated time.Time
	cmd.Cancel = func() error {
		if err := terminate(cmd.Process); err != nil {
			return err
		}
		terminated = time.Now()
		g.track(stopping, cmd.Process)
		NoticeStop(ctx)
		return nil
	}
	cmd.WaitDelay = g.grace
	startErr := cmd.Start()
	out.started()
	if startErr != nil {
		out.drain(time.Now(), nil)
		return startErr
	}
	defer g.untrack(stopping)
	err = cmd.Wait()
	// Wait has had Cancel's result, so terminated is set where Cancel ran.
	until, interrupted := time.Now().Add(g.grace), ctx.Done()
	if !terminated.IsZero() {
		until, interrupted = terminated.Add(g.grace), nil
	}
	out.drain(until, interrupted)
	return err
}

func (g *gracefulStop) track(c *stoppingCommand, p *os.Process) {
	g.mu.Lock()
	defer g.mu.Unlock()
	c.process = p
	g.stopping[c] = struct{}{}
}

func (g *gracefulStop) untrack(c *stoppingCommand) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.stopping, c)
}

// killStopping kills every command sent SIGTERM whose run has yet to return and stops
// reading its output, so the run returns at once, reporting whether there were any.
func (g *gracefulStop) killStopping() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for c := range g.stopping {
		_ = c.process.Kill()
		c.out.abandon()
	}
	return len(g.stopping) > 0
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
// or interrupted ends, then abandons what is left, so the writers are no longer
// written to once it returns.
func (o *output) drain(until time.Time, interrupted <-chan struct{}) {
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-o.copied:
	case <-timer.C:
	case <-interrupted:
	}
	o.abandon()
	<-o.copied
}
