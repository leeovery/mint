package engine_test

import (
	"strings"
	"testing"

	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

func TestRelease_MintConfigIsLeftOutOfTheNotesDiffAndChangeMap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedExportNotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertNotesGitCallsExclude(t, f, priorTag+"..HEAD", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml")
	prompt := notesStdinOf(t, f, "claude", leanClaudeArgs...)
	if !strings.Contains(prompt, exportDiff) {
		t.Errorf("notes call does not carry the src/export.go diff; got:\n%s", prompt)
	}
	if strings.Contains(prompt, ".mint.toml") {
		t.Errorf("notes call names .mint.toml; got:\n%s", prompt)
	}
}

func TestRelease_MintConfigOnlyRangeGetsTheMaintenanceBody(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	f.SeedSequence("git", ScriptedOut("")) // diff priorTag..HEAD: the only change is .mint.toml
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	if !invokedWith(f, "git", "diff", "--src-prefix=a/", "--dst-prefix=b/", "--no-color", "--no-ext-diff", priorTag+"..HEAD", "--", ".", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml") {
		t.Errorf("notes diff does not exclude .mint.toml; got %v", commandLines(f.Invocations()))
	}
	for _, inv := range f.Invocations() {
		if inv.Name == "claude" {
			t.Errorf("claude was invoked for a range whose only change is .mint.toml: %q", commandLine(inv))
		}
	}
	const want = "Maintenance release — no notable source changes"
	if got := tagAnnotationBody(t, f, nextTag); got != terminatedAnnotationBody(want) {
		t.Errorf("tag annotation body = %q, want the maintenance-release body %q", got, terminatedAnnotationBody(want))
	}
}

func TestRegenerateFreshBody_MintConfigIsLeftOutOfTheNotesDiffAndChangeMap(t *testing.T) {
	t.Parallel()

	f := seedFreshGit(exportDiff, "A\tsrc/export.go\n", "12\t0\tsrc/export.go\n")
	tr := &freshTransport{body: "notes"}

	res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, tr, t.TempDir(), freshCfg(), res, engine.NotesSinks{}); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	assertNotesGitCallsExclude(t, f, "v1.3.0..v1.4.0", ":(exclude)CHANGELOG.md", ":(exclude).mint.toml")
	if tr.calls() != 1 {
		t.Fatalf("transport calls = %d, want 1", tr.calls())
	}
	if strings.Contains(tr.prompts[0], ".mint.toml") {
		t.Errorf("regenerate prompt names .mint.toml; got:\n%s", tr.prompts[0])
	}
}

func TestRegenerateFreshBody_VersionFileExclusionBesideTheBuiltIns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		versionPattern string
		pathspecs      []string
	}{
		{
			name:      "plain mode leaves the version file out",
			pathspecs: []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml", ":(exclude)VERSION"},
		},
		{
			name:           "embedded mode keeps the version file in",
			versionPattern: `{version}`,
			pathspecs:      []string{":(exclude)CHANGELOG.md", ":(exclude).mint.toml"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := seedFreshGit(exportDiff, "A\tsrc/export.go\n", "12\t0\tsrc/export.go\n")
			cfg := freshCfg()
			cfg.Release.VersionFile = "VERSION"
			cfg.Release.VersionPattern = tt.versionPattern
			res := version.Resolution{Tag: "v1.4.0", PreviousTag: "v1.3.0"}
			if _, err := engine.RegenerateFreshBody(t.Context(), f, &freshTransport{body: "notes"}, t.TempDir(), cfg, res, engine.NotesSinks{}); err != nil {
				t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
			}

			assertNotesGitCallsExclude(t, f, "v1.3.0..v1.4.0", tt.pathspecs...)
		})
	}
}
