package engine_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/engine"
	"mint/internal/git"
	"mint/internal/runner"
)

// regenChangelogDate is the fixed section-header date the single-version regenerate
// tests inject, so the `## [x.y.z] - YYYY-MM-DD` header is fully deterministic and
// the in-place replace / no-net-change behaviour is exactly assertable.
func regenChangelogDate() time.Time {
	return time.Date(2026, time.June, 11, 0, 0, 0, 0, time.UTC)
}

// kacPreamble is the canonical Keep a Changelog 1.1.0 header preamble the changelog
// writer emits. The regenerate tests seed it verbatim so the in-place replace can be
// asserted against the exact surrounding file.
const kacPreamble = `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
`

func seedChangelog(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("seeding CHANGELOG.md: %v", err)
	}
}

func readChangelogFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}
	return string(data)
}

// regenMutator wraps a FakeRunner in a *git.Mutator with the happy-path defaults so
// each git mutation runs once through the recorded runner (no real git).
func regenMutator(f *runner.FakeRunner) *git.Mutator {
	return git.NewMutator(f)
}

func TestRegenerateChangelog_ExistingVersion_ReplacedInPlaceNoDuplicate(t *testing.T) {
	t.Parallel()

	// A single-version regenerate replaces the target version's section IN PLACE under
	// its `## [x.y.z] - date` header — not duplicated, not appended. The surrounding
	// sections keep their order and only the matched block's body changes, leaving
	// exactly one section for the version.
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+
		"\n## [0.0.2] - 2026-06-11\n\nOld second body.\n"+
		"## [0.0.1] - 2026-06-01\n\nInitial release.\n")

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	committed, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "0.0.2", "v0.0.2", regenChangelogDate(), "New second body.")
	if err != nil {
		t.Fatalf("RegenerateChangelog returned unexpected error: %v", err)
	}
	if !committed {
		t.Errorf("committed = false, want true (the matched section body changed)")
	}

	want := kacPreamble +
		"\n## [0.0.2] - 2026-06-11\n\nNew second body.\n" +
		"## [0.0.1] - 2026-06-01\n\nInitial release.\n"
	if got := readChangelogFile(t, dir); got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant\n%q", got, want)
	}
	if got := strings.Count(readChangelogFile(t, dir), "## [0.0.2]"); got != 1 {
		t.Errorf("section [0.0.2] appears %d times, want exactly 1 (no duplicate, no append)", got)
	}
}

func TestRegenerateChangelog_CommitSubjectIsRegenerateNotForward(t *testing.T) {
	t.Parallel()

	// The single CHANGELOG commit subject is EXACTLY `docs(changelog): regenerate
	// notes for {tag}` (using the canonical tag string) — it does NOT reuse the
	// forward `{commit_prefix} Release {tag}` subject, because nothing is being
	// released. Exactly one `git add CHANGELOG.md` then one `git commit` is staged.
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.4.0] - 2026-01-01\n\nStale body.\n")

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	committed, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "1.4.0", "v1.4.0", regenChangelogDate(), "Refreshed body.")
	if err != nil {
		t.Fatalf("RegenerateChangelog returned unexpected error: %v", err)
	}
	if !committed {
		t.Errorf("committed = false, want true (content changed)")
	}

	invs := r.Invocations()
	if len(invs) != 2 {
		t.Fatalf("invocations = %d, want 2 (git add then git commit)\n got: %v", len(invs), invs)
	}

	wantAdd := []string{"-C", dir, "add", "CHANGELOG.md"}
	if add := invs[0]; add.Name != "git" || !slices.Equal(add.Args, wantAdd) {
		t.Errorf("stage invocation = %s %v, want git %v (changelog only)", add.Name, add.Args, wantAdd)
	}

	wantCommit := []string{"-C", dir, "commit", "-m", "docs(changelog): regenerate notes for v1.4.0"}
	if commit := invs[1]; commit.Name != "git" || !slices.Equal(commit.Args, wantCommit) {
		t.Errorf("commit invocation = %s %v, want git %v", commit.Name, commit.Args, wantCommit)
	}

	subject := invs[1].Args[len(invs[1].Args)-1]
	if strings.Contains(subject, "Release ") {
		t.Errorf("commit subject = %q, must NOT reuse the forward `{commit_prefix} Release {tag}` subject", subject)
	}
}

func TestRegenerateChangelog_NoNetChange_MakesNoCommit(t *testing.T) {
	t.Parallel()

	// No-op safety: when the regenerated body is byte-identical to the version's
	// existing section (same body, same injected date) the in-place replace produces
	// NO net change to the file, so NO commit is staged — mint never makes an empty
	// commit. The runner is never touched.
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+
		"\n## [0.0.2] - 2026-06-11\n\nUnchanged body.\n"+
		"## [0.0.1] - 2026-06-01\n\nInitial release.\n")
	before := readChangelogFile(t, dir)

	r := runner.NewFakeRunner()

	committed, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "0.0.2", "v0.0.2", regenChangelogDate(), "Unchanged body.")
	if err != nil {
		t.Fatalf("RegenerateChangelog returned unexpected error: %v", err)
	}
	if committed {
		t.Errorf("committed = true, want false (a no-net-change write must not commit)")
	}
	if got := len(r.Invocations()); got != 0 {
		t.Errorf("invocations = %d, want 0 (no add/commit on a no-net-change write)\n got: %v", got, r.Invocations())
	}
	if got := readChangelogFile(t, dir); got != before {
		t.Errorf("CHANGELOG.md changed on a no-net-change write:\n%q\nwant unchanged\n%q", got, before)
	}
}

func TestRegenerateChangelog_NeverCutsTag(t *testing.T) {
	t.Parallel()

	// Regenerate touches only the mutable surface: no `git tag` is ever issued in any
	// case — only the at-most-one CHANGELOG add + commit. The tag is immutable history.
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [2.0.0] - 2026-02-02\n\nOld body.\n")

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	if _, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "2.0.0", "v2.0.0", regenChangelogDate(), "New body."); err != nil {
		t.Fatalf("RegenerateChangelog returned unexpected error: %v", err)
	}

	for _, inv := range r.Invocations() {
		for _, arg := range inv.Args {
			if arg == "tag" {
				t.Errorf("a `git tag` invocation was issued (%s %v); regenerate must never cut a tag", inv.Name, inv.Args)
			}
		}
	}
}

func TestRegenerateChangelog_AtMostOneCommit(t *testing.T) {
	t.Parallel()

	// At most ONE CHANGELOG commit is staged: a single `git commit` invocation across
	// the whole write — no hook-artifact commit (hooks don't run on regenerate) and no
	// second changelog commit.
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [3.1.0] - 2026-03-03\n\nBefore.\n")

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	if _, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "3.1.0", "v3.1.0", regenChangelogDate(), "After."); err != nil {
		t.Fatalf("RegenerateChangelog returned unexpected error: %v", err)
	}

	commits := 0
	for _, inv := range r.Invocations() {
		if slices.Contains(inv.Args, "commit") {
			commits++
		}
	}
	if commits != 1 {
		t.Errorf("git commit invocations = %d, want at most 1", commits)
	}
}

func TestRegenerateChangelog_CreatesAbsentFile(t *testing.T) {
	t.Parallel()

	// When CHANGELOG.md is absent the in-place writer creates it with the Keep a
	// Changelog preamble + the target version's section (the create-if-absent arm of
	// the reused writer), then stages the single regenerate commit.
	dir := t.TempDir()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{}, nil)

	committed, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "0.0.1", "v0.0.1", regenChangelogDate(), "Initial release.")
	if err != nil {
		t.Fatalf("RegenerateChangelog returned unexpected error: %v", err)
	}
	if !committed {
		t.Errorf("committed = false, want true (the file was created)")
	}

	want := kacPreamble + "\n## [0.0.1] - 2026-06-11\n\nInitial release.\n"
	if got := readChangelogFile(t, dir); got != want {
		t.Errorf("CHANGELOG.md =\n%q\nwant\n%q", got, want)
	}
}

func TestRegenerateChangelog_StageFails_SurfacesErrorBeforeCommit(t *testing.T) {
	t.Parallel()

	// A non-zero `git add` exit (the changelog DID change, so a stage is attempted) is
	// surfaced so 5-9 can reset the local write, and the commit must NOT run after a
	// failed stage.
	dir := t.TempDir()
	seedChangelog(t, dir, kacPreamble+"\n## [1.0.0] - 2026-01-01\n\nOld.\n")

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{Stderr: "fatal: pathspec error\n", ExitCode: 128}, errors.New("exit status 128"))

	committed, err := engine.RegenerateChangelog(t.Context(), regenMutator(r), dir, "1.0.0", "v1.0.0", regenChangelogDate(), "New.")
	if err == nil {
		t.Fatal("RegenerateChangelog returned nil error, want the git add failure surfaced")
	}
	if committed {
		t.Errorf("committed = true, want false on a failed stage")
	}
	if got := len(r.Invocations()); got != 1 {
		t.Errorf("invocations = %d, want 1 (commit must not run after a failed stage)", got)
	}
}
