package git_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mint/internal/git"
	"mint/internal/runner"
)

// fixedNow is the deterministic clock the tests inject so the stale-vs-live mtime
// comparison is reproducible — no real wall-clock dependency.
var fixedNow = time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

// lockStderr renders the git lock-contention stderr naming the absolute lock path,
// exactly as real git reports a held index/ref lock. The path is what the Mutator
// extracts to stat/remove.
func lockStderr(lockPath string) string {
	return "fatal: Unable to create '" + lockPath + "': File exists.\n\n" +
		"Another git process seems to be running in this repository, e.g.\n" +
		"an editor opened by 'git commit'. Please make sure all processes\n" +
		"are terminated then try again.\n"
}

// writeLock creates a lock file at path with the given mtime so a test can present
// either a stale (old mtime) or a live (recent mtime) lock to the Mutator.
func writeLock(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("pid 123\n"), 0o644); err != nil {
		t.Fatalf("writing lock file: %v", err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("setting lock mtime: %v", err)
	}
}

// lockExists reports whether the lock file at path is still present.
func lockExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// testMutator builds a Mutator over f with deterministic test wiring: a fixed clock,
// a no-op backoff (so tests never sleep), and a generous-but-bounded retry budget.
func testMutator(f *runner.FakeRunner, opts ...git.Option) *git.Mutator {
	base := []git.Option{
		git.WithNow(func() time.Time { return fixedNow }),
		git.WithBackoff(func(int) {}),
		git.WithRetryBudget(3),
		git.WithStalenessThreshold(5 * time.Second),
	}
	return git.NewMutator(f, append(base, opts...)...)
}

// lockErr scripts a non-zero git call whose stderr is the lock-contention signature
// for lockPath.
func lockErr(lockPath string) runner.ScriptedCall {
	return runner.ScriptedCall{
		Result: runner.Result{Stderr: lockStderr(lockPath), ExitCode: 128},
		Err:    errors.New("exit status 128"),
	}
}

// okCall scripts a clean successful git call.
func okCall() runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{}}
}

func TestMutate_ContendedLockThatClears_RetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	// A contended .git lock that frees within the budget → eventual mutation success.
	// The lock file exists with an OLD mtime (stale), so the Mutator clears it and
	// retries; the second attempt is seeded to succeed.
	lockPath := filepath.Join(t.TempDir(), ".git", "index.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	writeLock(t, lockPath, fixedNow.Add(-1*time.Hour)) // stale: well past the threshold

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		lockErr(lockPath), // attempt 1 — contended
		okCall(),          // attempt 2 — succeeds
	)

	m := testMutator(f)
	if _, err := m.Mutate(context.Background(), nil, "git", "commit", "-m", "x"); err != nil {
		t.Fatalf("Mutate returned unexpected error: %v", err)
	}

	if lockExists(lockPath) {
		t.Error("stale lock file still present, want it removed before the retry")
	}
	if got := len(f.Invocations()); got != 2 {
		t.Errorf("git invocations = %d, want 2 (contended attempt then a successful retry)", got)
	}
}

func TestMutate_StaleLock_ClearedAndRetried(t *testing.T) {
	t.Parallel()

	// A provably-stale lock (mtime older than the threshold, no live holder) is
	// CLEARED (os.Remove observed — file gone after) and the mutation retried.
	lockPath := filepath.Join(t.TempDir(), ".git", "shallow.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	writeLock(t, lockPath, fixedNow.Add(-10*time.Second)) // older than the 5s threshold

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		lockErr(lockPath), // attempt 1 — stale lock blocks
		okCall(),          // attempt 2 — succeeds after the clear
	)

	m := testMutator(f)
	if _, err := m.Mutate(context.Background(), nil, "git", "tag", "-d", "v0.0.1"); err != nil {
		t.Fatalf("Mutate returned unexpected error: %v", err)
	}

	if lockExists(lockPath) {
		t.Error("stale lock file still present, want it os.Remove'd before the retry")
	}
}

func TestMutate_LiveLock_NotClearedAndRetriedWithBackoff(t *testing.T) {
	t.Parallel()

	// A live/fresh lock (recent mtime, within the threshold) is NOT cleared: the
	// Mutator must leave the file intact (destroying a live lock corrupts a real
	// concurrent git op), back off (no-op in test), and retry. Here a later attempt
	// is seeded to succeed (the holder released its lock), so Mutate succeeds without
	// ever removing the file.
	lockPath := filepath.Join(t.TempDir(), ".git", "index.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	writeLock(t, lockPath, fixedNow.Add(-1*time.Second)) // fresh: within the 5s threshold

	backoffs := 0
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		lockErr(lockPath), // attempt 1 — live lock blocks
		okCall(),          // attempt 2 — the holder released; success
	)

	m := testMutator(f, git.WithBackoff(func(int) { backoffs++ }))
	if _, err := m.Mutate(context.Background(), nil, "git", "commit", "-m", "x"); err != nil {
		t.Fatalf("Mutate returned unexpected error: %v", err)
	}

	if !lockExists(lockPath) {
		t.Error("live lock file was removed, want it preserved (a live lock must never be destroyed)")
	}
	if backoffs != 1 {
		t.Errorf("backoff invoked %d times, want 1 (a live lock backs off before retry)", backoffs)
	}
}

func TestMutate_ExhaustedRetries_SurfacesLockError(t *testing.T) {
	t.Parallel()

	// Every seeded attempt returns the lock error against a LIVE lock: after the
	// budget is spent the Mutator returns the last git failure (carrying the lock
	// stderr) so the caller aborts — never hanging, never silently succeeding. The
	// live lock is NEVER removed.
	lockPath := filepath.Join(t.TempDir(), ".git", "index.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	writeLock(t, lockPath, fixedNow.Add(-1*time.Second)) // fresh: live for the whole run

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		lockErr(lockPath),
		lockErr(lockPath),
		lockErr(lockPath),
	)

	m := testMutator(f, git.WithRetryBudget(3))
	res, err := m.Mutate(context.Background(), nil, "git", "commit", "-m", "x")
	if err == nil {
		t.Fatal("Mutate returned nil error after exhausting the budget, want the lock failure to surface")
	}
	if !strings.Contains(res.Stderr, "Unable to create") {
		t.Errorf("returned stderr = %q, want it to carry the lock stderr so the caller can report it", res.Stderr)
	}
	if !lockExists(lockPath) {
		t.Error("live lock file was removed across the exhausted retries, want it preserved")
	}
	if got := len(f.Invocations()); got != 3 {
		t.Errorf("git invocations = %d, want 3 (one per budgeted attempt, no more)", got)
	}
}

func TestMutate_NonPositiveBudget_StillRunsOnce(t *testing.T) {
	t.Parallel()

	// A non-positive retry budget must NOT degrade Mutate into a zero-value silent
	// success: the budget is clamped to at least 1 at construction, so the runner is
	// still invoked exactly once and the seeded result/error is surfaced. For a release
	// tool, reporting a successful commit/tag/push without ever running git is dangerous.
	for _, budget := range []int{0, -1} {
		budget := budget
		t.Run(strconv.Itoa(budget), func(t *testing.T) {
			t.Parallel()

			seeded := errors.New("exit status 1")
			f := runner.NewFakeRunner()
			f.Seed("git", runner.Result{Stderr: "boom\n", ExitCode: 1}, seeded)

			m := testMutator(f, git.WithRetryBudget(budget))
			res, err := m.Mutate(context.Background(), nil, "git", "commit", "-m", "x")
			if !errors.Is(err, seeded) {
				t.Errorf("error = %v, want the seeded error surfaced (not a zero-value silent success)", err)
			}
			if res.Stderr != "boom\n" {
				t.Errorf("stderr = %q, want the seeded result surfaced", res.Stderr)
			}
			if got := len(f.Invocations()); got != 1 {
				t.Errorf("git invocations = %d, want 1 (a clamped budget still runs the command once)", got)
			}
		})
	}
}

func TestMutate_NonLockError_SurfacedImmediatelyWithoutRetry(t *testing.T) {
	t.Parallel()

	// A non-lock git failure (e.g. a push rejection) is returned on the FIRST attempt
	// — no retry, no filesystem touch. Surfacing non-lock failures unchanged is what
	// lets a push rejection still flow through as ErrPushRejected upstream.
	rejection := errors.New("exit status 1")
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stderr: "! [rejected] HEAD -> main (fetch first)\n", ExitCode: 1}, rejection)

	m := testMutator(f)
	res, err := m.Mutate(context.Background(), nil, "git", "push", "--atomic", "origin", "HEAD", "v0.0.1")
	if err == nil {
		t.Fatal("Mutate returned nil error on a non-lock failure, want it surfaced")
	}
	if !errors.Is(err, rejection) {
		t.Errorf("error = %v, want the original non-lock error surfaced unchanged", err)
	}
	if !strings.Contains(res.Stderr, "rejected") {
		t.Errorf("returned stderr = %q, want the rejection stderr surfaced unchanged", res.Stderr)
	}
	if got := len(f.Invocations()); got != 1 {
		t.Errorf("git invocations = %d, want 1 (a non-lock failure is never retried)", got)
	}
}

func TestMutate_LockGoneBetweenAttempts_RetriesWithoutRemove(t *testing.T) {
	t.Parallel()

	// If the lock file no longer exists (the holder released it after reporting the
	// error) the Mutator treats it as "gone" and simply retries — no os.Remove on a
	// missing file. The lock path is named in the stderr but the file is absent.
	lockPath := filepath.Join(t.TempDir(), ".git", "index.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating .git dir: %v", err)
	}
	// Deliberately do NOT create the lock file: it is gone by the time we classify.

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		lockErr(lockPath), // attempt 1 — reported a lock, but the file is gone
		okCall(),          // attempt 2 — succeeds
	)

	m := testMutator(f)
	if _, err := m.Mutate(context.Background(), nil, "git", "commit", "-m", "x"); err != nil {
		t.Fatalf("Mutate returned unexpected error: %v", err)
	}
	if got := len(f.Invocations()); got != 2 {
		t.Errorf("git invocations = %d, want 2 (a gone lock just retries)", got)
	}
}

func TestMutate_Success_NoLockLogicNoFilesystemTouch(t *testing.T) {
	t.Parallel()

	// The production happy path: the command succeeds on the first attempt, so Mutate
	// runs it once and returns — no retry, no classification, behaving exactly as a
	// bare runner call. This is what keeps every existing call-site test green.
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "ok"}, nil)

	m := testMutator(f)
	res, err := m.Mutate(context.Background(), nil, "git", "commit", "-m", "x")
	if err != nil {
		t.Fatalf("Mutate returned unexpected error: %v", err)
	}
	if res.Stdout != "ok" {
		t.Errorf("stdout = %q, want %q passed through unchanged", res.Stdout, "ok")
	}
	if got := len(f.Invocations()); got != 1 {
		t.Errorf("git invocations = %d, want 1 (happy path runs the command once)", got)
	}
}

func TestMutate_WithStdin_PipesThroughRunWith(t *testing.T) {
	t.Parallel()

	// A non-nil stdin routes the mutation through RunWith so the piped content (e.g.
	// the annotated-tag message) reaches the command — Mutate must drive the same
	// stdin-piping seam TagAndPush relies on.
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{}, nil)

	m := testMutator(f)
	if _, err := m.Mutate(context.Background(), strings.NewReader("tag body"), "git", "tag", "-a", "v0.0.1", "-F", "-"); err != nil {
		t.Fatalf("Mutate returned unexpected error: %v", err)
	}

	invs := f.Invocations()
	if len(invs) != 1 {
		t.Fatalf("invocations = %d, want 1", len(invs))
	}
	if invs[0].Stdin != "tag body" {
		t.Errorf("piped stdin = %q, want %q", invs[0].Stdin, "tag body")
	}
}

func TestRun_ReadPassThrough_NoLockLogicNoRetry(t *testing.T) {
	t.Parallel()

	// Run is a pure pass-through for reads a mutation call site also performs (e.g.
	// CommitDirtyTree's status probe): it delegates to the wrapped runner with NO lock
	// logic and NO retry. A read that "fails" (even with a lock-shaped stderr) is
	// returned as-is on the first call, never retried.
	readErr := errors.New("exit status 1")
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stderr: lockStderr("/some/.git/index.lock"), ExitCode: 1}, readErr)

	m := testMutator(f)
	res, err := m.Run(context.Background(), "git", "status", "--porcelain")
	if !errors.Is(err, readErr) {
		t.Errorf("error = %v, want the read error returned as-is", err)
	}
	if !strings.Contains(res.Stderr, "Unable to create") {
		t.Errorf("stderr = %q, want it passed through unchanged", res.Stderr)
	}
	if got := len(f.Invocations()); got != 1 {
		t.Errorf("git invocations = %d, want 1 (a read pass-through is never retried)", got)
	}
}

func TestRunWith_ReadPassThrough_DelegatesUnchanged(t *testing.T) {
	t.Parallel()

	// RunWith is the stdin-piping read pass-through: it delegates to the wrapped
	// runner with no lock logic, draining stdin into the recorded invocation.
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "out"}, nil)

	m := testMutator(f)
	res, err := m.RunWith(context.Background(), strings.NewReader("in"), "git", "hash-object", "--stdin")
	if err != nil {
		t.Fatalf("RunWith returned unexpected error: %v", err)
	}
	if res.Stdout != "out" {
		t.Errorf("stdout = %q, want %q", res.Stdout, "out")
	}
	invs := f.Invocations()
	if len(invs) != 1 || invs[0].Stdin != "in" {
		t.Errorf("invocation = %+v, want one call piping stdin %q", invs, "in")
	}
}
