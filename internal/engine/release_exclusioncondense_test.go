package engine_test

import (
	"fmt"
	"slices"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

func TestRelease_ExclusionCheck_CondensesItsListPastATenthOfTheConfiguredCap(t *testing.T) {
	t.Parallel()

	var diff string
	for i := range 120 {
		diff += fileDiffOfSize(t, fmt.Sprintf("vendor/pkg/file%03d.go", i), 80)
	}
	// The 120-line list takes more than a tenth of a 30,000-byte cap, which the notes
	// call still fits under.
	f, _ := checkedRelease(t, "max_prompt_bytes = 30000\n", diff, "", "", "none")

	checks := checkInvocations(f, "claude")
	if len(checks) != 1 {
		t.Fatalf("check calls = %d, want one", len(checks))
	}
	want := []string{"- vendor/: 120 files, 9.6 KB"}
	if got := checkList(checks[0].Stdin); !slices.Equal(got, want) {
		t.Errorf("check list = %q, want %q", got, want)
	}
}

func TestRelease_ACheckCallTooBigForTheCap_IsNotSentAndWarnsWithNoOutputAsTheReleaseCarriesOn(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A cap below the check's own instructions; the notes then fall back too.
	writeConfig(t, root, "max_prompt_bytes = 900\n\n[release]\non_notes_failure = \"fallback\"\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git",
		ScriptedOut(twoFileDiff(10_000)),
		ScriptedOut("M\tapi/a.go\nM\tcore/b.go\n"),
		ScriptedOut("156\t0\tapi/a.go\n156\t0\tcore/b.go\n"),
		ScriptedOut("subject one\nsubject two\n"), // log --format=%s
	)
	seedClaudeAfterCheck(f)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if got := len(checkInvocations(f, "claude")); got != 0 {
		t.Errorf("check calls = %d, want none", got)
	}
	warnings := checkFailedWarnings(rec)
	if len(warnings) != 1 {
		t.Fatalf("check-failed warnings = %d, want one; kinds = %v", len(warnings), rec.Kinds())
	}
	if warnings[0].Output != "" {
		t.Errorf("warning output = %q, want none", warnings[0].Output)
	}
	if got := tagAnnotationBody(t, f, nextTag); got != "subject one\nsubject two\n" {
		t.Errorf("tag annotation body = %q, want the commit-subject fallback", got)
	}
}
