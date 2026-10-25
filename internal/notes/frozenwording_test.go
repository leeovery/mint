package notes_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/runner"
)

// formatRules are the notes format's rules, each as the default prompt must carry
// it on the AI command's stdin, compared with whitespace normalised.
var formatRules = []struct{ name, want string }{
	{"format: optional headline then sections", "Format — an optional headline, then categorized sections:"},
	{"format: headline is the first line", "HEADLINE (optional): one plain sentence, only where the release has one, as the first line of the notes, followed by one blank line and then the first section. Where the release has no headline, the notes open with the first section."},
	{"format: sections omitted when empty", "Omit empty sections entirely — emit only a section that has at least one item."},
	{"format: fixed order in every release", "Keep this order in every release, whatever its main work."},
	{"format: section order", "💥 Breaking — changes that stop existing use working, or change it in a way users must act on ✨ Added — capabilities the previous release did not have 🔧 Changed — changes to behaviour the previous release already had ⚠️ Deprecated — features marked for future removal 🗑️ Removed — features the previous release had that are gone 🐛 Fixed — things that were broken in the previous release and now work 🔒 Security — security-relevant changes"},
	{"format: nothing above the first section but the headline", "Nothing goes above the first section but the headline: NO summary paragraph, NO TL;DR, NO label, and NO other preamble."},
	{"format: no wrapper", "Do NOT wrap them in any machine-parseable labels, headings, or markers beyond the format above."},
	{"format: no commentary or sign-off", "Strict: nothing else — no preamble and no meta-commentary. Do not explain what you are doing, do not restate these instructions, do not add a sign-off."},
	{"bullets: one line one sentence", "Each bullet is ONE line and ONE sentence: name the change in plain language, and where it helps add the user-facing effect after an em dash. That single line is the whole bullet — NO second sentence, NO bold lead-in label, NO sub-bullets, NO implementation detail, and NO rationale for how it was built."},
	{"bullets: three altitude examples as shipped", "Examples of the right altitude (what to aim for):\n\n    - Single-keypress review gates — press y/n/e/r to decide, no Enter needed.\n    - Failures no longer print twice when stdout and stderr share a terminal.\n    - Pin the AI model with `ai_command = \"claude -p --model sonnet\"`."},
	{"bullets: one per notable change", "ONE bullet per notable change — not per file, hunk, or commit."},
	{"bullets: combine rule word for word", "Combine closely-related changes into a single bullet."},
	{"bullets: bookkeeping ignored, no padding", "Ignore version-number bumps and other trivial bookkeeping churn. Most releases are a handful of bullets; do not pad."},
	{"bullets: deprecated and security opportunistic", "emit an item ONLY on an explicit textual marker in the diff"},
	{"each change once", "List every change once, in one section. The headline's summary of the release's main work is the only restatement in the notes."},
	{"headline: main body of work", "Write a headline only when the release has a MAIN BODY OF WORK"},
	{"headline: largest of several is no main body", "The largest of several separate changes is NOT a main body of work."},
	{"headline: two co-equal bodies", "Two bodies of work of EQUAL weight are named together, in the one sentence and led by the benefit, only where EACH passes the test once the other is set aside."},
	{"headline: second subject only when co-equal", "Name a second subject only when it is a co-equal body of work: never two pieces of one body, and never a main body plus a lesser piece"},
	{"headline: at most two bodies", "A headline names at most two bodies of work, in one sentence"},
	{"headline: none where one bullet states the release", "Leave the headline out where one bullet already states the whole release and nothing else stands beside it"},
	{"headline: kept where main work spreads over bullets", "Keep the headline where the main work is spread over several bullets and none of them names the whole"},
	{"headline: none on a grab-bag", "Write NO headline on a grab-bag of small unrelated changes"},
	{"headline: when in doubt leave it out", "When in doubt, leave it out"},
	{"headline: one terse sentence at bullet altitude", "The headline sums up the release's main body of work in ONE terse sentence, in plain language, at the same altitude as the bullets."},
	{"headline: no synthesis or narrative", "It is NOT a synthesis of everything in the release, and NOT a narrative thread that ties the changes together."},
	{"headline: benefit not scale", "Lead with the benefit, not the size of the work"},
	{"headline: any kind of change", "The main body of work may be a feature, a rework, a fix or a large epic."},
	{"headline: plain text", "Plain text: no bold, no emoji, and no label such as \"Highlights:\" or \"TL;DR:\"."},
	{"headline: sections not relieved", "The headline does not relieve the sections"},
	{"headline: picked by what changes for the user", "Pick the headline by what the release changes for its user, read from the diff."},
	{"headline: none in a part's notes", "When you write the notes for one part of a release, write no headline, not even one for that part"},
	{"headline: merge judges from merged bullets", "When you write the release's notes from the notes of its parts, judge its main body of work by the merged bullets, with the Change Map beside them."},
	{"placement: against the previous release", "judge every change against the PREVIOUS release, never against earlier states inside this one"},
	{"placement: added", "Something the previous release did not have is Added."},
	{"placement: changed", "A rework or redesign of something the previous release already had is Changed"},
	{"placement: fixed", "Something that behaved wrongly in the previous release and now behaves correctly is Fixed"},
	{"placement: removed", "Something the previous release had that is gone is Removed."},
	{"breaking: only there", "A breaking change goes under 💥 Breaking only, never repeated in another section."},
	{"breaking: what qualifies", "Put a change there ONLY where the diff shows that something users already do stops working, or gives a different result they must act on"},
	{"breaking: what does not", "A change that only adds, keeps more, allows more or improves is not breaking."},
	{"breaking: unsure is not breaking", "When unsure whether a change is breaking, it is not: put it in its usual section."},
	{"breaking: what breaks and what to do", "Each 💥 Breaking bullet says, in its one sentence, what breaks and what to do. Take what to do only from what the diff shows"},
	{"users meet: in", "Include only changes a user of the software meets"},
	{"users meet: out", "Leave out how the software is built and maintained"},
	{"users meet: the test", "The test is whether a user meets or notices the change, not whether the code behind it is internal"},
	{"users meet: users' terms", "Describe every change as users see and use it, never in the code's names, the mechanism, or a design document's jargon."},
	{"maintenance line", "Where every change in the release is one these rules leave out, the notes are ONE plain line saying the release is maintenance with no user-facing changes, in the shape of \"Maintenance release — no notable source changes\". That line has no headline and no sections"},
	{"maintenance line: change in doubt listed", "When unsure whether the release holds any change users meet, list the change in doubt in its section and do not write the maintenance line."},
	{"truth: docs describe behaviour", "Documentation, READMEs, comments, examples and tests DESCRIBE behaviour; editing them does not change it."},
	{"truth: test-only change is no fix", "A change to tests alone is not a fix."},
	{"truth: work built in this release", "Never present work built in this release as a change to, or fix of, the previous release."},
	{"truth: nothing the previous release had is new", "Never present as new anything the previous release already had, wherever the diff shows it: in unchanged context lines, in removed lines, or in code the diff only moves or renames."},
	{"ordering: by what changes mean to the user", "Within each section, order the changes by what they mean to the user, most important first."},
	{"ordering: weight in users' terms", "Weight is how much of the product a change touches, how often users meet it, and how much it changes their experience."},
	{"ordering: main work leads", "The pieces of the release's main work lead their sections."},
	{"ordering: not lines or files", "Lines, files, commits, and how easy a change is to name, are not weight."},
	{"change map: coverage check", "Use it to make sure nothing large is missed, not as a measure of importance"},
	{"change map: metadata", "DESCRIBE every change from the DIFF; the Change Map is metadata, never content."},
}

// stdinRecordingAICommand is an ai_command whose every call answers body and is
// recorded with its stdin.
func stdinRecordingAICommand(body string) (*runner.FakeRunner, *ai.Transport) {
	aiRunner := runner.NewFakeRunner()
	aiRunner.Seed("claude", runner.Result{Stdout: body}, nil)
	timeout := time.Minute
	return aiRunner, ai.NewTransport(aiRunner, ai.Config{AICommand: "claude -p", Timeout: &timeout})
}

func TestGenerator_SinglePass_DefaultPrompt_AICommandStdinCarriesEveryFormatRule(t *testing.T) {
	t.Parallel()

	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
	const nameStatus = "M\tcore/run.go\n"
	const numstat = "3\t3\tcore/run.go\n"
	aiRunner, transport := stdinRecordingAICommand("✨ Added\n- Something new.")
	gen := notes.NewGenerator(notes.NewAssembler(seedNormalPathGit(t, diff, nameStatus, numstat), "/repo", notes.ExcludeConfig{}), transport, t.TempDir())

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
		t.Fatalf("stdin is not the default prompt, Change Map, diff and reminder alone:\n got: %q\nwant: %q", stdin, want)
	}
	instructions := normalizeSpace(strings.TrimSuffix(stdin, "\n\n"+changeMap+"\n\n"+diff+"\n\n"+formatNeutralReminder))
	for _, rule := range formatRules {
		t.Run(rule.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(instructions, normalizeSpace(rule.want)) {
				t.Errorf("instructions on stdin missing %q", rule.want)
			}
		})
	}
}

func TestGenerator_SinglePass_FullOverride_AICommandStdinIsThePromptFileAndAsksForNoHeadline(t *testing.T) {
	t.Parallel()

	diff := "diff --git a/core/run.go b/core/run.go\n@@ -1 +1 @@\n-x\n+y\n"
	aiRunner, transport := stdinRecordingAICommand("- Something new.")
	root := t.TempDir()
	cfg := normalCfg()
	cfg.Release = config.Release{Prompt: writeFile(t, root, ".mint/notes-prompt.md", framingOverride)}
	gen := notes.NewGenerator(notes.NewAssembler(seedNormalPathGit(t, diff, "M\tcore/run.go\n", "3\t3\tcore/run.go\n"), "/repo", notes.ExcludeConfig{}), transport, root)

	if _, err := gen.Generate(t.Context(), "v1.0.0", cfg); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	calls := aiRunner.Invocations()
	if len(calls) != 1 {
		t.Fatalf("ai_command calls = %d, want one single-pass call", len(calls))
	}
	added, ok := strings.CutPrefix(calls[0].Stdin, framingOverride+"\n\n")
	if !ok {
		t.Fatalf("stdin does not open with the prompt file verbatim:\n%.300s", calls[0].Stdin)
	}
	if !strings.HasSuffix(added, "\n\n"+formatNeutralReminder) {
		t.Errorf("stdin does not end with the format-neutral reminder:\n...%s", tail(added, 300))
	}
	lower := strings.ToLower(strings.Replace(added, diff, "", 1))
	for _, defaultOnly := range []string{"headline", "maintenance"} {
		if strings.Contains(lower, defaultOnly) {
			t.Errorf("what mint adds around the prompt file names %q, which belongs to the default prompt alone", defaultOnly)
		}
	}
}

func TestGenerator_OverCap_AICommandStdinCarriesTheFrozenFramingsAndReminder(t *testing.T) {
	t.Parallel()

	const partBody = "- notes for one part"
	for _, tt := range mergeFramingReleases() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			aiRunner, transport := stdinRecordingAICommand(partBody)
			root := t.TempDir()
			run := newFramedRun(t, transport, chunksAndPieces(), tt.release(t, root), root)

			if _, err := run.gen.Generate(t.Context(), "v1.0.0", run.cfg); err != nil {
				t.Fatalf("Generate returned unexpected error: %v", err)
			}

			var parts, merges []string
			for _, call := range aiRunner.Invocations() {
				if isChunkPrompt(call.Stdin) {
					parts = append(parts, call.Stdin)
				} else {
					merges = append(merges, call.Stdin)
				}
			}
			pieces := 0
			for _, p := range parts {
				if _, _, ok := pieceOf(p); ok {
					pieces++
				}
			}
			if pieces < 2 || len(parts)-pieces < 2 {
				t.Fatalf("part calls = %d with %d pieces, want whole-file chunks beside pieces", len(parts), pieces)
			}
			for i, p := range parts {
				assertPartCall(t, fmt.Sprintf("part call %d", i+1), p, run.instructions, run.changeMap)
			}
			if len(merges) != 1 {
				t.Fatalf("merge calls = %d, want exactly one", len(merges))
			}
			partNotes := make([]string, len(parts))
			for i := range partNotes {
				partNotes[i] = partBody
			}
			if want := wantMergePrompt(run.instructions, run.changeMap, partNotes); merges[0] != want {
				t.Errorf("merge stdin:\n got: %q\nwant: %q", merges[0], want)
			}
			assertMergeAddsNoHeadlineOrRanking(t, "merge call", merges[0], run.instructions, partNotes)
		})
	}
}
