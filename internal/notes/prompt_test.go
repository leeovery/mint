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

func TestDefaultPrompt_WritesNoHeadlineInOnePartsNotes(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When you write the notes for one part of a release, write no headline, not even one for that part: the release's headline is written once, from every part's notes.",
	)
}

func TestDefaultPrompt_JudgesMainWorkAcrossEveryPartsNotes(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When you write the release's notes from the notes of its parts, judge its main body of work by the merged bullets, with the Change Map beside them.",
		"A change whose diff spans several parts can appear in several parts' notes: it counts once, as it is listed once, and is no weightier for the repeats. The main work may sit in one part's notes alone.",
	)
}

func TestDefaultPrompt_ListsOnlyWhatOnePartsDiffMakes(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When you write the notes for one part of a release, list only the changes that part's diff makes.",
		"The Change Map shows where the rest of the release landed, and the other parts cover it: a test in that part's diff is no evidence of a change made elsewhere.",
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

func TestDefaultPrompt_FilesBreakingChangeUnderBreakingOnly(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A breaking change goes under 💥 Breaking only, never repeated in another section.",
	)
}

func TestDefaultPrompt_CountsOnlyBrokenExistingUseAsBreaking(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Put a change there ONLY where the diff shows that something users already do stops working, or gives a different result they must act on: "+
			"a removed or renamed command, flag, option, setting or key binding; "+
			"a default changed so that existing setups behave differently in a way that matters; "+
			"a changed output or file format that scripts or tools read; "+
			"data or configuration from the previous release no longer read.",
	)
}

func TestDefaultPrompt_CountsAdditionsAndImprovementsAsNotBreaking(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A change that only adds, keeps more, allows more or improves is not breaking.",
	)
}

func TestDefaultPrompt_CountsUnsureChangeAsNotBreaking(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When unsure whether a change is breaking, it is not: put it in its usual section.",
	)
}

func TestDefaultPrompt_AsksBreakingBulletForWhatBreaksAndWhatTheDiffShowsToDo(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Each 💥 Breaking bullet says, in its one sentence, what breaks and what to do.",
		"Take what to do only from what the diff shows, such as a new name, a replacement, or a setting that keeps the old behaviour.",
		"Where the diff shows none, the bullet says what breaks and stops there.",
	)
}

func TestDefaultPrompt_IncludesWhatUsersMeet(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Include only changes a user of the software meets: what it does, its commands, options, configuration, output, errors, performance and compatibility.",
	)
}

func TestDefaultPrompt_LeavesOutHowSoftwareIsBuiltAndMaintained(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Leave out how the software is built and maintained: CI and release tooling, linting, build scripts, test suites and harnesses, "+
			"internal packages, modules and refactors, internal flags and plumbing commands, data-migration numbering, "+
			"contributor docs, and repo housekeeping such as restructuring the README.",
		"A new internal package is not a feature: describe what it lets users do, or leave it out.",
	)
}

func TestDefaultPrompt_TestsWhetherUserMeetsOrNoticesChange(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"The test is whether a user meets or notices the change, not whether the code behind it is internal: a speed-up users notice is listed.",
		"Where the shipped files are the product, such as prompts, skills or templates the software ships and uses, changes to those files count.",
	)
}

func TestDefaultPrompt_DescribesChangesInUsersTerms(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Describe every change as users see and use it, never in the code's names, the mechanism, or a design document's jargon.",
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

func TestDefaultPrompt_AsksForMaintenanceLineWhenNothingUsersMeet(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Where every change in the release is one these rules leave out, the notes are ONE plain line saying the release is maintenance with no user-facing changes, in the shape of \"Maintenance release — no notable source changes\".",
		"Never return empty notes.",
	)
}

func TestDefaultPrompt_GivesMaintenanceLineNoHeadlineAndNoSections(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"That line has no headline and no sections: it is the one body the format allows without sections.",
		"the headline line if the release has one, then the sections, or the one maintenance line for a release with nothing users meet.",
	)
}

func TestDefaultPrompt_ListsChangeInDoubtInsteadOfMaintenanceLine(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When unsure whether the release holds any change users meet, list the change in doubt in its section and do not write the maintenance line.",
	)
}

func TestDefaultPrompt_ListsChangeInDoubtInsteadOfPartsNothingLine(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"When you write the notes for one part of a release and are unsure whether that part's diff holds any change users meet, list the change in doubt in its section, and write neither the line saying the part holds nothing for the notes nor the maintenance line.",
	)
}

func TestDefaultPrompt_ClaimsNoBehaviourFromDocsCommentsExamplesOrTestsAlone(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Claim only behaviour the code in this diff changes.",
		"Documentation, READMEs, comments, examples and tests DESCRIBE behaviour; editing them does not change it.",
		"If every changed line behind it is a comment, documentation, an example or a test, drop the bullet.",
	)
}

func TestDefaultPrompt_CountsChangesToFilesThatAreTheProduct(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Where the files ARE the product, such as prompts, skills or templates the software ships and uses, changing them changes the product.",
	)
}

func TestDefaultPrompt_CountsNoTestOnlyChangeAsFix(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A change to tests alone is not a fix.",
	)
}

func TestDefaultPrompt_NeverPresentsWorkBuiltInReleaseAsChangeToPreviousRelease(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Never present work built in this release as a change to, or fix of, the previous release.",
	)
}

func TestDefaultPrompt_NeverPresentsWhatPreviousReleaseHadAsNew(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Never present as new anything the previous release already had, wherever the diff shows it: in unchanged context lines, in removed lines, or in code the diff only moves or renames.",
	)
}

func TestDefaultPrompt_OrdersEachSectionByWhatChangesMeanToUser(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Within each section, order the changes by what they mean to the user, most important first.",
	)
}

func TestDefaultPrompt_DefinesWeightInUsersTerms(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Weight is how much of the product a change touches, how often users meet it, and how much it changes their experience.",
		"A change to something users do every day outranks a change to a diagnostic or rarely used command.",
	)
}

func TestDefaultPrompt_LeadsSectionsWithPiecesOfMainWork(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"The pieces of the release's main work lead their sections.",
	)
}

func TestDefaultPrompt_CountsNoLinesFilesCommitsOrNameabilityAsWeight(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Lines, files, commits, and how easy a change is to name, are not weight.",
	)
}

func TestDefaultPrompt_DescribesChangeMapAsCoverageCheck(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"You are given a Change Map (a summary of where the release's changes landed) followed by the actual code diff",
		"The Change Map shows where the changes landed. Use it to make sure nothing large is missed, not as a measure of importance: a new folder may hold tooling, and the largest file is often a test or a deletion.",
		"DESCRIBE every change from the DIFF; the Change Map is metadata, never content.",
	)
}

func TestDefaultPrompt_CarriesNoChangeMapRankingRule(t *testing.T) {
	t.Parallel()

	prompt := normalizeSpace(notes.DefaultPrompt)
	for _, gone := range []string{
		"Rank importance using the Change Map",
		"new structure (a whole new package or directory) is the strongest signal",
		"weight it above raw size",
		"salience",
	} {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(gone)) {
			t.Errorf("default prompt still carries %q:\n%s", gone, notes.DefaultPrompt)
		}
	}
}

func TestDefaultPrompt_SpellsOutTheSectionOrderWhereMainWorkSitsLater(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Removed comes before Fixed, and Added before Changed, even where the later section holds the release's main work.",
	)
}

func TestDefaultPrompt_ListsAChangeAndWhatItBringsAsOneChange(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A change and what it brings with it are one change: a removal and the replacement that takes its place, a fix and the feature that makes it, a new mechanism and the behaviour users see from it each get one bullet, in the section where it matters most to the user.",
	)
}

func TestDefaultPrompt_FindsChangesInTheCodeBeforeReadingDocs(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Find the changes in the code first. Go through the diff's code and note what each change does; only then read its documentation, READMEs, examples, comments and tests, and only for the words to describe changes you already found in the code.",
		"Behaviour you find only there is not a change of this release: leave it out.",
		"A new guide or docs page users read is itself a change users meet; what it describes is not.",
	)
}

func TestDefaultPrompt_ReadsRemovedAndContextLinesAsThePreviousRelease(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Read the diff as the two releases side by side: its removed lines and its unchanged context lines are the previous release, its added lines are this one.",
		"Before you write that something is new, now possible or no longer happens, look for it in the removed and context lines.",
		"The previous release may already do it under another name, in another file, through another mechanism, or for fewer cases. Where it did, only what differs for the user is the change.",
	)
}

func TestDefaultPrompt_ChecksEveryStatedBeforeAgainstTheRemovedLines(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		`A bullet that states a before, with words such as "now", "no longer", "instead of", "moved from", "not just" or "any", makes a claim about the previous release: check it against the removed lines.`,
		"Where they show the previous release already behaved the new way, or never had the old one, drop the before and state only what this release adds, or drop the bullet.",
	)
}

func TestDefaultPrompt_TakesNoDocsChangeAsEvidenceOfNewBehaviour(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"A README, docs page, sample output or settings table that changes in this release is no evidence that what it describes changed: docs are often brought up to date with what an earlier release already shipped.",
		"A command, option or setting is new only where the diff adds the code that defines it, where it is declared, parsed or registered, not only code that uses it: a setting is new where the diff adds it to the code that reads the configuration, not where code only uses its value.",
		"A behaviour changed only where the diff changes the code that decides it. Where that code is not in the diff, the previous release already had it: leave the bullet out.",
	)
}

func TestDefaultPrompt_ReadsRewordedDocsAsACorrection(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Docs that now say one thing where they used to say another, in the diff's removed and added lines, were corrected: they were wrong about the previous release, and the software did not change unless the code that decides it changes too.",
	)
}

func TestDefaultPrompt_CountsMovedTextAsWhatThePreviousReleaseHad(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Text that leaves one file and reappears in another, split out, merged or renamed, is moved, not added: the previous release already had it.",
	)
}

func TestDefaultPrompt_ChecksEveryClaimedChangeAgainstAddedCodeBeforeReturning(t *testing.T) {
	t.Parallel()

	assertPromptSays(t,
		"Last check, before you return the notes: for each bullet that says something is new, now works differently or no longer happens, find the added code lines that make it so.",
		"Where you find only README, docs, sample, comment or test lines, or the removed lines show the previous release already did it, drop the bullet or the claim it makes about the previous release.",
	)
	assertOrder(t, notes.DefaultPrompt, "Last check, before you return the notes:", "Output discipline:")
}
