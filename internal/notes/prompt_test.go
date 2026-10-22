package notes_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/notes"
)

// writeFile writes body to {dir}/{name} (creating parent dirs) and returns name,
// failing the test on error. Used to stage context/prompt files under t.TempDir().
func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating dir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return name
}

// assertOrder fails unless each marker appears in s and they appear in the given
// order, each strictly after the previous. It is the load-bearing assertion for
// the composed AI input: instructions, then Change Map, then diff — nothing
// reordered, nothing interleaved.
func assertOrder(t *testing.T, s string, markers ...string) {
	t.Helper()

	prev := 0
	for i, m := range markers {
		idx := strings.Index(s[prev:], m)
		if idx < 0 {
			t.Fatalf("marker %d (%q) not found after offset %d in:\n%s", i, m, prev, s)
		}
		prev += idx + len(m)
	}
}

// defaultPromptRules enumerates the rules the default notes prompt MUST carry,
// each as a distinct substring the prompt is required to contain. Each entry pins
// one spec requirement so a regression that drops a rule fails a named subtest.
func defaultPromptRules() []struct{ name, want string } {
	return []struct{ name, want string }{
		{"one line one sentence per bullet", "ONE line and ONE sentence"},
		{"notes are for the people who use the software", "FOR THE PEOPLE WHO USE THE SOFTWARE"},
		{"no tl;dr", "NO TL;DR"},
		{"breaking section emoji header", "💥 Breaking"},
		{"added section emoji header", "✨ Added"},
		{"changed section emoji header", "🔧 Changed"},
		{"deprecated section emoji header", "⚠️ Deprecated"},
		{"removed section emoji header", "🗑️ Removed"},
		{"fixed section emoji header", "🐛 Fixed"},
		{"security section emoji header", "🔒 Security"},
		{"omit empty sections", "Omit empty sections"},
		{"unit of entry is the notable change", "notable change"},
		{"ignore version-number bumps", "version-number bumps"},
		{"no preamble", "no preamble"},
		{"no meta-commentary", "no meta-commentary"},
		{"rank with the change map", "Rank importance"},
		{"describe from the diff", "from the DIFF"},
		{"deprecated/security only on explicit marker", "textual marker"},
	}
}

func TestDefaultPrompt_CarriesEveryRule(t *testing.T) {
	t.Parallel()

	for _, rule := range defaultPromptRules() {
		t.Run(rule.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(notes.DefaultPrompt, rule.want) {
				t.Errorf("default prompt missing rule %q (substring %q):\n%s", rule.name, rule.want, notes.DefaultPrompt)
			}
		})
	}
}

func TestDefaultPrompt_RequestsNoMachineParseableWrapper(t *testing.T) {
	t.Parallel()

	// The AI returns the presentation format DIRECTLY — no machine-parseable
	// wrapper labels. The prompt must say so explicitly so no wrapper is requested.
	if !strings.Contains(notes.DefaultPrompt, "Return the notes directly") {
		t.Errorf("default prompt does not request notes returned directly (no machine wrapper):\n%s", notes.DefaultPrompt)
	}
}

// normalizeSpace collapses every run of whitespace to one space, so a rule is
// matched as words regardless of where the prompt wraps its lines.
func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// assertPromptSays fails unless the default prompt carries each phrase, compared
// word for word with whitespace normalised.
func assertPromptSays(t *testing.T, phrases ...string) {
	t.Helper()

	prompt := normalizeSpace(notes.DefaultPrompt)
	for _, phrase := range phrases {
		if !strings.Contains(prompt, normalizeSpace(phrase)) {
			t.Errorf("default prompt missing %q:\n%s", phrase, notes.DefaultPrompt)
		}
	}
}

func TestDefaultPrompt_AsksForOptionalHeadlineAsFirstLine(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"HEADLINE (optional): one plain sentence, only where the release has one, as the first line of the notes, followed by one blank line and then the first section.",
		"Where the release has no headline, the notes open with the first section.",
	)
}

func TestDefaultPrompt_ListsSevenSectionsInFixedOrder(t *testing.T) {
	t.Parallel()

	assertOrder(t, notes.DefaultPrompt,
		"\n    💥 Breaking    — ",
		"\n    ✨ Added       — ",
		"\n    🔧 Changed     — ",
		"\n    ⚠️ Deprecated  — ",
		"\n    🗑️ Removed     — ",
		"\n    🐛 Fixed       — ",
		"\n    🔒 Security    — ",
	)
	assertPromptSays(t,
		"in this order. Omit empty sections entirely — emit only a section that has at least one item.",
		"Keep this order in every release, whatever its main work.",
	)
}

func TestDefaultPrompt_AllowsNothingAboveFirstSectionButHeadline(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Nothing goes above the first section but the headline: NO summary paragraph, NO TL;DR, NO label, and NO other preamble.",
		"Do NOT wrap them in any machine-parseable labels, headings, or markers beyond the format above.",
		"no preamble and no meta-commentary. Do not explain what you are doing, do not restate these instructions, do not add a sign-off.",
	)
	if strings.Contains(normalizeSpace(notes.DefaultPrompt), "start directly with the first section header") {
		t.Errorf("default prompt still asks to start directly with the first section header:\n%s", notes.DefaultPrompt)
	}
}

func TestDefaultPrompt_ListsEveryChangeOnce(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"List every change once, in one section. The headline's summary of the release's main work is the only restatement in the notes.",
	)
}

func TestDefaultPrompt_HoldsHeadlineToBulletsAltitude(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"The headline sums up the release's main body of work in ONE terse sentence, in plain language, at the same altitude as the bullets.",
		"It names that body of work and leads with what it gives the user: what they can now do, or no longer suffer, in the terms they meet it — never in files, formats or internals.",
		"It is NOT a synthesis of everything in the release, and NOT a narrative thread that ties the changes together.",
	)
}

func TestDefaultPrompt_BarsScaleWordsAndInventoryFromHeadline(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		`Lead with the benefit, not the size of the work: no "major", "overhaul", "powerful" or other scale or praise words, and no inventory of the work's pieces. The sections carry the scale.`,
	)
}

func TestDefaultPrompt_LetsHeadlineNameAnyKindOfChange(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"The main body of work may be a feature, a rework, a fix or a large epic. Name it whatever section its pieces sit in.",
	)
}

func TestDefaultPrompt_KeepsHeadlinePlainText(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		`Plain text: no bold, no emoji, and no label such as "Highlights:" or "TL;DR:".`,
	)
}

func TestDefaultPrompt_KeepsSectionsCompleteBelowHeadline(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"The headline does not relieve the sections: list every notable change below it exactly as you would without one, the pieces of the main work included.",
	)
}

func TestDefaultPrompt_PicksHeadlineByWhatReleaseChangesForUser(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Pick the headline by what the release changes for its user, read from the diff. A new folder or a large file is not, of itself, the release's main thing, and the Change Map does not pick the headline.",
	)
}

func TestDefaultPrompt_WritesHeadlineOnlyForMainBodyOfWork(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		`Write a headline only when the release has a MAIN BODY OF WORK: one feature, rework or fix that anyone asked "what is this release?" would name.`,
		"Test it against your own bullets: either most of the notable changes serve it, or it alone clearly outweighs all the other changes put together.",
	)
}

func TestDefaultPrompt_LargestOfSeveralChangesIsNoMainBody(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"The largest of several separate changes is NOT a main body of work.",
	)
}

func TestDefaultPrompt_NamesTwoCoEqualBodiesOnlyWhereEachPassesAlone(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Two bodies of work of EQUAL weight are named together, in the one sentence and led by the benefit, only where EACH passes the test once the other is set aside.",
	)
}

func TestDefaultPrompt_NamesSecondSubjectOnlyWhenCoEqualBody(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		`Name a second subject only when it is a co-equal body of work: never two pieces of one body, and never a main body plus a lesser piece, whether "and" or anything else joins them.`,
	)
}

func TestDefaultPrompt_CapsHeadlineAtTwoBodiesInOneSentence(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A headline names at most two bodies of work, in one sentence, even where a release has more than two co-equal bodies.",
	)
}

func TestDefaultPrompt_LeavesHeadlineOutWhereOneBulletStatesWholeRelease(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Leave the headline out where one bullet already states the whole release and nothing else stands beside it: there the headline would only repeat that bullet.",
		"Judge that by whether a single bullet states the whole release, not by how many bullets there are.",
	)
}

func TestDefaultPrompt_KeepsHeadlineWhereMainWorkSpreadsOverSeveralBullets(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Keep the headline where the main work is spread over several bullets and none of them names the whole, even when the notes hold nothing else: it is the one line that says what the release is.",
	)
}

func TestDefaultPrompt_KeepsHeadlineForSingleBulletFeatureBesideUnrelatedChanges(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Keep it too for a main feature written as a single bullet beside unrelated changes: there is a rest to set it apart from.",
	)
}

func TestDefaultPrompt_WritesNoHeadlineOnGrabBag(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Write NO headline on a grab-bag of small unrelated changes, however large the largest of them, and stitch no theme across such changes.",
	)
}

func TestDefaultPrompt_LeavesHeadlineOutWhenInDoubt(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When in doubt, leave it out: write a headline only when one body of work, or two co-equal bodies, clearly stands out, and otherwise start the notes with the first section.",
		"A missing headline costs little; a wrong one misstates the release in its most-read line.",
	)
}

func TestDefaultPrompt_PlacesEveryChangeAgainstPreviousRelease(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Which section a change belongs in — judge every change against the PREVIOUS release, never against earlier states inside this one:",
	)
}

func TestDefaultPrompt_FilesWhatPreviousReleaseLackedUnderAdded(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Something the previous release did not have is Added. Its pieces are Added too, even where the diff shows them adjusted, capped or fixed along the way.",
		"A bug in something new in this release is never Fixed, and a tweak to it is never Changed.",
	)
}

func TestDefaultPrompt_FilesReworkAndNonFixChangeOfExistingBehaviourUnderChanged(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A rework or redesign of something the previous release already had is Changed, and so is any smaller change to how it works that is not a fix, such as a speed-up users notice or a changed default that is not breaking.",
		"A genuinely new capability is Added, whether it ships inside a rework or is added to something the previous release had, such as a new option on an existing command.",
	)
}

func TestDefaultPrompt_FilesWhatBehavedWronglyInPreviousReleaseUnderFixed(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Something that behaved wrongly in the previous release and now behaves correctly is Fixed, however the diff or its comments frame it.",
		"Behaving wrongly covers wrong output or placement, a crash or error, dropped or lost data, and something not working.",
		"That includes a rework whose purpose is the fix, and something an earlier release lost that this one brings back.",
	)
}

func TestDefaultPrompt_FilesWhatPreviousReleaseHadAndIsGoneUnderRemoved(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Something the previous release had that is gone is Removed. A removal that breaks existing use goes under 💥 Breaking only.",
	)
}

func TestDefaultPrompt_CarriesBulletRulesUnchanged(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		`Each bullet is ONE line and ONE sentence: name the change in plain language, and
where it helps add the user-facing effect after an em dash. That single line is
the whole bullet — NO second sentence, NO bold lead-in label, NO sub-bullets, NO
implementation detail, and NO rationale for how it was built. Aim for a line a
user understands at a glance, not a fragment they have to decode.

Examples of the right altitude (what to aim for):

    - Single-keypress review gates — press y/n/e/r to decide, no Enter needed.
    - Failures no longer print twice when stdout and stderr share a terminal.
    - Pin the AI model with `+"`ai_command = \"claude -p --model sonnet\"`"+`.`,
		`ONE bullet per notable change — not per file, hunk, or commit. Combine
closely-related changes into a single bullet. Ignore version-number bumps and
other trivial bookkeeping churn. Most releases are a handful of bullets; do not
pad.`,
		`Deprecated and Security are opportunistic — emit an item ONLY on an explicit
textual marker in the diff (a @deprecated annotation for Deprecated; an obvious
auth / cryptography / input-validation / CVE-bump surface for Security). Do NOT
infer them speculatively. These sections are empty in most releases.`,
	)
}

func TestComposePrompt_OrdersInstructionsThenChangeMapThenDiff(t *testing.T) {
	t.Parallel()

	// The composed AI input is EXACTLY instructions + Change Map preamble +
	// post-exclusion (capped) diff, in that order, nothing else. Distinct sentinels
	// stand in for each part so the assertion is on ORDERING, not content.
	instructions := "INSTRUCTIONS_SENTINEL"
	changeMap := "CHANGEMAP_SENTINEL"
	diff := "DIFF_SENTINEL"

	got := notes.ComposePrompt(instructions, changeMap, diff)

	assertOrder(t, got, instructions, changeMap, diff)
}

func TestComposePrompt_ContainsOnlyTheThreeParts(t *testing.T) {
	t.Parallel()

	// "Nothing else": every part's content survives composition and the result is
	// built only from the three inputs (plus join separators), so the three pieces
	// collectively account for all the non-whitespace content.
	instructions := "AAA instructions AAA"
	changeMap := "BBB change map BBB"
	diff := "CCC diff CCC"

	got := notes.ComposePrompt(instructions, changeMap, diff)

	if !strings.Contains(got, instructions) {
		t.Errorf("composed prompt missing instructions:\n%s", got)
	}
	if !strings.Contains(got, changeMap) {
		t.Errorf("composed prompt missing change map:\n%s", got)
	}
	if !strings.Contains(got, diff) {
		t.Errorf("composed prompt missing diff:\n%s", got)
	}

	// Stripping the three parts, the closing OutputReminder, and whitespace leaves
	// only separator characters — no smuggled extra content beyond the declared
	// compose. The reminder must sit LAST (recency is its whole point).
	if !strings.HasSuffix(got, notes.OutputReminder) {
		t.Errorf("composed prompt must END with the OutputReminder:\n%s", got)
	}
	residue := got
	for _, part := range []string{instructions, changeMap, diff, notes.OutputReminder} {
		residue = strings.Replace(residue, part, "", 1)
	}
	if strings.TrimSpace(residue) != "" {
		t.Errorf("composed prompt carries content beyond the declared parts: %q", strings.TrimSpace(residue))
	}
}

func TestResolveInstructions_NoContextNoPrompt_ReturnsDefaultVerbatim(t *testing.T) {
	t.Parallel()

	// With neither knob set, the instructions are exactly the default prompt — the
	// baseline mint ships.
	got, err := notes.ResolveInstructions(t.TempDir(), config.Release{})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if got != notes.DefaultPrompt {
		t.Errorf("instructions = %q, want the default prompt verbatim", got)
	}
}

func TestResolveInstructions_ContextString_InjectedNotReplacing(t *testing.T) {
	t.Parallel()

	// A [release].context whose value is NOT an existing file path is a literal
	// string, injected into (not replacing) the default prompt: the context text
	// appears AND the default rules still survive.
	ctx := "Project is a dev-workflow toolkit; emphasise user-facing changes."
	got, err := notes.ResolveInstructions(t.TempDir(), config.Release{Context: ctx})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if !strings.Contains(got, ctx) {
		t.Errorf("instructions missing injected context string:\n%s", got)
	}
	// Default rules must still be present — inject, never replace.
	if !strings.Contains(got, "💥 Breaking") || !strings.Contains(got, "✨ Added") {
		t.Errorf("default prompt rules absent after context inject:\n%s", got)
	}
}

func TestResolveInstructions_ContextFile_FileContentsInjected(t *testing.T) {
	t.Parallel()

	// When [release].context names an EXISTING file (resolved relative to root), its
	// CONTENTS are injected — not the path string itself.
	dir := t.TempDir()
	contents := "Audience: open-source maintainers. Lead with breaking changes."
	name := writeFile(t, dir, ".mint/context.md", contents)

	got, err := notes.ResolveInstructions(dir, config.Release{Context: name})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if !strings.Contains(got, contents) {
		t.Errorf("instructions missing injected context FILE contents:\n%s", got)
	}
	if strings.Contains(got, name) {
		t.Errorf("instructions leaked the context file PATH %q instead of its contents", name)
	}
	if !strings.Contains(got, "✨ Added") {
		t.Errorf("default prompt rules absent after context-file inject:\n%s", got)
	}
}

func TestResolveInstructions_PromptFile_FullyOverridesDefault(t *testing.T) {
	t.Parallel()

	// A [release].prompt is a file path whose CONTENTS fully replace the default
	// prompt: the override text is the instructions, and the default prompt's rules
	// are ABSENT (mint still appends Change Map + diff downstream via ComposePrompt).
	dir := t.TempDir()
	override := "Write a haiku summarising the release. Nothing else."
	name := writeFile(t, dir, ".mint/prompt.md", override)

	got, err := notes.ResolveInstructions(dir, config.Release{Prompt: name})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if got != override {
		t.Errorf("instructions = %q, want the override file contents verbatim", got)
	}
	if strings.Contains(got, "✨ Added") {
		t.Errorf("default prompt rules leaked into a full override:\n%s", got)
	}
}

func TestResolveInstructions_PromptOverrideStillFlowsThroughComposeWithChangeMapAndDiff(t *testing.T) {
	t.Parallel()

	// End-to-end of the override contract: the prompt file fully overrides the
	// instructions, but ComposePrompt still appends the Change Map and diff after it.
	dir := t.TempDir()
	override := "OVERRIDE_INSTRUCTIONS"
	name := writeFile(t, dir, "prompt.txt", override)

	instructions, err := notes.ResolveInstructions(dir, config.Release{Prompt: name})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	changeMap := "THE_CHANGE_MAP"
	diff := "THE_DIFF"
	composed := notes.ComposePrompt(instructions, changeMap, diff)

	assertOrder(t, composed, override, changeMap, diff)
	if strings.Contains(composed, "✨ Added") {
		t.Errorf("default prompt rules present despite full override:\n%s", composed)
	}
}

func TestResolveInstructions_PromptTakesPrecedenceOverContext(t *testing.T) {
	t.Parallel()

	// When both knobs are set, prompt (full override) wins — context is a default-
	// prompt inject knob and the default prompt is gone under a full override.
	dir := t.TempDir()
	override := "FULL OVERRIDE BODY"
	promptName := writeFile(t, dir, "prompt.txt", override)

	got, err := notes.ResolveInstructions(dir, config.Release{
		Prompt:  promptName,
		Context: "this context must be ignored",
	})
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if got != override {
		t.Errorf("instructions = %q, want the override verbatim (prompt wins over context)", got)
	}
	if strings.Contains(got, "this context must be ignored") {
		t.Errorf("context injected despite a full prompt override:\n%s", got)
	}
}

func TestResolveInstructions_PromptFileMissing_ReturnsError(t *testing.T) {
	t.Parallel()

	// A configured [release].prompt is an explicit operator choice; a missing file
	// is a real error, not a silent fall-back to the default prompt.
	_, err := notes.ResolveInstructions(t.TempDir(), config.Release{Prompt: "does/not/exist.md"})
	if err == nil {
		t.Fatal("ResolveInstructions returned nil error for a missing prompt file, want non-nil")
	}
}
