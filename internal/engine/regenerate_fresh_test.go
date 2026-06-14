package engine_test

import (
	"context"
	"errors"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/record"
	"mint/internal/runner"
	"mint/internal/version"
)

// This file pins task 5-6: the regenerate FRESH SOURCE — re-diff vX-1..vX + AI notes.
// The fresh path REUSES the forward notes engine (assembly + exclusion tiers + AI
// transport) but over 5-3's resolved `{PreviousTag}..{Tag}` range instead of the
// forward `last_tag..HEAD`. The oldest release (FirstRelease) emits the fixed
// "Initial release." body with NO AI and NO diff. The single source of every test
// here is FakeRunner scripting git + a recording transport scripting the AI body.

// freshTransport is a recording fake for the notes.Transport seam used by the fresh
// path: it captures every prompt and returns a scripted body/error so the engine
// tests assert the composed prompt (Change Map prepended) and the body passthrough
// without scripting a real ai_command through the runner.
type freshTransport struct {
	body    string
	err     error
	prompts []string
}

func (ft *freshTransport) Generate(_ context.Context, prompt string) (string, error) {
	ft.prompts = append(ft.prompts, prompt)
	return ft.body, ft.err
}

func (ft *freshTransport) calls() int { return len(ft.prompts) }

// freshCfg is the default config for the fresh-source tests: a generous
// max_diff_lines ceiling and the default tag prefix, no prompt-control knobs.
func freshCfg() config.Config {
	return config.Config{
		MaxDiffLines: 50000,
		Release:      config.Release{TagPrefix: "v"},
	}
}

// seedFreshGit scripts the three ordered git calls the fresh AI path makes:
// AssembleRange's `git diff` first, then BuildChangeMapForRange's `--name-status`,
// then `--numstat`, IN THAT ORDER.
func seedFreshGit(diff, nameStatus, numstat string) *runner.FakeRunner {
	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{Stdout: diff}},
		runner.ScriptedCall{Result: runner.Result{Stdout: nameStatus}},
		runner.ScriptedCall{Result: runner.Result{Stdout: numstat}},
	)
	return f
}

func TestRegenerateFreshBody_DiffsResolvedRangeNotLastTagHEAD(t *testing.T) {
	t.Parallel()

	// Fresh notes are generated from the vX-1..vX range, NOT last_tag..HEAD: the diff
	// git argv must carry the resolved DiffRange verbatim.
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	body := "## TL;DR\n\nShipped auth.\n"
	f := seedFreshGit(diff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	tr := &freshTransport{body: body}

	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	got, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res)
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	if got != body {
		t.Errorf("body = %q, want the AI body %q", got, body)
	}

	wantDiffArgs := []string{"diff", "v1.3.0..v1.4.0", "--", ".", ":(exclude)CHANGELOG.md"}
	if !invokedWith(f, "git", wantDiffArgs...) {
		t.Errorf("git diff argv not found; want %q in %v", wantDiffArgs, f.Invocations())
	}
	// And NOT a forward last_tag..HEAD range.
	if invokedWith(f, "git", "diff", "v1.4.0..HEAD", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Error("fresh path diffed last_tag..HEAD; want the resolved vX-1..vX range")
	}
}

func TestRegenerateFreshBody_AlwaysExcludesChangelog(t *testing.T) {
	t.Parallel()

	// CHANGELOG.md is ALWAYS excluded from the regenerate diff via the :(exclude)
	// pathspec, even when no diff_exclude globs and no version_file are configured.
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	f := seedFreshGit(diff, "M\tapi.go\n", "1\t1\tapi.go\n")
	tr := &freshTransport{body: "notes"}

	res := version.Resolution{Tag: "v2.0.0", PreviousTag: "v1.0.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	wantDiffArgs := []string{"diff", "v1.0.0..v2.0.0", "--", ".", ":(exclude)CHANGELOG.md"}
	if !invokedWith(f, "git", wantDiffArgs...) {
		t.Errorf("CHANGELOG.md exclude pathspec missing; want %q in %v", wantDiffArgs, f.Invocations())
	}
}

func TestRegenerateFreshBody_PlainModeExcludesVersionFile(t *testing.T) {
	t.Parallel()

	// PLAIN mode (version_file set, NO version_pattern): the strategy excludes the
	// whole-file version. The fresh diff argv carries :(exclude)CHANGELOG.md AND
	// :(exclude)<version_file>, reproducing the forward source view over the range.
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	f := seedFreshGit(diff, "M\tapi.go\n", "1\t1\tapi.go\n")
	tr := &freshTransport{body: "notes"}

	cfg := freshCfg()
	cfg.Release.VersionFile = "release.txt"
	res := version.Resolution{Tag: "v2.0.0", PreviousTag: "v1.0.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), cfg, res); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	want := []string{"diff", "v1.0.0..v2.0.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)release.txt"}
	if !invokedWith(f, "git", want...) {
		t.Errorf("plain-mode version_file not excluded; want %q in %v", want, f.Invocations())
	}
}

func TestRegenerateFreshBody_EmbeddedModeDoesNotExcludeVersionFile(t *testing.T) {
	t.Parallel()

	// EMBEDDED mode (version_file + version_pattern): the version line is in real
	// source we WANT in the notes, so the strategy does NOT exclude it. The fresh diff
	// argv carries :(exclude)CHANGELOG.md but NO :(exclude)<version_file>.
	diff := "diff --git a/main.go b/main.go\n@@ -1 +1 @@\n-old\n+new\n"
	f := seedFreshGit(diff, "M\tmain.go\n", "1\t1\tmain.go\n")
	tr := &freshTransport{body: "notes"}

	cfg := freshCfg()
	cfg.Release.VersionFile = "main.go"
	cfg.Release.VersionPattern = `RELEASE_VERSION="{version}"`
	res := version.Resolution{Tag: "v2.0.0", PreviousTag: "v1.0.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), cfg, res); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	// CHANGELOG.md is excluded; main.go is NOT.
	if !invokedWith(f, "git", "diff", "v1.0.0..v2.0.0", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Errorf("embedded-mode diff argv wrong; want CHANGELOG.md exclude only, got %v", f.Invocations())
	}
	if invokedWith(f, "git", "diff", "v1.0.0..v2.0.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)main.go") {
		t.Error("embedded-mode version_file was excluded; it must NOT be (real source we want in notes)")
	}
}

func TestRegenerateFreshBody_PathExclusionEvenWithBookkeepingCommitInRange(t *testing.T) {
	t.Parallel()

	// The vX-1..vX range ALREADY CONTAINS mint's bookkeeping commit. Exclusion is
	// PATH-based: the diff argv carries the full range and the path exclude pathspecs
	// (CHANGELOG.md + plain version_file) — there is NO attempt to subtract the commit
	// (no `^{commit}`, no `--not`, no extra revision in the argv).
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	f := seedFreshGit(diff, "M\tapi.go\n", "1\t1\tapi.go\n")
	tr := &freshTransport{body: "notes"}

	cfg := freshCfg()
	cfg.Release.VersionFile = "release.txt"
	res := version.Resolution{Tag: "v1.5.0", PreviousTag: "v1.4.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), cfg, res); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	// The diff invocation is the full range + path excludes, nothing more.
	want := []string{"diff", "v1.4.0..v1.5.0", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude)release.txt"}
	if !invokedWith(f, "git", want...) {
		t.Errorf("path-based exclusion argv wrong; want %q in %v", want, f.Invocations())
	}
	for _, inv := range f.Invocations() {
		if inv.Name != "git" {
			continue
		}
		for _, a := range inv.Args {
			if a == "--not" || a == "v1.5.0^" {
				t.Errorf("fresh path attempted commit-based subtraction (%q); exclusion must be path-based", a)
			}
		}
	}
}

func TestRegenerateFreshBody_PrependsChangeMapComputedAfterExclusion(t *testing.T) {
	t.Parallel()

	// The Change Map is computed AFTER exclusion (the same exclude pathspecs ride on
	// the name-status/numstat calls) and PREPENDED to the AI input — the composed
	// prompt carries the map BEFORE the diff.
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	f := seedFreshGit(diff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	tr := &freshTransport{body: "notes"}

	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	if tr.calls() != 1 {
		t.Fatalf("transport called %d times, want 1", tr.calls())
	}
	prompt := tr.prompts[0]
	mapIdx := indexOfSub(prompt, "New package/dir: auth/")
	diffIdx := indexOfSub(prompt, "diff --git a/auth/login.go")
	if mapIdx < 0 || diffIdx < 0 {
		t.Fatalf("prompt missing Change Map or diff; got:\n%s", prompt)
	}
	if mapIdx >= diffIdx {
		t.Errorf("Change Map (idx %d) must be prepended BEFORE the diff (idx %d)", mapIdx, diffIdx)
	}

	// Change Map calls also carry the exclude pathspecs (computed after exclusion).
	if !invokedWith(f, "git", "diff", "--name-status", "v1.3.0..v1.4.0", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Errorf("change map name-status missing exclude pathspec; got %v", f.Invocations())
	}
}

func TestRegenerateFreshBody_OldestReleaseEmitsInitialReleaseNoAINoDiff(t *testing.T) {
	t.Parallel()

	// The oldest release (FirstRelease) emits the fixed "Initial release." body with
	// NO AI and NO diff — mirroring the forward first-release rule. No git call and no
	// transport call may happen; an unseeded FakeRunner would surface any stray call.
	f := runner.NewFakeRunner()
	tr := &freshTransport{body: "should not be produced"}

	res := version.Resolution{Tag: "v0.1.0", FirstRelease: true}
	got, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res)
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	if got != record.FirstReleaseBody {
		t.Errorf("body = %q, want the fixed first-release body %q", got, record.FirstReleaseBody)
	}
	if got != "Initial release." {
		t.Errorf("body = %q, want %q", got, "Initial release.")
	}
	if len(f.Invocations()) != 0 {
		t.Errorf("first-release path made %d git calls, want 0 (no diff)", len(f.Invocations()))
	}
	if tr.calls() != 0 {
		t.Errorf("first-release path called the AI %d times, want 0", tr.calls())
	}
}

func TestRegenerateFreshBody_AppliesMaxDiffLinesGuard(t *testing.T) {
	t.Parallel()

	// max_diff_lines behaves as the forward path: an over-ceiling range diff returns
	// ErrDiffTooLarge (wrapped) and the AI is NEVER called.
	diff := "l1\nl2\nl3\nl4\n"
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: diff}, nil)
	tr := &freshTransport{body: "should not be produced"}

	cfg := freshCfg()
	cfg.MaxDiffLines = 2
	res := version.Resolution{Tag: "v1.1.0", PreviousTag: "v1.0.0"}
	_, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), cfg, res)
	if err == nil {
		t.Fatal("RegenerateFreshBody returned nil error, want ErrDiffTooLarge")
	}
	if !errors.Is(err, notes.ErrDiffTooLarge) {
		t.Errorf("error = %v, want it to match notes.ErrDiffTooLarge", err)
	}
	if tr.calls() != 0 {
		t.Errorf("transport called %d times, want 0 — the AI must NOT run on an over-ceiling diff", tr.calls())
	}
}

func TestRegenerateFreshBody_DegenerateRangeReturnsStubNoAI(t *testing.T) {
	t.Parallel()

	// The forward degenerate guard (SelectBody -> IsDegenerate -> StubBody) ALSO covers
	// the fresh producer: an empty/all-excluded post-exclusion vX-1..vX diff returns
	// notes.StubBody() with NO transport call. The fresh range always carries mint's
	// release-bookkeeping commit, so a version whose only non-excluded change was that
	// bookkeeping yields exactly this empty diff — and the AI is never run on it. This is
	// the function cmd wires as ProduceBody for BOTH single fresh and --all fresh.
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: "\n   \t\n"}, nil)
	tr := &freshTransport{body: "must never be produced"}

	res := version.Resolution{Tag: "v1.5.0", PreviousTag: "v1.4.0"}
	got, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res)
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	if got != notes.StubBody() {
		t.Errorf("body = %q, want the degenerate StubBody %q", got, notes.StubBody())
	}
	if tr.calls() != 0 {
		t.Errorf("transport called %d times, want 0 — the AI must NOT run on a degenerate diff", tr.calls())
	}
}

func TestRegenerateFreshRegenerator_DegenerateRangeReturnsStubNoAI(t *testing.T) {
	t.Parallel()

	// The fresh `r` regenerator inherits the SAME degenerate guard: re-running over an
	// empty post-exclusion range yields StubBody() with no transport call, regardless of
	// the user's one-time context — the degenerate signal pre-empts the AI before any
	// prompt is composed.
	f := runner.NewFakeRunner()
	f.Seed("git", runner.Result{Stdout: ""}, nil)
	tr := &freshTransport{body: "must never be produced"}

	res := version.Resolution{Tag: "v1.5.0", PreviousTag: "v1.4.0"}
	regen := engine.RegenerateFreshRegenerator(f, tr, t.TempDir(), freshCfg(), res)
	got, err := regen.Regenerate(t.Context(), "lead with auth")
	if err != nil {
		t.Fatalf("Regenerate returned unexpected error: %v", err)
	}

	if got != notes.StubBody() {
		t.Errorf("body = %q, want the degenerate StubBody %q", got, notes.StubBody())
	}
	if tr.calls() != 0 {
		t.Errorf("transport called %d times, want 0 — the AI must NOT run on a degenerate diff", tr.calls())
	}
}

func TestRegenerateFreshBody_SurfacesAIFailure(t *testing.T) {
	t.Parallel()

	// Single-mode fresh keeps the AI failure SURFACED (wrapped, errors.Is matches) so
	// the on_notes_failure default abort applies and 5-12's --all can intercept it for
	// skip-and-continue. The failure is NOT swallowed here.
	diff := "diff --git a/x.go b/x.go\n@@ -1 +1 @@\n-a\n+b\n"
	f := seedFreshGit(diff, "M\tx.go\n", "1\t1\tx.go\n")
	tr := &freshTransport{err: errFreshAIFailure}

	res := version.Resolution{Tag: "v1.1.0", PreviousTag: "v1.0.0"}
	_, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res)
	if err == nil {
		t.Fatal("RegenerateFreshBody returned nil error, want the AI failure surfaced")
	}
	if !errors.Is(err, errFreshAIFailure) {
		t.Errorf("error = %v, want it to wrap the AI failure", err)
	}
}

func TestRegenerateFreshRegenerator_ReRunsFreshAIRangeWithOneTimeContext(t *testing.T) {
	t.Parallel()

	// The fresh `r` regenerator re-runs the fresh AI path over the resolved vX-1..vX
	// range with the user's one-time context appended — the regenerate analogue of the
	// forward path's perRunRegenerator. It must range over the resolved DiffRange (NOT
	// last_tag..HEAD) and carry the context into the prompt.
	const oneTime = "Lead with the auth package."
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	f := seedFreshGit(diff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	tr := &freshTransport{body: "regenerated body"}

	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	regen := engine.RegenerateFreshRegenerator(f, tr, t.TempDir(), freshCfg(), res)
	got, err := regen.Regenerate(t.Context(), oneTime)
	if err != nil {
		t.Fatalf("Regenerate returned unexpected error: %v", err)
	}

	if got != "regenerated body" {
		t.Errorf("body = %q, want the AI body %q", got, "regenerated body")
	}
	if !invokedWith(f, "git", "diff", "v1.3.0..v1.4.0", "--", ".", ":(exclude)CHANGELOG.md") {
		t.Errorf("regenerator did not diff the resolved range v1.3.0..v1.4.0; got %v", f.Invocations())
	}
	if tr.calls() != 1 {
		t.Fatalf("transport called %d times, want 1", tr.calls())
	}
	if indexOfSub(tr.prompts[0], oneTime) < 0 {
		t.Errorf("prompt missing the one-time context %q:\n%s", oneTime, tr.prompts[0])
	}
}

func TestRegenerateFreshRegenerator_FirstRelease_ReturnsFixedBodyNoAI(t *testing.T) {
	t.Parallel()

	// The oldest release has no vX-1..vX range, so the regenerator mirrors the fresh
	// body's first-release rule: it returns the fixed "Initial release." body with NO AI
	// and NO diff — `r` on a first-release fresh gate never breaks.
	f := runner.NewFakeRunner()
	tr := &freshTransport{body: "should not be produced"}

	res := version.Resolution{Tag: "v0.1.0", FirstRelease: true}
	regen := engine.RegenerateFreshRegenerator(f, tr, t.TempDir(), freshCfg(), res)
	got, err := regen.Regenerate(t.Context(), "any context")
	if err != nil {
		t.Fatalf("Regenerate returned unexpected error: %v", err)
	}

	if got != record.FirstReleaseBody {
		t.Errorf("body = %q, want the fixed first-release body %q", got, record.FirstReleaseBody)
	}
	if len(f.Invocations()) != 0 {
		t.Errorf("first-release regenerator made %d git calls, want 0 (no diff)", len(f.Invocations()))
	}
	if tr.calls() != 0 {
		t.Errorf("first-release regenerator called the AI %d times, want 0", tr.calls())
	}
}

func TestRegenerateFreshBody_ReleaseVerbOverrideDrivesAIInvocation(t *testing.T) {
	t.Parallel()

	// THE key proof for the easy-miss site: with transport=nil (the PRODUCTION path),
	// resolveFreshTransport builds the REAL ai.Transport over the FakeRunner and MUST
	// resolve the AI command through the [release] verb. A [release].ai_command override
	// ("mybot gen --json") is therefore the binary+args the fresh-regenerate AI call
	// invokes — proving the construction site routes through config.VerbRelease, not the
	// bare shared/default. The composed prompt reaches mybot on stdin; the default
	// `claude` is never invoked.
	diff := "diff --git a/auth/login.go b/auth/login.go\n@@ -0,0 +1 @@\n+package auth\n"
	body := "## TL;DR\n\nShipped auth.\n"
	f := seedFreshGit(diff, "A\tauth/login.go\n", "20\t0\tauth/login.go\n")
	// The [release].ai_command override's BINARY (mybot) is the runner key — the transport
	// whitespace-splits "mybot gen --json" into name + args. Seeding the default `claude`
	// here would never be reached; an unseeded `mybot` would error the run, so the seed
	// proves the [release] override (not the default) is what gets invoked.
	f.Seed("mybot", runner.Result{Stdout: body}, nil)

	override := "mybot gen --json"
	cfg := freshCfg()
	cfg.Release.AICommand = &override
	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}

	got, err := engine.RegenerateFreshBody(t.Context(), f, nil, t.TempDir(), cfg, res)
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want the AI body %q", got, body)
	}

	// The assembled prompt reached the release-resolved command on stdin.
	if stdinOf(t, f, "mybot", "gen", "--json") == "" {
		t.Errorf("[release].ai_command %q was not invoked with a prompt on stdin", override)
	}
	// The default `claude` was never invoked — the fresh-regenerate call resolved [release].
	if invokedWith(f, "claude", "-p", "--model", "sonnet") {
		t.Error("default `claude -p --model sonnet` was invoked despite a [release].ai_command override; fresh-regenerate did not route through [release]")
	}
}

func TestRegenerateFreshBody_CommitOverrideDoesNotDriveAIInvocation(t *testing.T) {
	t.Parallel()

	// Independence proof: regenerate reads [release], NEVER [commit]. With a
	// [commit].ai_command override but NO [release] and NO shared override, the
	// fresh-regenerate call resolves to the FLOOR (claude -p --model sonnet), NOT
	// `wrongbot` — the [commit] override cannot leak into the release-routed site.
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	body := "notes"
	f := seedFreshGit(diff, "M\tapi.go\n", "1\t1\tapi.go\n")
	// Only the floor `claude` is seeded. If the site mistakenly read [commit], it would
	// invoke the unseeded `wrongbot` and error the run — so a clean run proves [commit]
	// did not drive the call.
	f.Seed("claude", runner.Result{Stdout: body}, nil)

	wrong := "wrongbot"
	cfg := freshCfg()
	cfg.Commit.AICommand = &wrong
	res := version.Resolution{Tag: "v2.0.0", PreviousTag: "v1.0.0"}

	got, err := engine.RegenerateFreshBody(t.Context(), f, nil, t.TempDir(), cfg, res)
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want the AI body %q", got, body)
	}

	// The floor drove the call; the [commit] override never leaked in.
	if stdinOf(t, f, "claude", "-p", "--model", "sonnet") == "" {
		t.Errorf("floor `claude -p --model sonnet` was not invoked with a prompt on stdin")
	}
	if invokedWith(f, "wrongbot") {
		t.Error("[commit].ai_command `wrongbot` drove the fresh-regenerate call; regenerate must read [release], never [commit]")
	}
}

func TestRegenerateFreshBody_ZeroConfigResolvesToClaudeFloor(t *testing.T) {
	t.Parallel()

	// A zero-config fresh-regenerate (no [release]/[commit]/shared ai_command) resolves to
	// the shipped floor `claude -p --model sonnet` — the production path builds the real
	// transport over the FakeRunner and invokes exactly the pinned default argv.
	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	body := "notes"
	f := seedFreshGit(diff, "M\tapi.go\n", "1\t1\tapi.go\n")
	f.Seed("claude", runner.Result{Stdout: body}, nil)

	res := version.Resolution{Tag: "v2.0.0", PreviousTag: "v1.0.0"}
	got, err := engine.RegenerateFreshBody(t.Context(), f, nil, t.TempDir(), freshCfg(), res)
	if err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want the AI body %q", got, body)
	}

	if stdinOf(t, f, "claude", "-p", "--model", "sonnet") == "" {
		t.Errorf("zero-config fresh-regenerate did not invoke the floor `claude -p --model sonnet` with a prompt on stdin")
	}
}

var errFreshAIFailure = errors.New("ai notes generation failed")

// indexOfSub returns the byte index of sub in s, or -1 when absent.
func indexOfSub(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
