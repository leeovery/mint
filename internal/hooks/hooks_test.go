package hooks_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mint/internal/hooks"
	"mint/internal/runner"
)

// sampleEnv is a fully-populated HookEnv reused by the execution tests; the bump
// kind is varied where a test cares about MINT_BUMP specifically.
func sampleEnv() hooks.HookEnv {
	return hooks.NewHookEnv("1.4.0", "1.3.2", "v1.4.0", hooks.BumpMinor, false)
}

func TestRunner_StringHook_RunsAsSingleShellCommand(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	r := hooks.NewRunner(fake)
	if err := r.Run(t.Context(), "npm ci && npm run build", "/repo", sampleEnv()); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	calls := fake.Invocations()
	if len(calls) != 1 {
		t.Fatalf("len(Invocations) = %d, want 1 (single string → one sh -c)", len(calls))
	}
	if calls[0].Name != "sh" {
		t.Errorf("Name = %q, want %q", calls[0].Name, "sh")
	}
	wantArgs := []string{"-c", "npm ci && npm run build"}
	if !slices.Equal(calls[0].Args, wantArgs) {
		t.Errorf("Args = %v, want %v", calls[0].Args, wantArgs)
	}
}

func TestRunner_ArrayHook_RunsEachEntryInDeclaredOrder(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	r := hooks.NewRunner(fake)
	value := []string{"step one", "step two", "step three"}
	if err := r.Run(t.Context(), value, "/repo", sampleEnv()); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	calls := fake.Invocations()
	if len(calls) != len(value) {
		t.Fatalf("len(Invocations) = %d, want %d", len(calls), len(value))
	}
	for i, entry := range value {
		wantArgs := []string{"-c", entry}
		if calls[i].Name != "sh" || !slices.Equal(calls[i].Args, wantArgs) {
			t.Errorf("call %d = %s %v, want sh %v", i, calls[i].Name, calls[i].Args, wantArgs)
		}
	}
}

func TestRunner_AnySliceHook_NormalisesToOrderedEntries(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	// Some TOML decoders surface a string array as []any; the runner must coerce
	// each element to a string and preserve declared order.
	r := hooks.NewRunner(fake)
	value := []any{"first", "second"}
	if err := r.Run(t.Context(), value, "/repo", sampleEnv()); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	calls := fake.Invocations()
	if len(calls) != 2 {
		t.Fatalf("len(Invocations) = %d, want 2", len(calls))
	}
	wantEntries := []string{"first", "second"}
	for i, entry := range wantEntries {
		wantArgs := []string{"-c", entry}
		if !slices.Equal(calls[i].Args, wantArgs) {
			t.Errorf("call %d Args = %v, want %v", i, calls[i].Args, wantArgs)
		}
	}
}

func TestRunner_RunsFromRepoRoot(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	const repoRoot = "/path/to/repo/root"
	r := hooks.NewRunner(fake)
	if err := r.Run(t.Context(), []string{"a", "b"}, repoRoot, sampleEnv()); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	for i, c := range fake.Invocations() {
		if c.Dir != repoRoot {
			t.Errorf("call %d Dir = %q, want %q", i, c.Dir, repoRoot)
		}
	}
}

func TestRunner_InjectsMintEnv(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	r := hooks.NewRunner(fake)
	env := hooks.NewHookEnv("1.4.0", "1.3.2", "v1.4.0", hooks.BumpMinor, false)
	if err := r.Run(t.Context(), "do thing", "/repo", env); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	calls := fake.Invocations()
	if len(calls) != 1 {
		t.Fatalf("len(Invocations) = %d, want 1", len(calls))
	}

	want := []string{
		"MINT_NEW_VERSION=1.4.0",
		"MINT_PREVIOUS_VERSION=1.3.2",
		"MINT_VERSION_TAG=v1.4.0",
		"MINT_BUMP=minor",
		"MINT_DRY_RUN=0",
	}
	for _, kv := range want {
		if !slices.Contains(calls[0].Env, kv) {
			t.Errorf("injected env missing %q; got %v", kv, calls[0].Env)
		}
	}
}

func TestRunner_DryRunRendersAsOne(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	r := hooks.NewRunner(fake)
	env := hooks.NewHookEnv("1.4.0", "1.3.2", "v1.4.0", hooks.BumpPatch, true)
	if err := r.Run(t.Context(), "do thing", "/repo", env); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := fake.Invocations()[0].Env; !slices.Contains(got, "MINT_DRY_RUN=1") {
		t.Errorf("env = %v, want it to contain MINT_DRY_RUN=1", got)
	}
}

func TestRunner_BumpExplicit_ForSetVersion(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{}, nil)

	// When --set-version was used the bump kind is "explicit"; the builder must map
	// BumpExplicit to MINT_BUMP=explicit.
	r := hooks.NewRunner(fake)
	env := hooks.NewHookEnv("2.0.0", "1.9.9", "v2.0.0", hooks.BumpExplicit, false)
	if err := r.Run(t.Context(), "do thing", "/repo", env); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	if got := fake.Invocations()[0].Env; !slices.Contains(got, "MINT_BUMP=explicit") {
		t.Errorf("env = %v, want it to contain MINT_BUMP=explicit", got)
	}
}

func TestRunner_FirstNonZeroExit_StopsAndReturnsFailure(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	exitErr := errors.New("exit status 1")
	// Entry 1 fails; entry 2 must never run.
	fake.SeedSequence("sh",
		runner.ScriptedCall{Result: runner.Result{Stderr: "build failed", ExitCode: 1}, Err: exitErr},
		runner.ScriptedCall{Result: runner.Result{}},
	)

	r := hooks.NewRunner(fake)
	err := r.Run(t.Context(), []string{"failing step", "later step"}, "/repo", sampleEnv())

	if err == nil {
		t.Fatal("Run returned nil error after a failing entry, want non-nil")
	}

	// Sequence stops at the first failure: only one invocation was made.
	if calls := fake.Invocations(); len(calls) != 1 {
		t.Fatalf("len(Invocations) = %d, want 1 (later entries must not run)", len(calls))
	}

	// The failing entry's outcome must stay inspectable: the wrapped runner error
	// and the entry + stderr/exit it carries.
	if !errors.Is(err, exitErr) {
		t.Errorf("error = %v, want it to wrap the failing runner error", err)
	}
	var hookErr *hooks.HookError
	if !errors.As(err, &hookErr) {
		t.Fatalf("error = %v, want a *hooks.HookError in the chain", err)
	}
	if hookErr.Entry != "failing step" {
		t.Errorf("HookError.Entry = %q, want %q", hookErr.Entry, "failing step")
	}
	if hookErr.Result.ExitCode != 1 {
		t.Errorf("HookError.Result.ExitCode = %d, want 1", hookErr.Result.ExitCode)
	}
	if hookErr.Result.Stderr != "build failed" {
		t.Errorf("HookError.Result.Stderr = %q, want %q", hookErr.Result.Stderr, "build failed")
	}
}

func TestRunner_EmptyOrAbsentValue_IsNoOp(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		value any
	}{
		{name: "nil", value: nil},
		{name: "empty string", value: ""},
		{name: "empty []string", value: []string{}},
		{name: "empty []any", value: []any{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fake := runner.NewFakeRunner()
			// No Seed: an unseeded sh would error, so a stray invocation is caught.
			r := hooks.NewRunner(fake)

			if err := r.Run(t.Context(), tc.value, "/repo", sampleEnv()); err != nil {
				t.Fatalf("Run returned unexpected error for %s: %v", tc.name, err)
			}
			if calls := fake.Invocations(); len(calls) != 0 {
				t.Fatalf("len(Invocations) = %d, want 0 (no-op)", len(calls))
			}
		})
	}
}

func TestHookEnv_RenderProducesAllVars(t *testing.T) {
	t.Parallel()

	// The render point is the single source of the MINT_* set; assert the full,
	// exact set so growth is a deliberate, visible change.
	env := hooks.NewHookEnv("1.4.0", "1.3.2", "v1.4.0", hooks.BumpMajor, false)
	got := env.Render()

	want := []string{
		"MINT_NEW_VERSION=1.4.0",
		"MINT_PREVIOUS_VERSION=1.3.2",
		"MINT_VERSION_TAG=v1.4.0",
		"MINT_BUMP=major",
		"MINT_DRY_RUN=0",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Render() = %v, want %v", got, want)
	}
}

// guard: every rendered entry is a KEY=VALUE pair.
func TestHookEnv_RenderEntriesAreKeyValue(t *testing.T) {
	t.Parallel()

	for _, kv := range sampleEnv().Render() {
		if !strings.Contains(kv, "=") {
			t.Errorf("rendered entry %q is not KEY=VALUE", kv)
		}
	}
}
