package runner_test

import (
	"errors"
	"strings"
	"testing"

	"mint/internal/runner"
)

func TestFakeRunner_ReturnsSeededResult(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("git", runner.Result{Stdout: "v1.2.3", ExitCode: 0}, nil)

	res, err := fake.Run(t.Context(), "git", "describe", "--tags")

	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	if res.Stdout != "v1.2.3" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "v1.2.3")
	}
	if res.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", res.ExitCode)
	}
}

func TestFakeRunner_SeededNonZeroExit(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	wantErr := errors.New("exit 2")
	fake.Seed("gh", runner.Result{Stderr: "not authenticated", ExitCode: 2}, wantErr)

	res, err := fake.Run(t.Context(), "gh", "auth", "status")

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want it to wrap the seeded error", err)
	}
	// Non-zero exit keeps the Result populated so callers read Stderr/ExitCode.
	if res.ExitCode != 2 {
		t.Errorf("ExitCode = %d, want 2", res.ExitCode)
	}
	if res.Stderr != "not authenticated" {
		t.Errorf("Stderr = %q, want %q", res.Stderr, "not authenticated")
	}
}

func TestFakeRunner_RecordsInvocationOrder(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("git", runner.Result{}, nil)
	fake.Seed("gh", runner.Result{}, nil)

	if _, err := fake.Run(t.Context(), "git", "status"); err != nil {
		t.Fatalf("Run(git) error: %v", err)
	}
	if _, err := fake.Run(t.Context(), "gh", "pr", "list"); err != nil {
		t.Fatalf("Run(gh) error: %v", err)
	}
	if _, err := fake.Run(t.Context(), "git", "push"); err != nil {
		t.Fatalf("Run(git push) error: %v", err)
	}

	calls := fake.Invocations()
	if len(calls) != 3 {
		t.Fatalf("len(Invocations) = %d, want 3", len(calls))
	}

	wantNames := []string{"git", "gh", "git"}
	wantArgs := [][]string{{"status"}, {"pr", "list"}, {"push"}}
	for i, c := range calls {
		if c.Name != wantNames[i] {
			t.Errorf("call %d Name = %q, want %q", i, c.Name, wantNames[i])
		}
		if strings.Join(c.Args, " ") != strings.Join(wantArgs[i], " ") {
			t.Errorf("call %d Args = %v, want %v", i, c.Args, wantArgs[i])
		}
	}
}

func TestFakeRunner_SimulatesCommandNotFound(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.SeedNotFound("gh")

	_, err := fake.Run(t.Context(), "gh", "auth", "status")

	if err == nil {
		t.Fatal("Run returned nil error for a not-found command, want non-nil")
	}
	// The fake must surface the same sentinel the real runner uses so tests of the
	// gh gate behave identically against either implementation.
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
}

func TestFakeRunner_RunWith_RecordsStdin(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("claude", runner.Result{Stdout: "notes body"}, nil)

	res, err := fake.RunWith(t.Context(), strings.NewReader("the prompt"), "claude", "-p")

	if err != nil {
		t.Fatalf("RunWith returned unexpected error: %v", err)
	}
	if res.Stdout != "notes body" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "notes body")
	}

	calls := fake.Invocations()
	if len(calls) != 1 {
		t.Fatalf("len(Invocations) = %d, want 1", len(calls))
	}
	if calls[0].Stdin != "the prompt" {
		t.Errorf("recorded Stdin = %q, want %q", calls[0].Stdin, "the prompt")
	}
}

func TestFakeRunner_SeedSequence_ReturnsOutcomesInCallOrder(t *testing.T) {
	t.Parallel()

	// Some callers issue several invocations of the SAME binary that must return
	// DIFFERENT outcomes — e.g. `git tag` succeeds, then `git push` is rejected.
	// SeedSequence scripts those per-call outcomes in order, since name-keyed Seed
	// alone cannot tell two calls to the same binary apart.
	fake := runner.NewFakeRunner()
	pushErr := errors.New("exit status 1")
	fake.SeedSequence("git",
		runner.ScriptedCall{Result: runner.Result{}},
		runner.ScriptedCall{Result: runner.Result{Stderr: "rejected", ExitCode: 1}, Err: pushErr},
	)

	if _, err := fake.Run(t.Context(), "git", "tag", "-a", "v1", "-F", "-"); err != nil {
		t.Fatalf("first call (tag) returned unexpected error: %v", err)
	}

	res, err := fake.Run(t.Context(), "git", "push", "--atomic", "origin", "HEAD", "v1")
	if !errors.Is(err, pushErr) {
		t.Fatalf("second call (push) error = %v, want it to wrap the seeded push error", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
}

func TestFakeRunner_UnseededCommandFails(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()

	_, err := fake.Run(t.Context(), "git", "status")

	// An unseeded command is a test-authoring mistake; the fake surfaces it rather
	// than silently returning a zero Result that could mask the gap.
	if err == nil {
		t.Fatal("Run returned nil error for an unseeded command, want non-nil")
	}
}

func TestFakeRunner_RunInteractive_RecordsAndReturnsSeeded(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.Seed("vi", runner.Result{}, nil)

	if err := fake.RunInteractive(t.Context(), "vi", "/tmp/mint-notes-x.md"); err != nil {
		t.Fatalf("RunInteractive returned unexpected error: %v", err)
	}

	calls := fake.Invocations()
	if len(calls) != 1 {
		t.Fatalf("len(Invocations) = %d, want 1", len(calls))
	}
	if calls[0].Name != "vi" {
		t.Errorf("recorded Name = %q, want %q", calls[0].Name, "vi")
	}
	if strings.Join(calls[0].Args, " ") != "/tmp/mint-notes-x.md" {
		t.Errorf("recorded Args = %v, want [/tmp/mint-notes-x.md]", calls[0].Args)
	}
}

func TestFakeRunner_RunInteractive_HonoursSeedNotFound(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	fake.SeedNotFound("ed")

	err := fake.RunInteractive(t.Context(), "ed", "/tmp/mint-notes-y.md")

	if err == nil {
		t.Fatal("RunInteractive returned nil error for a not-found editor, want non-nil")
	}
	if !errors.Is(err, runner.ErrCommandNotFound) {
		t.Errorf("error = %v, want it to match ErrCommandNotFound", err)
	}
}

func TestFakeRunner_RunInteractive_HonoursSeededError(t *testing.T) {
	t.Parallel()

	fake := runner.NewFakeRunner()
	wantErr := errors.New("editor exited non-zero")
	fake.Seed("vi", runner.Result{ExitCode: 1}, wantErr)

	err := fake.RunInteractive(t.Context(), "vi", "/tmp/mint-notes-z.md")

	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want it to wrap the seeded error", err)
	}
}

func TestFakeRunner_RunInDir_RecordsDirAndEnv(t *testing.T) {
	t.Parallel()

	// Hooks run via RunInDir from the repo root with MINT_* env layered on; the
	// fake must record both the working directory and the injected env so tests can
	// assert mint set them (the other methods leave both zero).
	fake := runner.NewFakeRunner()
	fake.Seed("sh", runner.Result{Stdout: "ok"}, nil)

	env := []string{"MINT_NEW_VERSION=1.4.0", "MINT_DRY_RUN=0"}
	res, err := fake.RunInDir(t.Context(), "/repo/root", env, "sh", "-c", "echo hi")

	if err != nil {
		t.Fatalf("RunInDir returned unexpected error: %v", err)
	}
	if res.Stdout != "ok" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "ok")
	}

	calls := fake.Invocations()
	if len(calls) != 1 {
		t.Fatalf("len(Invocations) = %d, want 1", len(calls))
	}
	if calls[0].Name != "sh" {
		t.Errorf("recorded Name = %q, want %q", calls[0].Name, "sh")
	}
	if strings.Join(calls[0].Args, " ") != "-c echo hi" {
		t.Errorf("recorded Args = %v, want [-c echo hi]", calls[0].Args)
	}
	if calls[0].Dir != "/repo/root" {
		t.Errorf("recorded Dir = %q, want %q", calls[0].Dir, "/repo/root")
	}
	if strings.Join(calls[0].Env, " ") != strings.Join(env, " ") {
		t.Errorf("recorded Env = %v, want %v", calls[0].Env, env)
	}
}

func TestFakeRunner_RunInDir_HonoursSeedSequence(t *testing.T) {
	t.Parallel()

	// A hook scripts each entry's exit via the existing per-name sequence scripting;
	// RunInDir must consume it the same way Run does so the first failure can be
	// modelled.
	fake := runner.NewFakeRunner()
	bang := errors.New("exit status 1")
	fake.SeedSequence("sh",
		runner.ScriptedCall{Result: runner.Result{}},
		runner.ScriptedCall{Result: runner.Result{Stderr: "boom", ExitCode: 1}, Err: bang},
	)

	if _, err := fake.RunInDir(t.Context(), "/repo", nil, "sh", "-c", "step1"); err != nil {
		t.Fatalf("first entry returned unexpected error: %v", err)
	}

	res, err := fake.RunInDir(t.Context(), "/repo", nil, "sh", "-c", "step2")
	if !errors.Is(err, bang) {
		t.Fatalf("second entry error = %v, want it to wrap the seeded error", err)
	}
	if res.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", res.ExitCode)
	}
}

// FakeRunner must be substitutable for the seam wherever a CommandRunner is
// consumed; this compile-time check guards that.
var _ runner.CommandRunner = (*runner.FakeRunner)(nil)
