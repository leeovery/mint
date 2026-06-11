package initgen_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/initgen"
)

func TestReleaseShim_HasShebang(t *testing.T) {
	t.Parallel()

	shim := initgen.ReleaseShim()

	if !strings.HasPrefix(shim, "#!/usr/bin/env sh\n") {
		t.Errorf("shim must start with the `#!/usr/bin/env sh` shebang, got:\n%s", shim)
	}
}

func TestReleaseShim_ExecsMintReleaseForwardingArgs(t *testing.T) {
	t.Parallel()

	shim := initgen.ReleaseShim()

	// exec REPLACES the shim process with `mint release` for clean signal /
	// exit-code propagation; "$@" forwards every argument verbatim.
	if !strings.Contains(shim, `exec mint release "$@"`) {
		t.Errorf(`shim must contain the exact line `+"`"+`exec mint release "$@"`+"`"+`, got:\n%s`, shim)
	}
}

func TestReleaseShim_GuardsOnCommandV(t *testing.T) {
	t.Parallel()

	shim := initgen.ReleaseShim()

	// `command -v mint` is the portable presence check (POSIX, no `which`).
	if !strings.Contains(shim, "command -v mint") {
		t.Errorf("shim must guard mint presence with `command -v mint`, got:\n%s", shim)
	}
}

func TestReleaseShim_PrintsInstallHintAndExitsNonZeroWhenAbsent(t *testing.T) {
	t.Parallel()

	shim := initgen.ReleaseShim()

	if !strings.Contains(shim, "brew install leeovery/tools/mint") {
		t.Errorf("shim must print the exact install hint `brew install leeovery/tools/mint`, got:\n%s", shim)
	}
	if !strings.Contains(shim, "exit 1") {
		t.Errorf("shim must `exit 1` (non-zero) when mint is absent, got:\n%s", shim)
	}
}

func TestShimMode_IsExecutable(t *testing.T) {
	t.Parallel()

	if initgen.ShimMode != 0o755 {
		t.Errorf("ShimMode = %#o, want %#o", initgen.ShimMode, os.FileMode(0o755))
	}
}

// TestReleaseShim_RuntimeAbsentMintFailsWithHint runs the generated shim under a
// PATH that lacks `mint` and asserts it exits non-zero and prints the install
// hint to stderr. This is the runtime confirmation that the string guard works
// when executed by a real POSIX `sh`.
func TestReleaseShim_RuntimeAbsentMintFailsWithHint(t *testing.T) {
	t.Parallel()

	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("no POSIX sh on PATH: %v", err)
	}

	dir := t.TempDir()
	shimPath := filepath.Join(dir, "release")
	if err := os.WriteFile(shimPath, []byte(initgen.ReleaseShim()), initgen.ShimMode); err != nil {
		t.Fatalf("writing shim: %v", err)
	}

	// emptyBin is a PATH dir containing only `sh` (symlinked) so `command -v mint`
	// finds nothing while the interpreter can still resolve its own builtins.
	emptyBin := filepath.Join(dir, "bin")
	if err := os.Mkdir(emptyBin, 0o755); err != nil {
		t.Fatalf("creating empty bin dir: %v", err)
	}

	cmd := exec.Command(sh, shimPath)
	cmd.Env = append(os.Environ(), "PATH="+emptyBin)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		t.Fatalf("expected the shim to exit non-zero, got err=%v", runErr)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("expected non-zero exit code, got 0")
	}
	if !strings.Contains(stderr.String(), "brew install leeovery/tools/mint") {
		t.Errorf("expected install hint on stderr, got:\n%s", stderr.String())
	}
}

// TestReleaseShim_RuntimePresentMintExecsRelease places a stub `mint` on PATH and
// asserts the shim execs it with `release` plus all forwarded args.
func TestReleaseShim_RuntimePresentMintExecsRelease(t *testing.T) {
	t.Parallel()

	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skipf("no POSIX sh on PATH: %v", err)
	}

	dir := t.TempDir()
	shimPath := filepath.Join(dir, "release")
	if err := os.WriteFile(shimPath, []byte(initgen.ReleaseShim()), initgen.ShimMode); err != nil {
		t.Fatalf("writing shim: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("creating bin dir: %v", err)
	}
	// Stub `mint` echoes the args it received so we can prove `release "$@"` forwarding.
	stubMint := "#!/usr/bin/env sh\nprintf '%s\\n' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "mint"), []byte(stubMint), 0o755); err != nil {
		t.Fatalf("writing stub mint: %v", err)
	}

	// The stub `mint` is itself a `#!/usr/bin/env sh` script, so its interpreter
	// (`sh`/`env`) must remain resolvable; prepend binDir to the inherited PATH so
	// `command -v mint` and `exec mint` find the stub first while sh/env still work.
	cmd := exec.Command(sh, shimPath, "-m", "--set-version", "2.0.0")
	cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var stdout strings.Builder
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("running shim with stub mint: %v", err)
	}

	got := strings.Fields(strings.TrimSpace(stdout.String()))
	want := []string{"release", "-m", "--set-version", "2.0.0"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("stub mint received args %v, want %v", got, want)
	}
}
