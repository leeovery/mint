package notes_test

import (
	"strings"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// trueToReleaseRules are the release's truth rules, each as the call's
// instructions must carry it, compared with whitespace normalised.
var trueToReleaseRules = []string{
	"Documentation, READMEs, comments, examples and tests DESCRIBE behaviour; editing them does not change it.",
	"Where the files ARE the product, such as prompts, skills or templates the software ships and uses, changing them changes the product.",
	"A change to tests alone is not a fix.",
	"Never present work built in this release as a change to, or fix of, the previous release.",
	"Never present as new anything the previous release already had, wherever the diff shows it: in unchanged context lines, in removed lines, or in code the diff only moves or renames.",
	"Read the diff as the two releases side by side: its removed lines and its unchanged context lines are the previous release, its added lines are this one.",
}

func TestGenerator_SinglePass_AICommandStdinCarriesTheTruthRulesAndNothingNew(t *testing.T) {
	t.Parallel()

	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
	const nameStatus = "M\tcore/run.go\n"
	const numstat = "3\t3\tcore/run.go\n"
	gitRunner := seedNormalPathGit(t, diff, nameStatus, numstat)
	aiRunner := runner.NewFakeRunner()
	aiRunner.Seed("claude", runner.Result{Stdout: "✨ Added\n- Something new.\n"}, nil)
	timeout := time.Minute
	transport := ai.NewTransport(aiRunner, ai.Config{AICommand: "claude -p", Timeout: &timeout})

	gen := notes.NewGenerator(notes.NewAssembler(gitRunner, "/repo", notes.ExcludeConfig{}), transport, t.TempDir())
	if _, err := gen.Generate(t.Context(), "v1.0.0", normalCfg()); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	calls := aiRunner.Invocations()
	if len(calls) != 1 {
		t.Fatalf("ai_command calls = %d, want one single-pass call", len(calls))
	}
	stdin := calls[0].Stdin
	changeMap, err := notes.NewAssembler(seedChangeMapGit(t, nameStatus, numstat), "/repo", notes.ExcludeConfig{}).
		BuildChangeMap(t.Context(), "v1.0.0", config.DefaultMaxPromptBytes)
	if err != nil {
		t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
	}
	if want := notes.DefaultPrompt + "\n\n" + changeMap + "\n\n" + diff + "\n\n" + formatNeutralReminder; stdin != want {
		t.Errorf("stdin is not the instructions, Change Map, diff and reminder alone:\n got: %q\nwant: %q", stdin, want)
	}
	instructions := normalizeSpace(strings.SplitN(stdin, "\n\n"+changeMap, 2)[0])
	for _, rule := range trueToReleaseRules {
		if !strings.Contains(instructions, normalizeSpace(rule)) {
			t.Errorf("instructions on stdin missing %q", rule)
		}
	}
	for _, inv := range gitRunner.Invocations() {
		if len(inv.Args) > 0 && inv.Args[0] == "log" {
			t.Errorf("commit history reached the call via git %v", inv.Args)
		}
	}
}
