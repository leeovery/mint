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
		{"changelogs are for humans", "FOR HUMANS"},
		{"no tl;dr", "NO TL;DR"},
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
	if !strings.Contains(got, "Keep a Changelog") || !strings.Contains(got, "✨ Added") {
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
