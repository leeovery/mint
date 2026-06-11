package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/runner"
)

// This file pins task 5-5: the regenerate --reuse SOURCE read — reading a tag's
// annotation body back via ONE deterministic git call and using it WHOLE (no
// parse). The body is the single source mint ever reads, written by the forward
// path's annotated tag (`git tag -a … -F -`) as subject + blank line + body, so
// `git for-each-ref --format=%(contents:body) refs/tags/<tag>` yields exactly the
// body part.
//
// ReadTagBody is the low-level read returning (body, hasBody, err) so 5-12's --all
// mode can branch on hasBody (skip-and-report); ReadReuseBody is single-mode's
// fail-loud wrapper that turns a missing body into the exact "use --fresh" error.

const reuseTag = "v1.4.0"

// forEachRefArgs is the exact git argv the reuse read must issue: a single
// for-each-ref with the contents:body format selector against refs/tags/<tag>.
func forEachRefArgs(tag string) []string {
	return []string{"for-each-ref", "--format=%(contents:body)", "refs/tags/" + tag}
}

// seedForEachRef scripts the single for-each-ref read's stdout.
func seedForEachRef(f *runner.FakeRunner, body string) {
	f.Seed("git", runner.Result{Stdout: body}, nil)
}

// TestReadTagBody_ReturnsBodyWhole proves a non-empty multi-line annotation body
// is read via the single for-each-ref call and returned VERBATIM — no parse, no
// split, no trimming of the returned body.
func TestReadTagBody_ReturnsBodyWhole(t *testing.T) {
	t.Parallel()

	body := "## What's Changed\n\n- Added the widget\n- Fixed the gadget\n"
	f := runner.NewFakeRunner()
	seedForEachRef(f, body)

	got, hasBody, err := engine.ReadTagBody(t.Context(), f, reuseTag)
	if err != nil {
		t.Fatalf("ReadTagBody returned %v, want nil", err)
	}
	if !hasBody {
		t.Errorf("hasBody = false, want true for a non-empty annotation body")
	}
	if got != body {
		t.Errorf("body = %q, want it returned verbatim %q", got, body)
	}
}

// TestReadTagBody_ExactGitArgv proves the read issues EXACTLY one for-each-ref with
// the contents:body format selector against refs/tags/<tag> — the deterministic,
// parse-free read the spec pins.
func TestReadTagBody_ExactGitArgv(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	seedForEachRef(f, "body")

	if _, _, err := engine.ReadTagBody(t.Context(), f, reuseTag); err != nil {
		t.Fatalf("ReadTagBody returned %v, want nil", err)
	}

	invs := f.Invocations()
	if len(invs) != 1 {
		t.Fatalf("recorded %d invocations, want exactly 1 (one deterministic read)", len(invs))
	}
	if !invokedWith(f, "git", forEachRefArgs(reuseTag)...) {
		t.Errorf("git argv = %q, want %q", invs[0].Args, forEachRefArgs(reuseTag))
	}
}

// TestReadTagBody_NoBodyVariants proves the three "no annotation body" shapes — a
// lightweight tag (empty for-each-ref output), an empty annotated body, and a
// whitespace-only body — all surface as hasBody=false (trim and check empty), the
// single branch point 5-12 will skip on.
func TestReadTagBody_NoBodyVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
	}{
		{name: "lightweight tag (no annotation object)", output: ""},
		{name: "empty annotation body", output: "\n"},
		{name: "whitespace-only annotation body", output: "  \n\t \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			seedForEachRef(f, tt.output)

			_, hasBody, err := engine.ReadTagBody(t.Context(), f, reuseTag)
			if err != nil {
				t.Fatalf("ReadTagBody returned %v, want nil", err)
			}
			if hasBody {
				t.Errorf("hasBody = true, want false for %s", tt.name)
			}
		})
	}
}

// TestReadReuseBody_ReturnsBodyVerbatim proves single mode returns a non-empty body
// verbatim with no error — the reuse path hands it straight to the provider write.
func TestReadReuseBody_ReturnsBodyVerbatim(t *testing.T) {
	t.Parallel()

	body := "Release highlights\n\n- one\n- two\n"
	f := runner.NewFakeRunner()
	seedForEachRef(f, body)

	got, err := engine.ReadReuseBody(t.Context(), f, reuseTag)
	if err != nil {
		t.Fatalf("ReadReuseBody returned %v, want nil", err)
	}
	if got != body {
		t.Errorf("body = %q, want verbatim %q", got, body)
	}
}

// TestReadReuseBody_NoBodyFailsLoud proves single mode fails loud with the EXACT
// message (em-dash and the `use --fresh` hint) for every "no annotation body"
// shape — a lightweight tag, an empty body, and a whitespace-only body.
func TestReadReuseBody_NoBodyFailsLoud(t *testing.T) {
	t.Parallel()

	wantErr := "tag " + reuseTag + " has no annotation body — use --fresh"

	tests := []struct {
		name   string
		output string
	}{
		{name: "lightweight tag", output: ""},
		{name: "empty body", output: "\n"},
		{name: "whitespace-only body", output: "   \n\t\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := runner.NewFakeRunner()
			seedForEachRef(f, tt.output)

			body, err := engine.ReadReuseBody(t.Context(), f, reuseTag)
			if err == nil {
				t.Fatalf("ReadReuseBody returned nil error, want the no-annotation-body failure")
			}
			if err.Error() != wantErr {
				t.Errorf("error = %q, want exactly %q", err.Error(), wantErr)
			}
			if body != "" {
				t.Errorf("body = %q, want empty (no empty provider release body written)", body)
			}
		})
	}
}

// TestReadReuseBody_NoAIOrDiffOnReusePath proves the reuse read touches NOTHING but
// the single for-each-ref: no AI invocation and no diff assembly. The claude/gh
// binaries and every other git command are left unseeded, so any stray call would
// surface the FakeRunner's unseeded-command error; the read must still succeed.
func TestReadReuseBody_NoAIOrDiffOnReusePath(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	seedForEachRef(f, "verbatim body")

	if _, err := engine.ReadReuseBody(t.Context(), f, reuseTag); err != nil {
		t.Fatalf("ReadReuseBody returned %v, want nil", err)
	}

	for _, inv := range f.Invocations() {
		if inv.Name != "git" {
			t.Errorf("reuse path invoked %q; only the for-each-ref read is allowed (no AI, no diff)", inv.Name)
		}
		if len(inv.Args) == 0 || inv.Args[0] != "for-each-ref" {
			t.Errorf("reuse path ran git %q; only for-each-ref is allowed (no diff assembly)", inv.Args)
		}
	}
	if got := len(f.Invocations()); got != 1 {
		t.Errorf("reuse path made %d calls, want exactly 1 (the deterministic read)", got)
	}
}

// TestReadTagBody_GitError surfaces a genuine git failure (the binary missing or a
// non-zero exit) as an error rather than masking it as "no body".
func TestReadTagBody_GitError(t *testing.T) {
	t.Parallel()

	f := runner.NewFakeRunner()
	f.SeedNotFound("git")

	_, _, err := engine.ReadTagBody(t.Context(), f, reuseTag)
	if err == nil {
		t.Fatalf("ReadTagBody returned nil error, want the git read failure surfaced")
	}
	if !strings.Contains(err.Error(), reuseTag) {
		t.Errorf("error = %q, want it to name the tag %q", err.Error(), reuseTag)
	}
}
