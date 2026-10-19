package engine_test

import (
	"errors"
	"slices"
	"testing"

	"mint/internal/config"
	"mint/internal/engine"
	"mint/internal/presenter/presentertest"
	"mint/internal/runner"
	"mint/internal/version"
)

// leanClaudeArgs is the argv the shipped default ai_command must reach claude with,
// written out literally so a drift in the constant or in its splitting fails here.
var leanClaudeArgs = []string{
	"-p", "--model", "sonnet",
	"--safe-mode", "--tools", "",
	"--strict-mcp-config", "--no-session-persistence",
	"--system-prompt", "Follow the instructions you are given and output only what they ask for.",
}

// invocationsNamed returns every recorded invocation of the named program, in order.
func invocationsNamed(f *runner.FakeRunner, name string) []runner.Invocation {
	var out []runner.Invocation
	for _, inv := range f.Invocations() {
		if inv.Name == name {
			out = append(out, inv)
		}
	}
	return out
}

// assertOnlyArgv fails unless name was invoked at least once and every invocation of it
// carried exactly want.
func assertOnlyArgv(t *testing.T, f *runner.FakeRunner, name string, want []string) []runner.Invocation {
	t.Helper()
	invs := invocationsNamed(f, name)
	if len(invs) == 0 {
		t.Fatalf("%s was never invoked; got %v", name, commandLines(f.Invocations()))
	}
	for i, inv := range invs {
		if !slices.Equal(inv.Args, want) {
			t.Errorf("%s invocation %d args = %q, want exactly %q", name, i, inv.Args, want)
		}
	}
	return invs
}

func TestRelease_ZeroConfig_RunsLeanClaudeCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir() // no .mint.toml
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	invs := assertOnlyArgv(t, f, "claude", leanClaudeArgs)
	if invs[0].Stdin == "" {
		t.Error("lean claude command was invoked without the composed prompt on stdin")
	}
}

func TestRelease_ExplicitSharedAICommand_RunsVerbatimWithoutLeanFlags(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "ai_command = 'llm -m gpt-4o'\n")
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("llm", runner.Result{Stdout: aiBody}, nil)
	seedRecordTagPush(f, root)
	f.Seed("gh", runner.Result{}, nil)
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err != nil {
		t.Fatalf("Release returned unexpected error: %v", err)
	}

	assertOnlyArgv(t, f, "llm", []string{"-m", "gpt-4o"})
	if got := invocationsNamed(f, "claude"); len(got) != 0 {
		t.Errorf("claude was invoked %d time(s) despite an explicit shared ai_command", len(got))
	}
}

func TestRelease_ClaudeRejectsLeanFlag_RunsOnlyTheLeanCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	f := runner.NewFakeRunner()
	seedPriorTagReadGates(f, root, "main")
	seedNormalAINotes(f)
	f.Seed("claude",
		runner.Result{Stderr: "error: unknown option '--safe-mode'\n", ExitCode: 1},
		errors.New("exit status 1"))
	rec := &presentertest.RecordingPresenter{}

	if err := engine.Release(t.Context(), newDeps(rec, f), priorTagNormalAIOptions()); err == nil {
		t.Fatal("Release returned nil, want the notes generation failure")
	}

	assertOnlyArgv(t, f, "claude", leanClaudeArgs)
}

func TestRegenerateFreshBody_ConfigWithoutAICommand_RunsLeanClaudeCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeConfig(t, root, "[release]\ncontext = 'house style'\n")
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load returned unexpected error: %v", err)
	}

	diff := "diff --git a/api.go b/api.go\n@@ -1 +1 @@\n-old\n+new\n"
	f := seedFreshGit(diff, "M\tapi.go\n", "1\t1\tapi.go\n")
	f.Seed("claude", runner.Result{Stdout: "notes"}, nil)

	res := version.Resolution{Tag: "v2.0.0", PreviousTag: "v1.0.0"}
	if _, err := engine.RegenerateFreshBody(t.Context(), f, nil, root, cfg, res); err != nil {
		t.Fatalf("RegenerateFreshBody returned unexpected error: %v", err)
	}

	assertOnlyArgv(t, f, "claude", leanClaudeArgs)
}
