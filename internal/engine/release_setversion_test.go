package engine_test

// This file holds the Phase 4 --set-version (explicit version) validation tests
// (task 4-6). --set-version PINS the next version outright rather than computing it
// from a bump flag: the engine parses the value as strict 3-part SemVer (reusing the
// tag-grammar parser), gates it strictly-greater than the current latest tag ON TOP
// of the free-tag preflight check, and on success marks the run explicit so
// MINT_BUMP=explicit is injected. The malformed / equal / less rejections all fire
// in Stage 1 — before any preflight or mutation — so they only seed the read gates.

import (
	"testing"

	"mint/internal/engine"
	"mint/internal/notes"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// seedVersionReadGates scripts the Stage-1 read timeline up to and including
// `git tag --list` — exactly the calls made before --set-version is validated
// (resolve root, resolve branch, list tags). It is all an early --set-version
// rejection (malformed / equal / less) reaches before aborting, so nothing past the
// tag list is seeded.
func seedVersionReadGates(f *runner.FakeRunner, root, releaseBranch, tagList string) {
	f.SeedSequence("git",
		ScriptedOut(root),                    // rev-parse --show-toplevel
		ScriptedOut("origin/"+releaseBranch), // symbolic-ref --short origin/HEAD
		ScriptedOut(tagList),                 // tag --list
	)
}

// setVersionOptions returns the explicit-version run options with the fixed clock
// and a NotesBody override (so the version-validation tests never wire the AI; the
// focus here is version selection, not notes).
func setVersionOptions(value string) engine.ReleaseOptions {
	return engine.ReleaseOptions{
		SetVersion: value,
		Now:        fixedClock,
		NotesBody:  "Explicit release.",
		NotesKind:  notes.KindFirstRelease,
	}
}

// stageFailureMessage returns the Message of the first recorded StageFailed event.
func stageFailureMessage(t *testing.T, rec *presentertest.RecordingPresenter) string {
	t.Helper()
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			return ev.StageFailed.Message
		}
	}
	t.Fatalf("no StageFailed event recorded; kinds = %v", rec.Kinds())
	return ""
}

// TestRelease_SetVersion_Malformed_Rejected proves a non-strict-3-part value is
// rejected in Stage 1 (before preflight or mutation): each malformed shape aborts
// non-zero with a "version" StageFailed and performs no mutation.
func TestRelease_SetVersion_Malformed_Rejected(t *testing.T) {
	t.Parallel()

	malformed := []string{
		"2.0",        // too few segments
		"2.0.0.1",    // too many segments
		"2.0.0-rc.1", // pre-release
		"2.0.0+b5",   // build metadata
		"abc",        // non-numeric
	}
	for _, value := range malformed {
		value := value
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			f := runner.NewFakeRunner()
			seedVersionReadGates(f, root, "main", "v1.2.3\n")
			rec := &presentertest.RecordingPresenter{}

			err := engine.Release(t.Context(), newDeps(rec, f), setVersionOptions(value))

			assertAbortNonZero(t, err)
			if name := stageFailedName(t, rec); name != "version" {
				t.Errorf("StageFailed.Name = %q, want %q", name, "version")
			}
			assertNoMutation(t, f)
		})
	}
}

// TestRelease_SetVersion_EqualToLatest_Rejected proves a value EQUAL to the current
// latest tag is rejected even though the target tag would be free — a non-forward
// jump corrupts tag-as-truth, so it aborts in Stage 1 with the strictly-greater
// message before any preflight or mutation.
func TestRelease_SetVersion_EqualToLatest_Rejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedVersionReadGates(f, root, "main", "v1.2.3\n")
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), setVersionOptions("1.2.3"))

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "version" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "version")
	}
	want := "--set-version 1.2.3 must be greater than the current latest version 1.2.3"
	if got := stageFailureMessage(t, rec); got != want {
		t.Errorf("StageFailed.Message = %q, want %q", got, want)
	}
	assertNoMutation(t, f)
}

// TestRelease_SetVersion_LessThanLatest_Rejected proves a value LESS than the
// current latest tag is rejected even though the lower target tag would be free:
// a backwards jump sorts below latest, so it aborts in Stage 1.
func TestRelease_SetVersion_LessThanLatest_Rejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedVersionReadGates(f, root, "main", "v1.2.3\n")
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), setVersionOptions("1.2.2"))

	assertAbortNonZero(t, err)
	if name := stageFailedName(t, rec); name != "version" {
		t.Errorf("StageFailed.Name = %q, want %q", name, "version")
	}
	want := "--set-version 1.2.2 must be greater than the current latest version 1.2.3"
	if got := stageFailureMessage(t, rec); got != want {
		t.Errorf("StageFailed.Message = %q, want %q", got, want)
	}
	assertNoMutation(t, f)
}

// TestRelease_SetVersion_GreaterThanLatest_BecomesNext drives a full spine where
// --set-version 2.0.0 is strictly greater than the latest tag v1.2.3: the run
// succeeds and the PINNED version becomes the next version — the annotated tag is
// v2.0.0 (not a computed patch v1.2.4) and the run reaches RunFinished.
func TestRelease_SetVersion_GreaterThanLatest_BecomesNext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// Read gates: a prior tag v1.2.3 exists; the explicit target v2.0.0 is free.
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut("v1.2.3\n"),      // tag --list (a prior tag exists)
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain (clean)
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v2.0.0 (absent)
		ScriptedOut("0\t1"),          // rev-list left-right count (ahead only)
		ScriptedOut(""),              // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v2.0.0 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v2.0.0
	)
	f.Seed("gh", runner.Result{}, nil) // gh auth status, then gh release create
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), setVersionOptions("2.0.0"))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	// The PINNED version became the next version: the annotated tag is v2.0.0.
	if !invokedWith(f, "git", "tag", "-a", "v2.0.0", "-F", "-") {
		t.Errorf("annotated tag was not v2.0.0; got %v", commandLines(f.Invocations()))
	}
	// A computed patch (v1.2.4) must NOT have been tagged.
	if invokedWith(f, "git", "tag", "-a", "v1.2.4", "-F", "-") {
		t.Errorf("computed patch v1.2.4 was tagged; --set-version must bypass Next")
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("--set-version run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_SetVersion_FirstRelease_Accepted proves that on a first release
// (no tags → latest 0.0.0) any valid 3-part version greater than 0.0.0 is accepted:
// --set-version 1.0.0 on a tagless repo tags v1.0.0 and finishes.
func TestRelease_SetVersion_FirstRelease_Accepted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	// No tags: latest resolves to 0.0.0; the explicit target v1.0.0 is free.
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut(""),              // tag --list (no tags)
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain (clean)
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v1.0.0 (absent)
		ScriptedOut("0\t1"),          // rev-list left-right count (ahead only)
		ScriptedOut(""),              // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v1.0.0 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v1.0.0
	)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), setVersionOptions("1.0.0"))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !invokedWith(f, "git", "tag", "-a", "v1.0.0", "-F", "-") {
		t.Errorf("first-release explicit tag was not v1.0.0; got %v", commandLines(f.Invocations()))
	}
	fin, _ := rec.At(len(rec.Events) - 1)
	if fin.Kind != presentertest.KindRunFinished {
		t.Errorf("first-release --set-version run did not finish; last event = %v", fin.Kind)
	}
}

// TestRelease_SetVersion_InjectsMintBumpExplicit drives a full spine with a
// configured preflight hook and proves a successful --set-version sets the bump kind
// to explicit: the hook's injected env carries MINT_BUMP=explicit (not patch).
func TestRelease_SetVersion_InjectsMintBumpExplicit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release.hooks]\npreflight = \"scripts/check.sh\"\n")

	f := runner.NewFakeRunner()
	f.SeedSequence("git",
		ScriptedOut(root),            // rev-parse --show-toplevel
		ScriptedOut("origin/main"),   // symbolic-ref --short origin/HEAD
		ScriptedOut("v1.2.3\n"),      // tag --list
		ScriptedOut(""),              // fetch --tags
		ScriptedOut(""),              // status --porcelain (clean)
		ScriptedOut("main"),          // rev-parse --abbrev-ref HEAD (on branch)
		ScriptedNonZero(),            // rev-parse -q --verify refs/tags/v2.0.0 (absent)
		ScriptedOut("0\t1"),          // rev-list left-right count (ahead only)
		ScriptedOut(""),              // ls-remote --tags (tag free remote)
		ScriptedOut(startingSHA),     // rev-parse HEAD (capture the clean start)
		ScriptedOut(""),              // -C root add CHANGELOG.md
		ScriptedOut(""),              // -C root commit -m
		ScriptedOut(githubRemoteURL), // remote get-url origin (provider detection)
		ScriptedOut(""),              // tag -a v2.0.0 -F -
		ScriptedOut(""),              // push --atomic origin HEAD v2.0.0
	)
	f.Seed("sh", runner.Result{}, nil) // preflight hook exits zero
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	err := engine.Release(t.Context(), newDeps(rec, f), setVersionOptions("2.0.0"))
	if err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	env := hookEnvFor(t, f, "scripts/check.sh")
	if !sliceContains(env, "MINT_BUMP=explicit") {
		t.Errorf("preflight hook env = %v, want it to contain MINT_BUMP=explicit", env)
	}
}

// hookEnvFor returns the injected env of the first `sh -c <entry>` invocation, the
// shape every lifecycle hook runs as. It lets a spine test assert the MINT_* env a
// hook actually received.
func hookEnvFor(t *testing.T, f *runner.FakeRunner, entry string) []string {
	t.Helper()
	for _, inv := range f.Invocations() {
		if inv.Name == "sh" && len(inv.Args) == 2 && inv.Args[0] == "-c" && inv.Args[1] == entry {
			return inv.Env
		}
	}
	t.Fatalf("no `sh -c %q` invocation recorded; got %v", entry, commandLines(f.Invocations()))
	return nil
}

// sliceContains reports whether want is present in env.
func sliceContains(env []string, want string) bool {
	for _, e := range env {
		if e == want {
			return true
		}
	}
	return false
}

// assertVersionBumpExplicitDistinct guards the enum: BumpExplicit must be distinct
// from the computed bumps so the engine can route on it.
func assertVersionBumpExplicitDistinct(t *testing.T) {
	t.Helper()
	if version.BumpExplicit == version.BumpPatch ||
		version.BumpExplicit == version.BumpMinor ||
		version.BumpExplicit == version.BumpMajor {
		t.Errorf("version.BumpExplicit collides with a computed bump value")
	}
}

func TestVersionBumpExplicit_Distinct(t *testing.T) {
	t.Parallel()
	assertVersionBumpExplicitDistinct(t)
}
