// Package runner is mint's single execution seam for external commands
// (git, gh, claude). Every invocation of an external binary goes through the
// CommandRunner interface so the fragile logic around those processes — exit
// status, stderr inspection, missing-binary detection, and stdin piping — can be
// driven and asserted in tests without touching the host. The os/exec-backed
// ExecRunner is the production implementation; FakeRunner (in this package) is
// the test double that scripts results without spawning processes.
package runner

import (
	"context"
	"errors"
	"io"
)

// ErrCommandNotFound is the sentinel returned (wrapped) when the named binary
// cannot be located on PATH. Callers branch on it with errors.Is to tell a
// missing tool apart from one that ran and exited non-zero — a distinction the
// preflight gh gate depends on (a missing gh is a hard prerequisite failure,
// whereas gh ran-and-failed is a different condition).
var ErrCommandNotFound = errors.New("command not found")

// Result is the outcome of running an external command. Stdout and Stderr are
// captured separately (never interleaved) and ExitCode carries the process exit
// status. On a non-zero exit the Result is still fully populated alongside a
// non-nil error, so callers can both branch on err and read Stderr/ExitCode.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner abstracts external-command execution behind a single seam.
//
// Run executes name with args and returns the captured Result. RunWith is the
// same but pipes stdin into the command's standard input — established now
// because Stage 4 pipes the composed prompt to `claude -p` on stdin and reads
// the notes body off stdout.
//
// Both report a non-nil error when the command cannot run or exits non-zero;
// for a non-zero exit the returned Result remains populated so Stderr and
// ExitCode are still readable. A missing binary is reported as an error matching
// ErrCommandNotFound (via errors.Is).
//
// RunInteractive is the INTERACTIVE launch seam: it runs name with args wired
// directly to the real terminal (os.Stdin/os.Stdout/os.Stderr) so a child that
// takes over the screen — the user's $EDITOR during the review gate's edit
// choice — owns the terminal for its session. Because the child drives the
// terminal itself, its output is NOT captured into a Result; the method returns
// only an error. Like Run, a missing binary is reported as an error matching
// ErrCommandNotFound (via errors.Is) so the editor launcher can tell "no editor
// to launch" (return to the gate) apart from "the editor ran and failed"
// (surface and abort).
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
	RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (Result, error)
	RunInteractive(ctx context.Context, name string, args ...string) error
}
