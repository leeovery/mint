package commit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/commit"
	"mint/internal/config"
)

// writeFile writes body to {dir}/{name} (creating parent dirs) and returns name,
// failing the test on error. Used to stage prompt-override files under t.TempDir().
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
// commit's composed AI input: instructions, then the staged diff — nothing
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

// configWithCommit returns a Config carrying the given [commit] knobs, leaving every
// other field at its zero value (commit's composer only reads cfg.Commit).
func configWithCommit(c config.Commit) config.Config {
	return config.Config{Commit: c}
}

func TestComposePrompt_OrdersInstructionsThenDiff(t *testing.T) {
	t.Parallel()

	// The composed AI input is EXACTLY instructions + staged diff, in that order,
	// nothing else. Distinct sentinels stand in for each part so the assertion is on
	// ORDERING, not content.
	instructions := "INSTRUCTIONS_SENTINEL"
	diff := "DIFF_SENTINEL"

	got := commit.ComposePrompt(instructions, diff)

	assertOrder(t, got, instructions, diff)
}

func TestComposePrompt_ContainsOnlyTheTwoParts(t *testing.T) {
	t.Parallel()

	// "Nothing else": both parts survive composition and the result is built only
	// from the two inputs (plus the join separator), so the two pieces collectively
	// account for all the non-whitespace content.
	instructions := "AAA instructions AAA"
	diff := "BBB diff BBB"

	got := commit.ComposePrompt(instructions, diff)

	if !strings.Contains(got, instructions) {
		t.Errorf("composed prompt missing instructions:\n%s", got)
	}
	if !strings.Contains(got, diff) {
		t.Errorf("composed prompt missing diff:\n%s", got)
	}

	// Stripping the two parts and whitespace leaves only separator characters — no
	// smuggled extra content beyond the two inputs.
	residue := got
	for _, part := range []string{instructions, diff} {
		residue = strings.Replace(residue, part, "", 1)
	}
	if strings.TrimSpace(residue) != "" {
		t.Errorf("composed prompt carries content beyond the two parts: %q", strings.TrimSpace(residue))
	}
}

// defaultPromptRules enumerates the rules the default commit prompt MUST carry,
// each as a distinct substring the prompt is required to contain. Each entry pins
// one spec requirement so a regression that drops a rule fails a named subtest.
func defaultPromptRules() []struct{ name, want string } {
	return []struct{ name, want string }{
		{"conventional commits standard", "Conventional Commits"},
		{"type: description subject shape", "type: description"},
		{"imperative subject", "imperative"},
		{"concise subject", "concise"},
		{"optional wrapped body for the why", "body"},
		{"infer the type from the diff", "infer"},
		{"scope omitted by default", "scope"},
		{"no preamble", "no preamble"},
		{"no meta-commentary", "no meta-commentary"},
	}
}

func TestDefaultPrompt_CarriesEveryRule(t *testing.T) {
	t.Parallel()

	for _, rule := range defaultPromptRules() {
		t.Run(rule.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(commit.DefaultPrompt, rule.want) {
				t.Errorf("default prompt missing rule %q (substring %q):\n%s", rule.name, rule.want, commit.DefaultPrompt)
			}
		})
	}
}

func TestDefaultPrompt_RequestsConventionalCommitSubjectWithOptionalBody(t *testing.T) {
	t.Parallel()

	// The default format is a Conventional Commits `type: description` subject —
	// imperative and concise — with an optional wrapped body for the why.
	for _, want := range []string{"Conventional Commits", "type: description", "imperative", "body"} {
		if !strings.Contains(commit.DefaultPrompt, want) {
			t.Errorf("default prompt does not request the conventional-commit subject/body shape (missing %q):\n%s", want, commit.DefaultPrompt)
		}
	}
}

func TestDefaultPrompt_InstructsInferTypeFromDiff(t *testing.T) {
	t.Parallel()

	// The AI must infer the type (feat/fix/chore/docs/…) from the diff.
	if !strings.Contains(commit.DefaultPrompt, "infer") {
		t.Errorf("default prompt does not instruct the AI to infer the type:\n%s", commit.DefaultPrompt)
	}
	if !strings.Contains(commit.DefaultPrompt, "type") {
		t.Errorf("default prompt does not mention the type to infer:\n%s", commit.DefaultPrompt)
	}
}

func TestDefaultPrompt_InstructsScopeOmittedByDefault(t *testing.T) {
	t.Parallel()

	// Scope is off by default — the prompt must say to omit it and not to guess it.
	if !strings.Contains(commit.DefaultPrompt, "scope") {
		t.Errorf("default prompt does not address scope:\n%s", commit.DefaultPrompt)
	}
	if !strings.Contains(commit.DefaultPrompt, "Omit the scope") {
		t.Errorf("default prompt does not instruct omitting the scope:\n%s", commit.DefaultPrompt)
	}
}

func TestDefaultPrompt_ForbidsCommitPrefixAndBranding(t *testing.T) {
	t.Parallel()

	// No mint branding / commit_prefix and no preamble or meta-commentary in the
	// message text — a plain conventional-commit message is emitted directly.
	for _, want := range []string{"branding", "no preamble", "no meta-commentary"} {
		if !strings.Contains(commit.DefaultPrompt, want) {
			t.Errorf("default prompt does not forbid branding/preamble (missing %q):\n%s", want, commit.DefaultPrompt)
		}
	}
}

func TestDefaultPrompt_RequestsNoMachineParseableWrapper(t *testing.T) {
	t.Parallel()

	// The AI returns the commit message DIRECTLY — no machine-parseable wrapper
	// labels. The prompt must say so explicitly so no wrapper is requested.
	if !strings.Contains(commit.DefaultPrompt, "Return the commit message directly") {
		t.Errorf("default prompt does not request the message returned directly (no machine wrapper):\n%s", commit.DefaultPrompt)
	}
}

func TestResolveInstructions_NoContextNoPrompt_ReturnsDefaultVerbatim(t *testing.T) {
	t.Parallel()

	// With neither knob set, the instructions are exactly the default prompt — the
	// baseline mint ships.
	got, err := commit.ResolveInstructions(configWithCommit(config.Commit{}), t.TempDir())
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if got != commit.DefaultPrompt {
		t.Errorf("instructions = %q, want the default prompt verbatim", got)
	}
}

func TestResolveInstructions_Context_InjectedNotReplacing(t *testing.T) {
	t.Parallel()

	// A [commit].context is injected into (not replacing) the default prompt: the
	// context text appears AND the default rules still survive.
	ctx := "Conventional Commits; dev-workflow toolkit."
	got, err := commit.ResolveInstructions(configWithCommit(config.Commit{Context: ctx}), t.TempDir())
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if !strings.Contains(got, ctx) {
		t.Errorf("instructions missing injected context string:\n%s", got)
	}
	// Default rules must still be present — inject, never replace.
	if !strings.Contains(got, "Conventional Commits") || !strings.Contains(got, "infer") {
		t.Errorf("default prompt rules absent after context inject:\n%s", got)
	}
}

func TestResolveInstructions_AbsentContext_LeavesDefaultUnchanged(t *testing.T) {
	t.Parallel()

	// Absent context = default prompt unchanged (byte-identical to the default).
	got, err := commit.ResolveInstructions(configWithCommit(config.Commit{Context: ""}), t.TempDir())
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if got != commit.DefaultPrompt {
		t.Errorf("absent context changed the default prompt:\n%s", got)
	}
}

func TestResolveInstructions_PromptFile_FullyOverridesDefault(t *testing.T) {
	t.Parallel()

	// A [commit].prompt is a file path whose CONTENTS fully replace the default
	// prompt: the override text is the instructions, and the default prompt's rules
	// are ABSENT (mint still appends the diff downstream via ComposePrompt).
	dir := t.TempDir()
	override := "Write a single line summarising the change. Nothing else."
	name := writeFile(t, dir, ".mint/commit-prompt.md", override)

	got, err := commit.ResolveInstructions(configWithCommit(config.Commit{Prompt: name}), dir)
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	if got != override {
		t.Errorf("instructions = %q, want the override file contents verbatim", got)
	}
	if strings.Contains(got, "Conventional Commits") {
		t.Errorf("default prompt rules leaked into a full override:\n%s", got)
	}
}

func TestResolveInstructions_PromptOverride_StillFlowsThroughComposeWithDiff(t *testing.T) {
	t.Parallel()

	// End-to-end of the override contract: the prompt file fully overrides the
	// instructions, but ComposePrompt still appends the diff after it — in the same
	// trailing position, never dropped or reordered.
	dir := t.TempDir()
	override := "OVERRIDE_INSTRUCTIONS"
	name := writeFile(t, dir, "commit-prompt.txt", override)

	instructions, err := commit.ResolveInstructions(configWithCommit(config.Commit{Prompt: name}), dir)
	if err != nil {
		t.Fatalf("ResolveInstructions returned unexpected error: %v", err)
	}

	diff := "THE_DIFF"
	composed := commit.ComposePrompt(instructions, diff)

	assertOrder(t, composed, override, diff)
	if strings.Contains(composed, "Conventional Commits") {
		t.Errorf("default prompt rules present despite full override:\n%s", composed)
	}
}

func TestResolveInstructions_PromptTakesPrecedenceOverContext(t *testing.T) {
	t.Parallel()

	// When both knobs are set, prompt (full override) wins — context is a default-
	// prompt inject knob and the default prompt is gone under a full override.
	dir := t.TempDir()
	override := "FULL OVERRIDE BODY"
	promptName := writeFile(t, dir, "commit-prompt.txt", override)

	got, err := commit.ResolveInstructions(configWithCommit(config.Commit{
		Prompt:  promptName,
		Context: "this context must be ignored",
	}), dir)
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

func TestResolveInstructions_UnreadablePromptOverride_FailsLoudNoFallback(t *testing.T) {
	t.Parallel()

	// A configured [commit].prompt is an explicit operator choice; an unreadable
	// (here: missing) override file is a real error, NOT a silent fall-back to the
	// default prompt.
	got, err := commit.ResolveInstructions(configWithCommit(config.Commit{Prompt: "does/not/exist.md"}), t.TempDir())
	if err == nil {
		t.Fatal("ResolveInstructions returned nil error for an unreadable prompt override, want non-nil")
	}
	if got == commit.DefaultPrompt {
		t.Errorf("ResolveInstructions fell back to the default prompt on an unreadable override:\n%s", got)
	}
}
