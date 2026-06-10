package engine_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// indexOfCmd returns the index of the first recorded invocation whose command
// line equals name+args exactly, or -1 if none matched. It lets the preflight-hook
// tests assert ORDER on the FakeRunner timeline (hook after the built-in gates,
// before any mutation).
func indexOfCmd(f *runner.FakeRunner, name string, args ...string) int {
	want := name + " " + strings.Join(args, " ")
	for i, inv := range f.Invocations() {
		if commandLine(inv) == want {
			return i
		}
	}
	return -1
}

// firstIndexWithPrefix returns the index of the first recorded invocation whose
// command line starts with prefix, or -1 if none matched. The mutation commands
// (`git tag -a`, `git push`, …) are matched by prefix because their full args
// vary; the hook must precede all of them.
func firstIndexWithPrefix(f *runner.FakeRunner, prefix string) int {
	for i, inv := range f.Invocations() {
		if strings.HasPrefix(commandLine(inv), prefix) {
			return i
		}
	}
	return -1
}

// shInvoked reports whether the FakeRunner recorded any `sh -c …` call — the
// shape every hook entry runs as.
func shInvoked(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "sh" {
			return true
		}
	}
	return false
}

// countSh returns how many `sh` invocations the FakeRunner recorded — used to
// prove an array hook STOPPED at the first non-zero entry (exactly one ran).
func countSh(f *runner.FakeRunner) int {
	n := 0
	for _, inv := range f.Invocations() {
		if inv.Name == "sh" {
			n++
		}
	}
	return n
}

// TestRelease_PreflightHook_RunsAfterGatesBeforeMutation proves the configured
// preflight hook runs AFTER the built-in preflight gates pass and BEFORE any
// mutation: the `sh -c "scripts/check.sh"` invocation appears after the last
// built-in git gate (ls-remote --tags) and before the bookkeeping commit, tag,
// and push; the run completes successfully.
func TestRelease_PreflightHook_RunsAfterGatesBeforeMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"scripts/check.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("sh", runner.Result{}, nil) // preflight hook exits zero
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	hookAt := indexOfCmd(f, "sh", "-c", "scripts/check.sh")
	if hookAt == -1 {
		t.Fatalf("preflight hook `sh -c scripts/check.sh` never ran; got %v", commandLines(f.Invocations()))
	}
	// The hook must run AFTER the last built-in network gate (ls-remote --tags).
	lastGateAt := indexOfCmd(f, "git", "ls-remote", "--tags", "origin", "refs/tags/v0.0.1")
	if lastGateAt == -1 {
		t.Fatalf("built-in tag-free-remote gate never ran; got %v", commandLines(f.Invocations()))
	}
	if hookAt < lastGateAt {
		t.Errorf("preflight hook ran at %d, before the last built-in gate at %d", hookAt, lastGateAt)
	}
	// The hook must run BEFORE any mutation (commit, tag, push).
	assertHookBeforeMutation(t, f, hookAt)

	// The run reached the successful end-of-run line.
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("run did not finish with a configured preflight hook; last event = %v", fin.Kind)
	}
}

// TestRelease_PreflightHook_NonZeroAbortsBeforeMutation proves a non-zero preflight
// hook aborts cleanly BEFORE any mutation: a non-zero `sh` exit yields a non-zero
// *AbortError, a StageFailed naming "preflight", and NO mutation (nothing tagged,
// pushed, or published) — the abort precedes all mutation, so there is nothing to
// unwind.
func TestRelease_PreflightHook_NonZeroAbortsBeforeMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"scripts/check.sh\"\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	// The preflight hook exits non-zero.
	f.Seed("sh", runner.Result{ExitCode: 1}, errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "preflight" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "preflight")
	}
	// A pre-mutation abort has nothing to unwind — no reset, no Unwound.
	if recorded(rec, presentertest.KindUnwound) {
		t.Errorf("non-zero preflight hook emitted an Unwound; it precedes all mutation")
	}
	assertNoMutation(t, f)
	// The review gate never ran (the abort is before the gate).
	if recorded(rec, presentertest.KindPrompt) {
		t.Errorf("review gate prompted despite a failing preflight hook")
	}
}

// TestRelease_PreflightHook_ArrayAbortsOnFirstNonZero proves an array preflight
// hook aborts on the FIRST non-zero entry: with two entries the first exits
// non-zero, so only one `sh` runs (the second entry never executes) and the run
// aborts before any mutation.
func TestRelease_PreflightHook_ArrayAbortsOnFirstNonZero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = [\"a\", \"b\"]\n")

	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	// The first `sh` entry exits non-zero; the second is seeded zero but must never run.
	f.SeedSequence("sh",
		runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")},
		runner.ScriptedCall{Result: runner.Result{}},
	)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "preflight" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "preflight")
	}
	// Exactly one entry ran: the first failed, so the second was never reached.
	if got := countSh(f); got != 1 {
		t.Errorf("sh ran %d times, want exactly 1 (array stops at the first non-zero entry)", got)
	}
	if invokedWith(f, "sh", "-c", "b") {
		t.Errorf("the second array entry ran despite the first failing")
	}
	assertNoMutation(t, f)
}

// TestRelease_PreflightHook_AbsentSkipped proves an absent [release.hooks] table is
// a silent no-op: no `sh` invocation occurs and the happy-path run proceeds
// normally to a successful finish.
func TestRelease_PreflightHook_AbsentSkipped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No .mint.toml at all — no [release.hooks].
	f := runner.NewFakeRunner()
	seedHappyGit(f, root, "main", "v0.0.1")
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if shInvoked(f) {
		t.Errorf("an absent preflight hook ran `sh`; got %v", commandLines(f.Invocations()))
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("absent-hook run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_PreflightHook_NotRunWhenGateFails proves the preflight hook is gated
// BEHIND the built-in preflight gates: when a built-in gate fails first (here the
// working tree is dirty), the hook never runs — no `sh` invocation occurs and the
// run aborts on the failing gate.
func TestRelease_PreflightHook_NotRunWhenGateFails(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"scripts/check.sh\"\n")

	f := runner.NewFakeRunner()
	// Resolve + fetch succeed; the clean-tree gate fails (porcelain non-empty).
	f.SeedSequence("git",
		ScriptedOut(root),          // rev-parse --show-toplevel
		ScriptedOut("origin/main"), // symbolic-ref --short origin/HEAD
		ScriptedOut(""),            // tag --list
		ScriptedOut(""),            // fetch --tags
		ScriptedOut(" M file.go"),  // status --porcelain (DIRTY — gate fails)
	)
	f.Seed("sh", runner.Result{}, nil) // seeded but must never run
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), patchOptions())

	assertAbortNonZero(t, err)
	if shInvoked(f) {
		t.Errorf("preflight hook ran despite a failing built-in gate; got %v", commandLines(f.Invocations()))
	}
	if !recorded(rec, presentertest.KindStageFailed) {
		t.Errorf("failing built-in gate did not surface a StageFailed event")
	}
	assertNoMutation(t, f)
}

// assertHookBeforeMutation fails the test if any mutation command (bookkeeping
// commit, annotated tag, push, or provider release) precedes the hook at hookAt.
func assertHookBeforeMutation(t *testing.T, f *runner.FakeRunner, hookAt int) {
	t.Helper()
	mutations := []string{
		"git -C ",           // bookkeeping add/commit
		"git tag -a",        // annotated tag
		"git push",          // atomic push
		"gh release create", // provider release
	}
	for _, prefix := range mutations {
		if at := firstIndexWithPrefix(f, prefix); at != -1 && at < hookAt {
			t.Errorf("mutation %q (at %d) preceded the preflight hook (at %d)", prefix, at, hookAt)
		}
	}
}

// stageFailedName returns the Name of the first recorded StageFailed event,
// failing the test if none fired.
func stageFailedName(t *testing.T, rec *presentertest.RecordingPresenter) string {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			return ev.StageFailed.Name
		}
	}
	t.Fatalf("no StageFailed event recorded; kinds = %v", rec.Kinds())
	return ""
}
