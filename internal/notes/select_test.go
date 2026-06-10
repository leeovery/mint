package notes_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/record"
	"mint/internal/runner"
)

// newSelector builds a Selector over the same composed dependencies SelectBody
// holds in production: the Generator (assembler + transport + root), the
// Assembler (for the degenerate check), the runner (for the no-AI/fallback
// bodies), and the repo root. The same FakeRunner backs both the Generator's
// Assembler and the standalone Assembler so a single seeded git sequence drives
// the whole run.
func newSelector(t *testing.T, r *runner.FakeRunner, transport notes.Transport) *notes.Selector {
	t.Helper()
	root := t.TempDir()
	assembler := notes.NewAssembler(r, notes.ExcludeConfig{})
	gen := notes.NewGenerator(assembler, transport, root)
	return notes.NewSelector(gen, assembler, r, root)
}

// abortRel is a Release that aborts the normal AI path on failure — the strict
// default. Used to prove branches 1-3 never route through on_notes_failure even
// when it would abort.
func abortRel() config.Release {
	return config.Release{OnNotesFailure: "abort"}
}

func TestSelector_SelectBody_FirstReleaseWinsOverNoAIAndDegenerate(t *testing.T) {
	t.Parallel()

	// First release (no prior tag) selects "Initial release." and KindFirstRelease,
	// winning over BOTH --no-ai and a (would-be) degenerate diff. There is no diff
	// base, so the transport is NEVER called, the diff is NEVER assembled (no
	// `git diff` invocation), and on_notes_failure="abort" is irrelevant — no abort
	// happens.
	transport := &recordingTransport{body: "must never be returned"}
	r := runner.NewFakeRunner() // nothing seeded: any git call would error.
	sel := newSelector(t, r, transport)

	state := notes.SelectState{FirstRelease: true, NoAI: true}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: abortRel()})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != record.FirstReleaseBody {
		t.Errorf("body = %q, want the fixed first-release body %q", body, record.FirstReleaseBody)
	}
	if kind != notes.KindFirstRelease {
		t.Errorf("kind = %v, want KindFirstRelease", kind)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times, want 0 (first release never calls the AI)", transport.calls())
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git invoked %d times, want 0 (first release assembles no diff, runs no degenerate check, no fallback)", len(r.Invocations()))
	}
}

func TestSelector_SelectBody_DegenerateDiffWinsOverNoAI(t *testing.T) {
	t.Parallel()

	// With a prior tag, a degenerate (whitespace-only) post-exclusion diff wins over
	// --no-ai: the body is StubBody, kind is KindDegenerate, and the AI is NEVER
	// called even though noAI is also set (degenerate wins over --no-ai).
	transport := &recordingTransport{body: "must never be returned"}
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: "   \n\t\n"}, nil) // whitespace-only diff.
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0", NoAI: true}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: abortRel()})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != notes.StubBody() {
		t.Errorf("body = %q, want StubBody %q", body, notes.StubBody())
	}
	if kind != notes.KindDegenerate {
		t.Errorf("kind = %v, want KindDegenerate", kind)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times, want 0 (degenerate never calls the AI)", transport.calls())
	}

	// Only the degenerate-check diff assemble ran; no `git log` fallback, no abort.
	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("git invocations = %d, want 1 (the degenerate-check assemble only)", len(invs))
	}
	assertGitArgv(t, invs[0], wantArgs("v1.0.0"))
}

func TestSelector_SelectBody_NoAIWinsOverNormalAIPath(t *testing.T) {
	t.Parallel()

	// With a prior tag and a NON-degenerate diff, --no-ai wins over the normal AI
	// path: the body is the NoAIBody commit-subject list, kind is KindNoAI, the
	// transport is NOT called, and it never aborts (on_notes_failure="abort" is
	// irrelevant here).
	const diff = "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n"
	const subjects = "Add login flow\nFix token refresh\n"
	transport := &recordingTransport{body: "must never be returned"}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},     // degenerate-check assemble.
		runner.ScriptedCall{Result: runner.Result{Stdout: subjects}}, // NoAIBody's git log.
	)
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0", NoAI: true}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: abortRel()})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the commit-subject list %q", body, subjects)
	}
	if kind != notes.KindNoAI {
		t.Errorf("kind = %v, want KindNoAI", kind)
	}
	if transport.calls() != 0 {
		t.Errorf("transport called %d times, want 0 (--no-ai never calls the AI)", transport.calls())
	}

	// The git log used the commit-subject argv: --no-ai routed to NoAIBody.
	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("git invocations = %d, want 2 (assemble + commit-subject log)", len(invs))
	}
	assertGitArgv(t, invs[0], wantArgs("v1.0.0"))
	assertGitArgv(t, invs[1], wantCommitSubjectArgs("v1.0.0"))
}

func TestSelector_SelectBody_NormalAIPathRunsOnlyWhenNoEarlierGuardApplies(t *testing.T) {
	t.Parallel()

	// The normal AI path runs ONLY when none of the first three guards apply: a prior
	// tag, a non-degenerate diff, and no --no-ai. The transport IS called, the body is
	// the AI body, and the kind is KindNormalAI.
	const diff = "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	const aiBody = "## TL;DR\n\nShipped the auth package.\n"
	transport := &recordingTransport{body: aiBody}
	// Generate assembles the diff once (via GenerateFromDiff over the already-assembled
	// diff), then BuildChangeMap issues name-status + numstat. SelectBody assembles the
	// diff first for the degenerate check, so the seeded sequence is: degenerate
	// assemble, name-status, numstat.
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "A\tauth/login.go\n"}},
		runner.ScriptedCall{Result: runner.Result{Stdout: "20\t0\tauth/login.go\n"}},
	)
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0", NoAI: false}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: abortRel(), MaxDiffLines: 50000})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != aiBody {
		t.Errorf("body = %q, want the AI body %q", body, aiBody)
	}
	if kind != notes.KindNormalAI {
		t.Errorf("kind = %v, want KindNormalAI", kind)
	}
	if transport.calls() != 1 {
		t.Errorf("transport called %d times, want 1 (the normal AI path)", transport.calls())
	}

	// The AI prompt carries the assembled diff — proving the assembled diff fed the
	// generator, not a re-assembled one with a different value.
	if prompt := transport.lastPrompt(t); !strings.Contains(prompt, diff) {
		t.Errorf("prompt missing the assembled diff:\n%s", prompt)
	}
}

func TestSelector_SelectBody_OnNotesFailureGovernsOnlyNormalAIPath_Abort(t *testing.T) {
	t.Parallel()

	// In the normal AI path, a transport failure with on_notes_failure="abort" routes
	// through ResolveFailure and returns the abort error (matchable via errors.Is to
	// the cause), with NO body.
	const diff = "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n"
	transport := &recordingTransport{err: ai.ErrTimeout}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},           // degenerate-check assemble.
		runner.ScriptedCall{Result: runner.Result{Stdout: "M\ta.go\n"}},    // name-status.
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t1\ta.go\n"}}, // numstat.
	)
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0", NoAI: false}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: config.Release{OnNotesFailure: "abort"}, MaxDiffLines: 50000})
	if err == nil {
		t.Fatal("SelectBody returned nil error on a transport failure with abort, want the abort error")
	}
	if !errors.Is(err, ai.ErrTimeout) {
		t.Errorf("error = %v, want it to match the cause ai.ErrTimeout", err)
	}
	if body != "" {
		t.Errorf("body = %q, want empty when aborting", body)
	}
	if kind != notes.KindNormalAI {
		t.Errorf("kind = %v, want KindNormalAI (the AI path was taken, then aborted)", kind)
	}
}

func TestSelector_SelectBody_OnNotesFailureGovernsOnlyNormalAIPath_Fallback(t *testing.T) {
	t.Parallel()

	// In the normal AI path, a transport failure with on_notes_failure="fallback"
	// routes through ResolveFailure and returns the fallback body with KindFallback.
	const diff = "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n-a\n+b\n"
	const subjects = "Recovered subject one\nRecovered subject two\n"
	transport := &recordingTransport{err: ai.ErrNotesFailure}
	r := runner.NewFakeRunner()
	r.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},           // degenerate-check assemble.
		runner.ScriptedCall{Result: runner.Result{Stdout: "M\ta.go\n"}},    // name-status.
		runner.ScriptedCall{Result: runner.Result{Stdout: "1\t1\ta.go\n"}}, // numstat.
		runner.ScriptedCall{Result: runner.Result{Stdout: subjects}},       // ResolveFailure's git log.
	)
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0", NoAI: false}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: config.Release{OnNotesFailure: "fallback"}, MaxDiffLines: 50000})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error in fallback mode: %v", err)
	}
	if body != subjects {
		t.Errorf("body = %q, want the fallback commit-subject list %q", body, subjects)
	}
	if kind != notes.KindFallback {
		t.Errorf("kind = %v, want KindFallback", kind)
	}
}

func TestSelector_SelectBody_FirstReleaseNeverRoutesThroughOnNotesFailure(t *testing.T) {
	t.Parallel()

	// Branches 1-3 never route through on_notes_failure even when it is set to a
	// value: first-release with on_notes_failure="fallback" still returns the fixed
	// first-release body (NOT a fallback commit-subject list), runs no git, and never
	// touches the resolver.
	transport := &recordingTransport{body: "must never be returned"}
	r := runner.NewFakeRunner()
	sel := newSelector(t, r, transport)

	state := notes.SelectState{FirstRelease: true}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: config.Release{OnNotesFailure: "fallback"}})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != record.FirstReleaseBody {
		t.Errorf("body = %q, want the fixed first-release body %q (not a fallback)", body, record.FirstReleaseBody)
	}
	if kind != notes.KindFirstRelease {
		t.Errorf("kind = %v, want KindFirstRelease", kind)
	}
	if len(r.Invocations()) != 0 {
		t.Errorf("git invoked %d times, want 0 (first release never reaches on_notes_failure)", len(r.Invocations()))
	}
}

func TestSelector_SelectBody_DegenerateNeverRoutesThroughOnNotesFailure(t *testing.T) {
	t.Parallel()

	// Branch 2 never routes through on_notes_failure even when set: a degenerate diff
	// with on_notes_failure="fallback" returns StubBody/KindDegenerate, NOT a fallback
	// commit-subject list, and runs only the degenerate-check assemble (no git log).
	transport := &recordingTransport{body: "must never be returned"}
	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stdout: ""}, nil) // empty diff.
	sel := newSelector(t, r, transport)

	state := notes.SelectState{LastTag: "v1.0.0"}
	body, kind, err := sel.SelectBody(t.Context(), state, config.Config{Release: config.Release{OnNotesFailure: "fallback"}})
	if err != nil {
		t.Fatalf("SelectBody returned unexpected error: %v", err)
	}
	if body != notes.StubBody() {
		t.Errorf("body = %q, want StubBody %q (not a fallback)", body, notes.StubBody())
	}
	if kind != notes.KindDegenerate {
		t.Errorf("kind = %v, want KindDegenerate", kind)
	}

	invs := r.Invocations()
	if len(invs) != 1 {
		t.Fatalf("git invocations = %d, want 1 (degenerate-check assemble only, no fallback log)", len(invs))
	}
	assertGitArgv(t, invs[0], wantArgs("v1.0.0"))
}
