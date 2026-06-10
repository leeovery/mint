package runner

import (
	"context"
	"fmt"
	"io"
)

// Invocation is a single recorded call against a FakeRunner. Stdin holds the
// fully-drained contents piped via RunWith (empty for Run), so tests can assert
// both that a command ran and what was fed to it — e.g. the AI prompt sent to
// `claude -p`.
type Invocation struct {
	Name  string
	Args  []string
	Stdin string
}

// scriptedResult is the pre-seeded outcome for a command name.
type scriptedResult struct {
	result   Result
	err      error
	notFound bool
}

// FakeRunner is the in-memory CommandRunner test double. It never spawns a
// process: callers Seed each command name with the Result/error it should
// return, and every call is recorded in order for later assertion. It matches on
// command name (the args are recorded for assertion but not used for matching),
// which is sufficient for the engine's scripted-tool tests.
type FakeRunner struct {
	scripts     map[string]scriptedResult
	invocations []Invocation
}

// NewFakeRunner returns an empty FakeRunner with no seeded commands.
func NewFakeRunner() *FakeRunner {
	return &FakeRunner{scripts: make(map[string]scriptedResult)}
}

// Compile-time assertion that FakeRunner satisfies the seam.
var _ CommandRunner = (*FakeRunner)(nil)

// Seed registers the Result and error returned for any future call to name.
// Pass a non-nil err together with a populated Result to model a non-zero exit
// (the Result stays readable alongside the error, matching the real runner).
func (f *FakeRunner) Seed(name string, result Result, err error) {
	f.scripts[name] = scriptedResult{result: result, err: err}
}

// SeedNotFound registers name as a missing binary: future calls return an error
// matching ErrCommandNotFound, mirroring ExecRunner so gate tests behave the
// same against either implementation.
func (f *FakeRunner) SeedNotFound(name string) {
	f.scripts[name] = scriptedResult{notFound: true}
}

// Invocations returns the recorded calls in the order they were made.
func (f *FakeRunner) Invocations() []Invocation {
	return f.invocations
}

// Run records the call and returns the seeded outcome for name.
func (f *FakeRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return f.RunWith(ctx, nil, name, args...)
}

// RunWith records the call (draining stdin into the Invocation) and returns the
// seeded outcome for name.
func (f *FakeRunner) RunWith(_ context.Context, stdin io.Reader, name string, args ...string) (Result, error) {
	f.invocations = append(f.invocations, Invocation{
		Name:  name,
		Args:  args,
		Stdin: drainStdin(stdin),
	})

	script, ok := f.scripts[name]
	if !ok {
		// Surface an unseeded command rather than returning a zero Result that
		// could silently mask a missing test setup.
		return Result{}, fmt.Errorf("fakerunner: command %q was not seeded", name)
	}

	if script.notFound {
		return Result{}, fmt.Errorf("running %q: %w", name, ErrCommandNotFound)
	}

	return script.result, script.err
}

// drainStdin reads stdin to completion, returning "" for a nil reader. A read
// error is ignored: the fake is a test helper and any failure would surface as a
// mismatched recorded Stdin, which the test then catches directly.
func drainStdin(stdin io.Reader) string {
	if stdin == nil {
		return ""
	}
	b, _ := io.ReadAll(stdin)
	return string(b)
}
