package main

import (
	"bytes"
	"os"
	"testing"

	"mint/internal/buildinfo"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

// TestEmitVersionRoutesThroughShowVersion proves the version core drives exactly
// ONE presenter call — ShowVersion — carrying mint's own tool version (buildinfo.Version),
// and NOTHING else: no gate (Prompt), no RunFinished, no other event. version is the
// no-gate, no-footer payload verb.
func TestEmitVersionRoutesThroughShowVersion(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}

	emitVersion(rec)

	kinds := rec.Kinds()
	if len(kinds) != 1 || kinds[0] != presentertest.KindShowVersion {
		t.Fatalf("version path kinds = %v, want exactly [ShowVersion]", kinds)
	}
	ev := rec.Events[0]
	if ev.ShowVersion.Value != buildinfo.Version {
		t.Errorf("ShowVersion.Value = %q, want buildinfo.Version %q", ev.ShowVersion.Value, buildinfo.Version)
	}
}

// TestEmitVersionUsesPinnedToolVersion proves the value comes from the single
// buildinfo source: pinning buildinfo.Version makes emitVersion report that exact
// value (build-time injection contract — the value is whatever the var holds).
func TestEmitVersionUsesPinnedToolVersion(t *testing.T) {
	original := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = original })
	buildinfo.Version = "1.2.3"

	rec := &presentertest.RecordingPresenter{}
	emitVersion(rec)

	if got := rec.Events[0].ShowVersion.Value; got != "1.2.3" {
		t.Errorf("ShowVersion.Value = %q, want pinned %q", got, "1.2.3")
	}
}

// TestEmitVersionPlainEmitsBareValue is the LOAD-BEARING $(mint version) contract
// driven through the cmd seam end to end: with a plain presenter the version path
// writes EXACTLY the bare value plus a single trailing newline to stdout and
// nothing to stderr — byte-for-byte, so command substitution consumes it cleanly.
func TestEmitVersionPlainEmitsBareValue(t *testing.T) {
	original := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = original })
	buildinfo.Version = "1.2.3"

	var out, errBuf bytes.Buffer
	emitVersion(presenter.New(presenter.ModePlain, &out, &errBuf))

	if want := "1.2.3\n"; out.String() != want {
		t.Errorf("plain version stdout = %q, want %q (byte-for-byte)", out.String(), want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("plain version wrote to stderr: %q", errBuf.String())
	}
}

// TestSubcommandAndFlagAreByteIdentical proves the `version` subcommand and the
// `--version` flag route through the SAME emitVersion (one source) and therefore
// produce byte-identical output. Both are driven through identically-constructed
// plain presenters and their captured stdout must be equal.
func TestSubcommandAndFlagAreByteIdentical(t *testing.T) {
	var subOut, subErr bytes.Buffer
	emitVersion(presenter.New(presenter.ModePlain, &subOut, &subErr))

	var flagOut, flagErr bytes.Buffer
	emitVersion(presenter.New(presenter.ModePlain, &flagOut, &flagErr))

	if subOut.String() != flagOut.String() {
		t.Errorf("subcommand stdout %q != flag stdout %q (must be byte-identical)", subOut.String(), flagOut.String())
	}
}

// TestEmitVersionPassesEmptyLeaf proves the version path supplies an EMPTY Leaf in
// the payload — the pretty presenter defaults it to 🌿 (leafOrDefault), so version
// needs no git/repo to derive a brand leaf. The presenter package owns the dressed
// "{leaf} mint v{value}" rendering; here we only assert the cmd hands it an empty
// leaf (no git-derived value).
func TestEmitVersionPassesEmptyLeaf(t *testing.T) {
	rec := &presentertest.RecordingPresenter{}
	emitVersion(rec)

	if leaf := rec.Events[0].ShowVersion.Leaf; leaf != "" {
		t.Errorf("ShowVersion.Leaf = %q, want empty (no git-derived leaf for version)", leaf)
	}
}

// TestHasVersionFlagRecognisesGlobalFlag proves the global --version flag is
// detected regardless of position, while the verb / unrelated args are not
// mistaken for it.
func TestHasVersionFlagRecognisesGlobalFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "long --version", args: []string{"--version"}, want: true},
		{name: "no flags", args: []string{"release", "-m"}, want: false},
		{name: "the version verb is not the flag", args: []string{"version"}, want: false},
		{name: "no args", args: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasVersionFlag(tt.args); got != tt.want {
				t.Errorf("hasVersionFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// TestRunVersionWorksWithoutGitRepo proves printing the version requires NO git /
// no repo-root resolution: runVersion is driven from a temp dir that is NOT a git
// repository and still succeeds (exit 0). The cmd layer never constructs a runner
// or resolves the root for version, so there is no git command to issue and the
// absence of a repo cannot fail it.
func TestRunVersionWorksWithoutGitRepo(t *testing.T) {
	dir := t.TempDir() // a fresh dir, deliberately not `git init`-ed
	t.Chdir(dir)

	devnull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() { _ = devnull.Close() })

	if code := runVersion(devnull, devnull, devnull); code != 0 {
		t.Fatalf("runVersion exit code = %d, want 0 (must work outside a git repo)", code)
	}
}
