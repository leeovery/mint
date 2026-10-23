package notes_test

import (
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// formatNeutralReminder is spelled out, not read from notes.OutputReminder: the
// exact wording is the contract.
const formatNeutralReminder = "Now output ONLY the release notes themselves, in the format above, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes."

func TestOutputReminder_IsTheFormatNeutralWording(t *testing.T) {
	t.Parallel()

	if notes.OutputReminder != formatNeutralReminder {
		t.Errorf("OutputReminder = %q, want %q", notes.OutputReminder, formatNeutralReminder)
	}
	if strings.Contains(strings.ToLower(notes.OutputReminder), "headline") {
		t.Errorf("OutputReminder names a headline: %q", notes.OutputReminder)
	}
}

// reminderRepo is one repo's prompt setup for the single-call reminder tests.
type reminderRepo struct {
	name    string
	release func(t *testing.T, root string) config.Release
	// instructions is what the call must open with: the governing instructions.
	instructions string
}

const (
	reminderContext  = "Our users are operators running the tool in CI."
	reminderOverride = "Write a terse bullet list of what changed. Nothing else."
)

func reminderRepos() []reminderRepo {
	return []reminderRepo{
		{
			name:         "default prompt",
			release:      func(*testing.T, string) config.Release { return config.Release{} },
			instructions: notes.DefaultPrompt,
		},
		{
			name: "default prompt with [release].context",
			release: func(*testing.T, string) config.Release {
				return config.Release{Context: reminderContext}
			},
			instructions: notes.DefaultPrompt + "\n\nAdditional project context (apply the rules above with this in mind):\n\n" + reminderContext,
		},
		{
			name: "full [release].prompt override",
			release: func(t *testing.T, root string) config.Release {
				return config.Release{Prompt: writeFile(t, root, ".mint/notes-prompt.md", reminderOverride)}
			},
			instructions: reminderOverride,
		},
	}
}

func TestGenerator_SinglePass_PromptIsInstructionsChangeMapDiffThenReminder(t *testing.T) {
	t.Parallel()

	for _, repo := range reminderRepos() {
		t.Run(repo.name, func(t *testing.T) {
			t.Parallel()

			diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
			const nameStatus = "M\tcore/run.go\n"
			const numstat = "3\t3\tcore/run.go\n"
			r := seedNormalPathGit(t, diff, nameStatus, numstat)
			transport := &recordingTransport{body: "notes body"}
			root := t.TempDir()
			cfg := normalCfg()
			cfg.Release = repo.release(t, root)

			gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, root)
			if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			if transport.calls() != 1 {
				t.Fatalf("transport calls = %d, want one single-pass call", transport.calls())
			}
			changeMap, err := notes.NewAssembler(seedChangeMapGit(t, nameStatus, numstat), "/repo", notes.ExcludeConfig{}).
				BuildChangeMap(t.Context(), "v1.0.0", config.DefaultMaxPromptBytes)
			if err != nil {
				t.Fatalf("BuildChangeMap returned unexpected error: %v", err)
			}
			prompt := transport.lastPrompt(t)
			if !strings.HasSuffix(prompt, "\n\n"+formatNeutralReminder) {
				t.Errorf("prompt does not end with the format-neutral reminder:\n...%s", tail(prompt, 300))
			}
			want := repo.instructions + "\n\n" + changeMap + "\n\n" + diff + "\n\n" + formatNeutralReminder
			if prompt != want {
				t.Errorf("prompt is not instructions, Change Map, diff and reminder alone:\n got: %q\nwant: %q", prompt, want)
			}
			for _, inv := range r.Invocations() {
				if len(inv.Args) > 0 && inv.Args[0] == "log" {
					t.Errorf("commit history reached the prompt via git %v", inv.Args)
				}
			}
		})
	}
}

func TestGenerator_SinglePass_OverrideCallAsksForNoHeadline(t *testing.T) {
	t.Parallel()

	// Everything mint adds around an override repo's prompt file, the reminder
	// included, leaves the headline to the operator's format.
	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
	r := seedNormalPathGit(t, diff, "M\tcore/run.go\n", "3\t3\tcore/run.go\n")
	transport := &recordingTransport{body: "notes body"}
	root := t.TempDir()
	cfg := normalCfg()
	cfg.Release = config.Release{Prompt: writeFile(t, root, "notes-prompt.md", reminderOverride)}

	gen := notes.NewGenerator(notes.NewAssembler(r, "/repo", notes.ExcludeConfig{}), transport, root)
	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	added, ok := strings.CutPrefix(transport.lastPrompt(t), reminderOverride+"\n\n")
	if !ok {
		t.Fatalf("override call does not open with the prompt file verbatim:\n%.300s", transport.lastPrompt(t))
	}
	if strings.Contains(strings.ToLower(added), "headline") {
		t.Errorf("mint's additions to an override call name a headline:\n%s", added)
	}
}

func TestGenerator_OverCap_EveryChunkPieceAndMergeCallEndsWithReminder(t *testing.T) {
	t.Parallel()

	ct := &chunkTransport{mergeBody: "merged notes"}
	files := []pieceFile{
		{"alpha/a.go", fileDiff("alpha/a.go", 10_000)},
		{"big/huge.go", hunkedFileDiff("big/huge.go", 12, 5_000)},
		{"gamma/c.go", fileDiff("gamma/c.go", 10_000)},
	}
	runPieces(t, ct, files)

	if n := len(piecesOf(t, ct, "big/huge.go")); n < 2 {
		t.Fatalf("pieces of big/huge.go = %d, want the file sent as pieces", n)
	}
	if n := len(ct.chunkPrompts()); n < 4 {
		t.Fatalf("chunk calls = %d, want whole-file chunks beside the pieces", n)
	}
	if n := len(ct.mergePrompts()); n != 1 {
		t.Fatalf("merge calls = %d, want 1", n)
	}
	for i, p := range ct.prompts {
		if !strings.HasSuffix(p, "\n\n"+formatNeutralReminder) {
			t.Errorf("call %d does not end with the format-neutral reminder:\n...%s", i+1, tail(p, 300))
		}
	}
}

func TestGenerator_OverCap_DefaultPromptChunkAndMergeCallsEndWithReminder(t *testing.T) {
	t.Parallel()

	s := chunkScenario{files: threeTenKBFiles(), root: t.TempDir()}
	s.cfg = config.Config{MaxPromptBytes: config.DefaultMaxPromptBytes}
	s.cfg.MaxPromptBytes = s.fixedPart(t) + 15_000

	ct, _ := s.run(t, "merged notes")

	if len(ct.chunkPrompts()) < 2 || len(ct.mergePrompts()) != 1 {
		t.Fatalf("chunk calls = %d, merge calls = %d; want a chunked run", len(ct.chunkPrompts()), len(ct.mergePrompts()))
	}
	for i, p := range ct.prompts {
		if !strings.HasPrefix(p, notes.DefaultPrompt+"\n\n") {
			t.Errorf("call %d does not open with the default prompt", i+1)
		}
		if !strings.HasSuffix(p, "\n\n"+formatNeutralReminder) {
			t.Errorf("call %d does not end with the format-neutral reminder:\n...%s", i+1, tail(p, 300))
		}
	}
}

// tail returns the last n bytes of s, for failure messages over long prompts.
func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
