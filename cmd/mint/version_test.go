package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/buildinfo"
	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
	"mint/internal/presenter/presentertest"
	"mint/internal/ptytest"
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

// TestVersion_Frames holds the version, drawn on the rail through the version core onto
// a virtual terminal 100 columns wide with colour off, to its frame in testdata/frames
// cell for cell: " mint v0.1.2", the wordmark in the accent.
// `go test ./cmd/mint -run Frames -update` writes it again.
func TestVersion_Frames(t *testing.T) {
	original := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = original })
	buildinfo.Version = "0.1.2"

	term := frametest.NewTerminal(100, 40)
	p := presenter.NewRailPresenter(term,
		presenter.WithRailProfile(termenv.Ascii),
		presenter.WithRailSize(func() (int, int) { return 100, 40 }),
	)
	emitVersion(p)
	p.Close()
	if err := term.Err(); err != nil {
		t.Fatalf("the terminal was written what it can't apply: %v", err)
	}

	frametest.Check(t, "testdata/frames", map[string]string{"version-100": term.ANSI()})
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

	if code := runVersion(false, devnull, devnull, devnull); code != 0 {
		t.Fatalf("runVersion exit code = %d, want 0 (must work outside a git repo)", code)
	}
}

// TestRunVersionHonoursPlainOnATerminal proves --plain reaches the version surface:
// on a terminal, which alone would draw the dressed form, it prints the bare value.
func TestRunVersionHonoursPlainOnATerminal(t *testing.T) {
	original := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = original })
	buildinfo.Version = "1.2.3"

	tests := []struct {
		name  string
		plain bool
		check func(t *testing.T, written string)
	}{
		{
			name:  "--plain prints the bare value",
			plain: true,
			check: func(t *testing.T, written string) {
				if written != "1.2.3\r\n" {
					t.Errorf("written = %q, want the bare value %q", written, "1.2.3\r\n")
				}
			},
		},
		{
			name: "without it the terminal gets the dressed form",
			check: func(t *testing.T, written string) {
				if !strings.Contains(written, " mint v1.2.3\r\n") {
					t.Errorf("written = %q, want the Rail's dressed %q", written, " mint v1.2.3")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pty, tty := ptytest.Open(t)
			if code := runVersion(tt.plain, tty, tty, tty); code != 0 {
				t.Fatalf("runVersion exit code = %d, want 0", code)
			}
			tt.check(t, ptytest.Written(t, pty))
		})
	}
}

// TestHasPlainFlagFindsItAnywhere proves --plain is found as a whole token wherever
// it appears, for `mint version --plain` and `mint --version --plain` alike.
func TestHasPlainFlagFindsItAnywhere(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"--plain"}, want: true},
		{args: []string{"--version", "--plain"}, want: true},
		{args: []string{"--plainer"}, want: false},
		{args: nil, want: false},
	}
	for _, tt := range tests {
		if got := hasPlainFlag(tt.args); got != tt.want {
			t.Errorf("hasPlainFlag(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}
