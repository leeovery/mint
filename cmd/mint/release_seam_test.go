package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"mint/internal/notes"
	"mint/internal/notescache"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

const (
	releaseSeamNotes = "## Added\n- the export command\n"
	releaseSeamDiff  = "diff --git a/src/export.go b/src/export.go\n@@ -0,0 +1 @@\n+package export\n" +
		"diff --git a/docs/design/export.md b/docs/design/export.md\n@@ -0,0 +1 @@\n+# Export design\n"
	releaseSeamFallbackMove = `set on_notes_failure = "fallback" under [release] to ship the commit-subject list rather than stop`
)

// releaseSeamRun is one release through the seam, its repo, note cache and chunk-notes
// store each rooted in a temp dir of its own.
type releaseSeamRun struct {
	root, chunkNotesBase string
	f                    *runner.FakeRunner
	rec                  *presentertest.RecordingPresenter
}

func newReleaseSeamRun(t *testing.T) *releaseSeamRun {
	t.Helper()
	return &releaseSeamRun{
		root:           t.TempDir(),
		chunkNotesBase: t.TempDir(),
		f:              runner.NewFakeRunner(),
		rec:            &presentertest.RecordingPresenter{},
	}
}

func (run *releaseSeamRun) release(t *testing.T, stdinInteractive bool, args ...string) int {
	t.Helper()
	opts, err := parseReleaseFlags(args)
	if err != nil {
		t.Fatalf("parseReleaseFlags(%q) returned error: %v", args, err)
	}
	noteCache := notescache.NewStore(t.TempDir(), time.Now)
	return release(t.Context(), run.rec, run.f, opts, stdinInteractive, noteCache, notescache.NewChunkNotes(run.chunkNotesBase))
}

// seedReadGates scripts git up to the pre-gate HEAD capture of a release from v1.2.3
// on main, with release_branch unset so the branch is read from origin/HEAD.
func (run *releaseSeamRun) seedReadGates() {
	run.f.SeedSequence("git",
		gitOut(run.root),      // rev-parse --show-toplevel
		gitOut("origin/main"), // symbolic-ref --short refs/remotes/origin/HEAD
		gitOut("v1.2.3\n"),    // tag --list
		gitOut(""),            // fetch --tags
		gitOut(""),            // status --porcelain
		gitOut("main"),        // rev-parse --abbrev-ref HEAD
		gitNonZero(),          // rev-parse -q --verify refs/tags/v1.2.4
		gitOut("0\t1"),        // rev-list --left-right --count
		gitOut(""),            // ls-remote --tags
		gitOut("startsha"),    // rev-parse HEAD
	)
}

// seedNotesDiff scripts the notes diff and its Change Map.
func (run *releaseSeamRun) seedNotesDiff(diff, nameStatus, numstat string) {
	run.f.SeedSequence("git", gitOut(diff), gitOut(nameStatus), gitOut(numstat))
}

// seedRecordTagPush scripts the bookkeeping commit, the tag and the atomic push.
func (run *releaseSeamRun) seedRecordTagPush() {
	run.f.SeedSequence("git",
		gitOut(""),           // -C root add CHANGELOG.md
		gitOut(""),           // -C root commit -m
		gitOut(githubOrigin), // remote get-url origin
		gitOut(""),           // tag -a v1.2.4 -F -
		gitOut(""),           // push --atomic origin HEAD v1.2.4
	)
	run.f.Seed("gh", runner.Result{}, nil)
}

func (run *releaseSeamRun) writeConfig(t *testing.T, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(run.root, ".mint.toml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("writing .mint.toml: %v", err)
	}
}

func gitNonZero() runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}
}

// aiFailed is a notes call that exits non-zero with nothing on either stream. The
// transport retries a non-zero exit once, so a failing call takes two.
func aiFailed() runner.ScriptedCall {
	return runner.ScriptedCall{Result: runner.Result{ExitCode: 1}, Err: errors.New("exit status 1")}
}

func recordedGates(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindPrompt {
			gates = append(gates, ev.Prompt)
		}
	}
	return gates
}

func floorChoices(rec *presentertest.RecordingPresenter) []presenter.Gate {
	var gates []presenter.Gate
	for _, g := range recordedGates(rec) {
		if g.Has(presenter.ChoiceCommitSubjects) {
			gates = append(gates, g)
		}
	}
	return gates
}

func nextMoves(rec *presentertest.RecordingPresenter) []presenter.NextMoves {
	var moves []presenter.NextMoves
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindNextMoves {
			moves = append(moves, ev.NextMoves)
		}
	}
	return moves
}

func exclusionSuggestions(rec *presentertest.RecordingPresenter) []presenter.ExclusionSuggestions {
	var suggested []presenter.ExclusionSuggestions
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindExclusionSuggestions {
			suggested = append(suggested, ev.ExclusionSuggestions)
		}
	}
	return suggested
}

func stageFailures(rec *presentertest.RecordingPresenter) []presenter.StageFailure {
	var failures []presenter.StageFailure
	for _, ev := range rec.Events {
		if ev.Kind == presentertest.KindStageFailed {
			failures = append(failures, ev.StageFailed)
		}
	}
	return failures
}

// touchesConfig reports whether any git call named .mint.toml.
func touchesConfig(f *runner.FakeRunner) bool {
	for _, inv := range f.Invocations() {
		if inv.Name == "git" && slices.Contains(inv.Args, ".mint.toml") {
			return true
		}
	}
	return false
}

func TestRelease_YesAtATerminal_RaisesNoExclusionPauseAndRecordsNoAnswers(t *testing.T) {
	t.Parallel()

	run := newReleaseSeamRun(t)
	run.seedReadGates()
	run.seedNotesDiff(releaseSeamDiff, "A\tsrc/export.go\nA\tdocs/design/export.md\n", "1\t0\tsrc/export.go\n1\t0\tdocs/design/export.md\n")
	run.f.SeedSequence("claude",
		gitOut("docs/design/: design documents, not shipped behaviour\n"), // the exclusion check
		gitOut(releaseSeamNotes), // the notes
	)
	run.seedRecordTagPush()

	if code := run.release(t, true, "-y"); code != 0 {
		t.Fatalf("exit code = %d, want 0; events %v", code, run.rec.Kinds())
	}

	want := presenter.ExclusionSuggestion{Glob: "docs/design/", Line: `diff_exclude = ["docs/design/"]`}
	suggested := exclusionSuggestions(run.rec)
	if len(suggested) != 1 || len(suggested[0].Suggestions) != 1 ||
		suggested[0].Suggestions[0].Glob != want.Glob || suggested[0].Suggestions[0].Line != want.Line {
		t.Fatalf("ExclusionSuggestions = %+v, want one printing %+v", suggested, want)
	}
	for _, g := range recordedGates(run.rec) {
		if g.Has(presenter.ChoiceKeep) {
			t.Errorf("a -y run paused on a suggestion: %q", g.Question)
		}
	}
	if _, err := os.Stat(filepath.Join(run.root, ".mint.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".mint.toml stat error = %v, want it never written", err)
	}
	if touchesConfig(run.f) {
		t.Errorf("a -y run made the answers commit; got %+v", run.f.Invocations())
	}
}

// seedFailedNotes scripts a release whose exclusion check finds nothing and whose
// single notes call then fails, its retry with it.
func (run *releaseSeamRun) seedFailedNotes() {
	run.seedReadGates()
	run.seedNotesDiff(releaseSeamDiff, "A\tsrc/export.go\nA\tdocs/design/export.md\n", "1\t0\tsrc/export.go\n1\t0\tdocs/design/export.md\n")
	run.f.SeedSequence("claude", gitOut("none"), aiFailed(), aiFailed())
}

func TestRelease_NotesFailureWithStdinNotATerminal_StopsWithTheNextMoves(t *testing.T) {
	t.Parallel()

	run := newReleaseSeamRun(t)
	run.seedFailedNotes()

	if code := run.release(t, false); code != 1 {
		t.Fatalf("exit code = %d, want 1; events %v", code, run.rec.Kinds())
	}

	if gates := floorChoices(run.rec); len(gates) != 0 {
		t.Errorf("a non-TTY run was offered the floor's choice: %+v", gates)
	}
	moves := nextMoves(run.rec)
	if len(moves) != 1 || !slices.Contains(moves[0].Moves, releaseSeamFallbackMove) {
		t.Errorf("NextMoves = %+v, want one carrying %q", moves, releaseSeamFallbackMove)
	}
}

func TestRelease_NotesFailureAtATerminalWithoutYes_OffersTheFloorChoice(t *testing.T) {
	t.Parallel()

	run := newReleaseSeamRun(t)
	run.seedFailedNotes()

	if code := run.release(t, true); code != 1 {
		t.Fatalf("exit code = %d, want 1 on the default abort; events %v", code, run.rec.Kinds())
	}

	gates := floorChoices(run.rec)
	if len(gates) != 1 {
		t.Fatalf("floor choices = %+v, want one", gates)
	}
	want := []presenter.Choice{presenter.ChoiceCommitSubjects, presenter.ChoiceEdit, presenter.ChoiceAbort}
	if got := gates[0].Keys(); !slices.Equal(got, want) {
		t.Errorf("floor keys = %v, want %v", got, want)
	}
	if moves := nextMoves(run.rec); len(moves) != 0 {
		t.Errorf("NextMoves = %+v, want none at the floor's choice", moves)
	}
}

// twoPartCapBytes is a max_prompt_bytes under which each file of twoPartDiff goes in
// a call of its own, as the default prompt's length changes.
var twoPartCapBytes = len(notes.DefaultPrompt) + 11_200

var twoPartCapToml = fmt.Sprintf("max_prompt_bytes = %d\n", twoPartCapBytes)

// twoPartDiff is a two-file diff too large for one call under twoPartCapBytes.
func twoPartDiff() string {
	var b strings.Builder
	for _, path := range []string{"api/a.go", "core/b.go"} {
		b.WriteString("diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n")
		b.WriteString(strings.Repeat("+"+strings.Repeat("x", 63)+"\n", 10_000/64))
	}
	return b.String()
}

func TestRelease_ChunkedRunFails_SavesTheFinishedPartsNotesInTheGivenStore(t *testing.T) {
	t.Parallel()

	run := newReleaseSeamRun(t)
	run.writeConfig(t, twoPartCapToml)
	run.seedReadGates()
	run.seedNotesDiff(twoPartDiff(), "M\tapi/a.go\nM\tcore/b.go\n", "156\t0\tapi/a.go\n156\t0\tcore/b.go\n")
	// The check finds nothing; whichever part call arrives first returns its notes and
	// the other fails, its retry with it.
	run.f.SeedSequence("claude", gitOut("none"), gitOut("- part notes"), aiFailed(), aiFailed())

	if code := run.release(t, false); code != 1 {
		t.Fatalf("exit code = %d, want 1; events %v", code, run.rec.Kinds())
	}

	scope := strings.ReplaceAll(filepath.Clean(run.root), string(filepath.Separator), "-")
	path := filepath.Join(run.chunkNotesBase, scope, "v1.2.4.md")
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the saved chunk notes: %v", err)
	}
	if !strings.Contains(string(saved), "- part notes") {
		t.Errorf("saved chunk notes = %q, want the finished part's notes", saved)
	}
	failures := stageFailures(run.rec)
	if len(failures) != 1 || failures[0].SavedNotes != path {
		t.Errorf("StageFailures = %+v, want one naming %q", failures, path)
	}
}
