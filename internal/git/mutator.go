// Package git is mint's lock-resilient git MUTATION wrapper — the built-in that
// carries forward the legacy git_safe behaviour, tested once here and applied to
// every git mutation mint makes. It wraps a runner.CommandRunner: a mutation that
// hits a contended `.git` lock is retried within a bounded budget, and a
// provably-STALE lock (no live holder) is cleared so the retry can proceed, while a
// LIVE/fresh lock is NEVER destroyed (destroying a live lock would corrupt a real
// concurrent git op). A background agent or editor briefly holding the index lock
// therefore can no longer blow up a release.
//
// The load-bearing invariant is STALE-vs-LIVE discrimination: a lock is treated as
// stale only when its file mtime is older than StalenessThreshold; anything fresher
// is assumed live and is waited on (backoff + retry), never removed. The exact
// threshold, retry budget, and backoff are implementation detail — tuned for
// production, fully injectable so tests are deterministic and never sleep.
//
// Only the staleness check (os.Stat) and the lock removal (os.Remove) touch the
// filesystem directly; every git invocation, including each retry, flows through the
// wrapped CommandRunner so it stays scripted and asserted in tests. The wrapper needs
// NO repo root: git names the ABSOLUTE lock path in its own stderr, which is the only
// path the wrapper ever stats or removes.
package git

import (
	"bytes"
	"context"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"mint/internal/runner"
)

// lockPathPattern extracts the absolute lock-file path git names in its
// lock-contention stderr (`Unable to create '<path>': File exists`). The captured
// group is the `.../.git/….lock` path the wrapper stats and (when stale) removes.
var lockPathPattern = regexp.MustCompile(`Unable to create '([^']+)': File exists`)

// anotherProcessSignature is the second half of git's lock-contention stderr. Either
// signature marks a failure as lock contention (vs an ordinary git error), but the
// PATH-bearing one is required to classify/clear a stale lock.
const anotherProcessSignature = "Another git process seems to be running"

// Default tuning for the production Mutator. These are deliberately small and short:
// a real contended lock clears in well under a second, and a budget of a few attempts
// with a brief backoff covers the transient case without ever stalling a release.
const (
	defaultRetryBudget        = 3
	defaultStalenessThreshold = 5 * time.Second
	defaultBackoffStep        = 100 * time.Millisecond
)

// Mutator wraps a CommandRunner with lock resilience for git MUTATIONS. Reads pass
// straight through (Run/RunWith); only Mutate carries the retry + stale-lock logic.
// It is constructed once per run from the raw runner and shared by every mutation
// call site (record, release, the engine unwind).
type Mutator struct {
	runner             runner.CommandRunner
	retryBudget        int
	stalenessThreshold time.Duration
	backoff            func(attempt int)
	now                func() time.Time
}

// Option configures a Mutator. Production relies on the defaults; tests inject a
// fixed clock, a no-op backoff, and a tuned budget/threshold so they are
// deterministic and never sleep.
type Option func(*Mutator)

// WithRetryBudget sets the maximum number of attempts a contended mutation makes
// before surfacing the lock failure.
func WithRetryBudget(n int) Option {
	return func(m *Mutator) { m.retryBudget = n }
}

// WithStalenessThreshold sets how old a lock's mtime must be (relative to now) before
// it is treated as stale and cleared. Anything fresher is assumed live.
func WithStalenessThreshold(d time.Duration) Option {
	return func(m *Mutator) { m.stalenessThreshold = d }
}

// WithBackoff sets the wait between retries against a LIVE lock. Production sleeps;
// tests inject a no-op so they never block.
func WithBackoff(b func(attempt int)) Option {
	return func(m *Mutator) { m.backoff = b }
}

// WithNow sets the clock used for the lock mtime comparison. Production defaults to
// time.Now; tests inject a fixed clock for deterministic stale-vs-live decisions.
func WithNow(now func() time.Time) Option {
	return func(m *Mutator) { m.now = now }
}

// NewMutator builds a Mutator wrapping r with production defaults, then applies opts.
// The defaults make a Mutator over the real runner safe to use as-is; the options are
// the test-injection seam.
func NewMutator(r runner.CommandRunner, opts ...Option) *Mutator {
	m := &Mutator{
		runner:             r,
		retryBudget:        defaultRetryBudget,
		stalenessThreshold: defaultStalenessThreshold,
		backoff:            defaultBackoff,
		now:                time.Now,
	}
	for _, opt := range opts {
		opt(m)
	}
	// Clamp the budget to at least 1: a non-positive budget would skip the Mutate retry
	// loop entirely and return a zero-value (Result{}, nil) — a SILENT success having
	// run no git at all, which for a release tool would falsely report a commit/tag/push.
	// Guaranteeing budget ≥ 1 here means Mutate always invokes the runner at least once.
	if m.retryBudget < 1 {
		m.retryBudget = 1
	}
	return m
}

// defaultBackoff is the production wait between retries against a live lock: a short,
// linearly-growing sleep so the holder has a moment to release before the next try.
func defaultBackoff(attempt int) {
	time.Sleep(time.Duration(attempt) * defaultBackoffStep)
}

// Run is a read PASS-THROUGH: it delegates to the wrapped runner unchanged, with NO
// lock logic and NO retry, so a mutation call site holding the Mutator can also do its
// reads (e.g. CommitDirtyTree's `git status --porcelain` probe) through the same seam.
func (m *Mutator) Run(ctx context.Context, name string, args ...string) (runner.Result, error) {
	return m.runner.Run(ctx, name, args...)
}

// RunWith is the stdin-piping read PASS-THROUGH: it delegates to the wrapped runner
// unchanged, with no lock logic, for reads that pipe stdin.
func (m *Mutator) RunWith(ctx context.Context, stdin io.Reader, name string, args ...string) (runner.Result, error) {
	return m.runner.RunWith(ctx, stdin, name, args...)
}

// Mutate runs a git MUTATION with lock resilience. stdin is the command's piped input
// as raw BYTES, not an io.Reader: a retry must pipe the FULL payload again, and a
// single shared io.Reader is exhausted after the first attempt — a lock-retried
// `git tag -a … -F -` would then write an EMPTY annotation. Holding the bytes lets
// invoke build a fresh reader per attempt (the same discipline ai.Transport documents).
// With a nil stdin it uses Run, else RunWith. On success it returns immediately. On a
// NON-lock failure it surfaces the result+error unchanged on the first attempt (so e.g.
// a push rejection still flows through as ErrPushRejected upstream — non-lock errors are
// never swallowed or retried). On a LOCK-contention failure it classifies the named lock
// and either clears a provably-stale lock or backs off a live one, then retries within
// the budget, RE-classifying each attempt. When the budget is exhausted and the lock
// still blocks it returns the last git failure so the caller aborts — it never hangs and
// never silently succeeds.
func (m *Mutator) Mutate(ctx context.Context, stdin []byte, name string, args ...string) (runner.Result, error) {
	var (
		res runner.Result
		err error
	)
	for attempt := 1; attempt <= m.retryBudget; attempt++ {
		res, err = m.invoke(ctx, stdin, name, args...)
		if err == nil {
			return res, nil
		}

		lockPath, isLock := lockContention(res.Stderr)
		if !isLock {
			// A non-lock failure surfaces unchanged on the first attempt — never retried.
			return res, err
		}

		// Re-classify the named lock on every attempt: it may have gone stale or been
		// released since the previous try. The final attempt does not wait/clear — its
		// failure is the surfaced result.
		if attempt < m.retryBudget {
			m.handleLock(lockPath, attempt)
		}
	}
	// Budget exhausted with the lock still blocking: surface the last git failure
	// (carrying the lock stderr) so the caller aborts.
	return res, err
}

// invoke runs the underlying command once through the wrapped runner, choosing the
// stdin-piping seam when stdin is non-nil. It builds a FRESH bytes.NewReader from the
// stdin bytes on every call so each retry attempt pipes the complete payload — a reader
// constructed once and reused would be drained after the first attempt.
func (m *Mutator) invoke(ctx context.Context, stdin []byte, name string, args ...string) (runner.Result, error) {
	if stdin == nil {
		return m.runner.Run(ctx, name, args...)
	}
	return m.runner.RunWith(ctx, bytes.NewReader(stdin), name, args...)
}

// handleLock applies the stale-vs-live decision for one contended attempt: a
// provably-stale lock is removed so the retry can proceed; a live lock is left intact
// and waited on via backoff. A lock that has already vanished just falls through to a
// plain retry. This is the only direct filesystem access in the wrapper.
func (m *Mutator) handleLock(lockPath string, attempt int) {
	if m.lockIsStale(lockPath) {
		// STALE: no live holder — clear it so the retry can take the lock. A removal
		// error is non-fatal; the retry will simply hit the lock again and re-classify.
		_ = os.Remove(lockPath)
		return
	}
	// LIVE/fresh (or already gone): never destroy a live lock — wait and retry, the
	// holder may release it.
	m.backoff(attempt)
}

// lockIsStale reports whether the lock file at lockPath is provably stale: it exists
// and its mtime is older than now minus the staleness threshold. A lock that no longer
// exists, or one that cannot be stat'd, is NOT treated as stale (so it is never
// removed) — a fresh/recent lock is assumed live.
func (m *Mutator) lockIsStale(lockPath string) bool {
	info, err := os.Stat(lockPath)
	if err != nil {
		// Gone or unreadable: not provably stale — do not remove, just retry/back off.
		return false
	}
	return info.ModTime().Before(m.now().Add(-m.stalenessThreshold))
}

// lockContention inspects git stderr for a lock-contention signature and returns the
// named lock path. It reports contention when either git lock signature is present;
// the path is the one git names in `Unable to create '<path>': File exists` (empty
// when only the generic "Another git process" line is present).
func lockContention(stderr string) (lockPath string, isLock bool) {
	if match := lockPathPattern.FindStringSubmatch(stderr); match != nil {
		return match[1], true
	}
	if strings.Contains(stderr, anotherProcessSignature) {
		return "", true
	}
	return "", false
}
