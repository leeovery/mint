package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"mint/internal/engine"
	"mint/internal/initgen"
	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
)

// seedRoot scripts the FakeRunner so `git rev-parse --show-toplevel` resolves to
// root — the single read mint init issues — so init writes the two files at that
// root rather than the invocation directory.
func seedRoot(r *runner.FakeRunner, root string) {
	r.Seed("git", runner.Result{Stdout: root + "\n"}, nil)
}

// configPath / shimPath are the two targets mint init drops at the repo root.
func configPath(root string) string { return filepath.Join(root, ".mint.toml") }
func shimPath(root string) string   { return filepath.Join(root, "release") }

// runInit drives the engine init orchestrator with the standard test seams (a
// FakeRunner resolving to root, a RecordingPresenter), returning both so the
// caller asserts files-on-disk and recorded outcomes.
func runInit(t *testing.T, root string, opts engine.InitOptions) (*presentertest.RecordingPresenter, error) {
	t.Helper()
	r := runner.NewFakeRunner()
	seedRoot(r, root)
	p := &presentertest.RecordingPresenter{}
	err := engine.Init(context.Background(), engine.InitDeps{Presenter: p, Runner: r}, opts)
	return p, err
}

// initEvents returns just the recorded InitResult outcomes, in order, so tests
// assert the per-file dispositions without reaching across other event kinds.
func initEvents(p *presentertest.RecordingPresenter) []presenter.InitOutcome {
	var out []presenter.InitOutcome
	for _, ev := range p.Events {
		if ev.Kind == presentertest.KindInitResult {
			out = append(out, ev.InitResult)
		}
	}
	return out
}

// TestInit_NeitherExists_CreatesBoth proves the empty-project happy path: both
// targets are written at the repo root with the exact generator content, each
// reported InitCreated.
func TestInit_NeitherExists_CreatesBoth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	p, err := runInit(t, root, engine.InitOptions{})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if got := readFile(t, root, ".mint.toml"); got != initgen.MintTOML() {
		t.Errorf(".mint.toml content = %q, want the MintTOML template", got)
	}
	if got := readFile(t, root, "release"); got != initgen.ReleaseShim() {
		t.Errorf("release content = %q, want the ReleaseShim script", got)
	}

	want := []presenter.InitOutcome{
		{Action: presenter.InitCreated, Target: ".mint.toml"},
		{Action: presenter.InitCreated, Target: "release"},
	}
	if got := initEvents(p); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %+v, want %+v", got, want)
	}
}

// TestInit_WrittenShimIsExecutable proves the release shim is written with the
// generator's executable mode so `./release` is runnable straight after init.
func TestInit_WrittenShimIsExecutable(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := runInit(t, root, engine.InitOptions{}); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	info, err := os.Stat(shimPath(root))
	if err != nil {
		t.Fatalf("stat release shim: %v", err)
	}
	if got := info.Mode().Perm(); got != initgen.ShimMode {
		t.Errorf("shim mode = %v, want %v", got, initgen.ShimMode)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("shim mode %v is not executable (no 0o111 bits)", info.Mode().Perm())
	}
}

// TestInit_OnlyConfigExists_OtherCreated proves the two targets are INDEPENDENT:
// the pre-existing .mint.toml is skipped (with the short reason) and left
// byte-for-byte unchanged, while the absent release shim is still created.
func TestInit_OnlyConfigExists_OtherCreated(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const existing = "# hand-written config, must survive\n"
	if err := os.WriteFile(configPath(root), []byte(existing), 0o644); err != nil {
		t.Fatalf("seeding existing config: %v", err)
	}

	p, err := runInit(t, root, engine.InitOptions{})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if got := readFile(t, root, ".mint.toml"); got != existing {
		t.Errorf("existing config was modified: got %q, want %q", got, existing)
	}
	if got := readFile(t, root, "release"); got != initgen.ReleaseShim() {
		t.Errorf("release content = %q, want the ReleaseShim script", got)
	}

	want := []presenter.InitOutcome{
		{Action: presenter.InitSkipped, Target: ".mint.toml", Reason: "exists, use --force"},
		{Action: presenter.InitCreated, Target: "release"},
	}
	if got := initEvents(p); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %+v, want %+v", got, want)
	}
}

// TestInit_OnlyShimExists_OtherCreated is the mirror of the above: the existing
// release shim is skipped and unchanged while the absent config is created.
func TestInit_OnlyShimExists_OtherCreated(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const existing = "#!/bin/sh\necho custom\n"
	if err := os.WriteFile(shimPath(root), []byte(existing), 0o755); err != nil {
		t.Fatalf("seeding existing shim: %v", err)
	}

	p, err := runInit(t, root, engine.InitOptions{})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if got := readFile(t, root, ".mint.toml"); got != initgen.MintTOML() {
		t.Errorf(".mint.toml content = %q, want the MintTOML template", got)
	}
	if got := readFile(t, root, "release"); got != existing {
		t.Errorf("existing shim was modified: got %q, want %q", got, existing)
	}

	want := []presenter.InitOutcome{
		{Action: presenter.InitCreated, Target: ".mint.toml"},
		{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"},
	}
	if got := initEvents(p); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %+v, want %+v", got, want)
	}
}

// TestInit_BothExist_BothSkipped proves the non-clobbering default: both
// pre-existing files are skipped (InitSkipped with the short reason) and neither
// is overwritten.
func TestInit_BothExist_BothSkipped(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	const existingConfig = "# existing config\n"
	const existingShim = "#!/bin/sh\necho existing\n"
	if err := os.WriteFile(configPath(root), []byte(existingConfig), 0o644); err != nil {
		t.Fatalf("seeding existing config: %v", err)
	}
	if err := os.WriteFile(shimPath(root), []byte(existingShim), 0o755); err != nil {
		t.Fatalf("seeding existing shim: %v", err)
	}

	p, err := runInit(t, root, engine.InitOptions{})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if got := readFile(t, root, ".mint.toml"); got != existingConfig {
		t.Errorf("existing config was modified: got %q, want %q", got, existingConfig)
	}
	if got := readFile(t, root, "release"); got != existingShim {
		t.Errorf("existing shim was modified: got %q, want %q", got, existingShim)
	}

	want := []presenter.InitOutcome{
		{Action: presenter.InitSkipped, Target: ".mint.toml", Reason: "exists, use --force"},
		{Action: presenter.InitSkipped, Target: "release", Reason: "exists, use --force"},
	}
	if got := initEvents(p); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %+v, want %+v", got, want)
	}
}

// TestInit_Force_RewritesBoth proves --force overwrites both pre-existing files
// regardless, each reported InitCreated (the engine resolves the disposition; the
// presenter never inspects --force).
func TestInit_Force_RewritesBoth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(configPath(root), []byte("# stale config\n"), 0o644); err != nil {
		t.Fatalf("seeding stale config: %v", err)
	}
	if err := os.WriteFile(shimPath(root), []byte("#!/bin/sh\necho stale\n"), 0o755); err != nil {
		t.Fatalf("seeding stale shim: %v", err)
	}

	p, err := runInit(t, root, engine.InitOptions{Force: true})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	if got := readFile(t, root, ".mint.toml"); got != initgen.MintTOML() {
		t.Errorf(".mint.toml was not regenerated: got %q", got)
	}
	if got := readFile(t, root, "release"); got != initgen.ReleaseShim() {
		t.Errorf("release was not regenerated: got %q", got)
	}

	info, err := os.Stat(shimPath(root))
	if err != nil {
		t.Fatalf("stat regenerated shim: %v", err)
	}
	if got := info.Mode().Perm(); got != initgen.ShimMode {
		t.Errorf("regenerated shim mode = %v, want %v", got, initgen.ShimMode)
	}

	want := []presenter.InitOutcome{
		{Action: presenter.InitCreated, Target: ".mint.toml"},
		{Action: presenter.InitCreated, Target: "release"},
	}
	if got := initEvents(p); !reflect.DeepEqual(got, want) {
		t.Errorf("outcomes = %+v, want %+v", got, want)
	}
}

// TestInit_Force_RestoresExecutableMode proves a --force overwrite of an existing
// NON-executable release shim restores the executable mode — so the shim is
// runnable straight after a regenerate regardless of the prior file's perms. This
// guards the os.WriteFile-keeps-existing-mode-on-overwrite footgun.
func TestInit_Force_RestoresExecutableMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// A pre-existing shim with a NON-executable mode.
	if err := os.WriteFile(shimPath(root), []byte("#!/bin/sh\necho stale\n"), 0o644); err != nil {
		t.Fatalf("seeding non-executable shim: %v", err)
	}

	if _, err := runInit(t, root, engine.InitOptions{Force: true}); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	info, err := os.Stat(shimPath(root))
	if err != nil {
		t.Fatalf("stat regenerated shim: %v", err)
	}
	if got := info.Mode().Perm(); got != initgen.ShimMode {
		t.Errorf("regenerated shim mode = %v, want %v (executable restored on --force)", got, initgen.ShimMode)
	}
}

// TestInit_WritesAtRepoRoot proves the files land at the git-resolved root, NOT
// the invocation directory: the FakeRunner resolves the root to a temp dir, both
// files appear under that root, and init issues exactly the one root-resolution
// read.
func TestInit_WritesAtRepoRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	r := runner.NewFakeRunner()
	seedRoot(r, root)
	p := &presentertest.RecordingPresenter{}
	if err := engine.Init(context.Background(), engine.InitDeps{Presenter: p, Runner: r}, engine.InitOptions{}); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	for _, path := range []string{configPath(root), shimPath(root)} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %q to exist at the repo root: %v", path, err)
		}
	}

	inv := r.Invocations()
	if len(inv) != 1 {
		t.Fatalf("runner invocations = %d, want exactly 1 (the root read); got %v", len(inv), inv)
	}
	if inv[0].Name != "git" || !reflect.DeepEqual(inv[0].Args, []string{"rev-parse", "--show-toplevel"}) {
		t.Errorf("root read = %s %v, want git rev-parse --show-toplevel", inv[0].Name, inv[0].Args)
	}
}

// TestInit_ScaffoldsOnlyTwoFiles proves nothing beyond the two targets is
// scaffolded: no hook scripts, no prompt-override file, no .release/hooks dir —
// the only entries created at the root are .mint.toml and release.
func TestInit_ScaffoldsOnlyTwoFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := runInit(t, root, engine.InitOptions{}); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("reading repo root: %v", err)
	}
	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.Name()
	}
	sort.Strings(got)
	want := []string{".mint.toml", "release"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("root entries = %v, want exactly %v (no hook/prompt files)", got, want)
	}
}

// TestInit_NoRunFinishedNoPrompt proves init's terminal output is the InitResult
// lines alone: it never calls RunFinished (no release-style footer) and never
// calls Prompt (no interactive gate).
func TestInit_NoRunFinishedNoPrompt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	p, err := runInit(t, root, engine.InitOptions{})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	for _, k := range p.Kinds() {
		if k == presentertest.KindRunFinished {
			t.Errorf("init called RunFinished; it must not (its InitResult lines are terminal)")
		}
		if k == presentertest.KindPrompt {
			t.Errorf("init called Prompt; it has no interactive gate")
		}
	}
}

// TestInit_RootResolutionFails_Aborts proves a non-git invocation aborts cleanly
// (the runner's root read fails) without emitting any InitResult.
func TestInit_RootResolutionFails_Aborts(t *testing.T) {
	t.Parallel()

	r := runner.NewFakeRunner()
	r.Seed("git", runner.Result{ExitCode: 128}, errors.New("not a git repository"))
	p := &presentertest.RecordingPresenter{}

	err := engine.Init(context.Background(), engine.InitDeps{Presenter: p, Runner: r}, engine.InitOptions{})
	if err == nil {
		t.Fatal("Init returned nil error when the root could not be resolved")
	}
	if got := initEvents(p); len(got) != 0 {
		t.Errorf("outcomes = %+v, want none when the root cannot be resolved", got)
	}
}
