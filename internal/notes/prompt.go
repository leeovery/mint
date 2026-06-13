package notes

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mint/internal/config"
)

// DefaultPrompt is the instructions mint ships when no [release].prompt override is
// configured. It encodes the default notes format — Keep a Changelog's taxonomy and
// principles rendered in mint's emoji skin — plus the salience and discipline rules
// the spec pins (Stage 4: Default notes format / Prompt control / Output format).
// It is the "instructions" half of the composed AI input; ComposePrompt appends the
// Change Map preamble and the diff after it. Wording is deliberately ship-and-refine
// (tuned over real releases); every rule below is required and individually
// assertable in the tests.
const DefaultPrompt = `You are writing the release notes for a software release. You are given a
Change Map (a diff-derived salience summary) followed by the actual code diff.
Read the WHOLE diff and produce concise, human-facing release notes in the exact
format described below. Changelogs are FOR HUMANS: a reader should grasp what
changed and why it matters to them, without reading the diff.

Format — categorized sections using the Keep a Changelog taxonomy, each with an
emoji header, in this order. Omit empty sections entirely — emit only a section
that has at least one item:

    ✨ Added       — new features and capabilities
    🔧 Changed     — changes to existing behaviour
    ⚠️ Deprecated  — features marked for future removal
    🗑️ Removed     — removed features
    🐛 Fixed       — bug fixes
    🔒 Security    — security-relevant changes

Each bullet is ONE line and ONE sentence: name the change in plain language, and
where it helps add the user-facing effect after an em dash. That single line is
the whole bullet — NO second sentence, NO bold lead-in label, NO sub-bullets, NO
implementation detail, and NO rationale for how it was built. Aim for a line a
user understands at a glance, not a fragment they have to decode.

Examples of the right altitude (what to aim for):

    - Single-keypress review gates — press y/n/e/r to decide, no Enter needed.
    - Failures no longer print twice when stdout and stderr share a terminal.
    - Pin the AI model with ` + "`ai_command = \"claude -p --model sonnet\"`" + `.

Scope and salience:

- ONE bullet per notable change — not per file, hunk, or commit. Combine
  closely-related changes into a single bullet. Ignore version-number bumps and
  other trivial bookkeeping churn. Most releases are a handful of bullets; do not
  pad.
- NO TL;DR, NO summary paragraph, NO preamble — start directly with the first
  section header.
- Rank importance using the Change Map: new structure (a whole new package or
  directory) is the strongest signal — weight it above raw size. But DESCRIBE
  every change from the DIFF; the Change Map is salience metadata, never content.

Deprecated and Security are opportunistic — emit an item ONLY on an explicit
textual marker in the diff (a @deprecated annotation for Deprecated; an obvious
auth / cryptography / input-validation / CVE-bump surface for Security). Do NOT
infer them speculatively. These sections are empty in most releases.

Output discipline:

- Return the notes directly in this format. Do NOT wrap them in any
  machine-parseable labels, headings, or markers beyond the format above.
- Strict: no preamble and no meta-commentary. Do not explain what you are doing, do
  not restate these instructions, do not add a sign-off. Output only the notes.`

// OutputReminder is the closing line ComposePrompt appends AFTER the diff. It
// restates the output contract at the very END of the prompt because recency wins:
// with the discipline rules buried above a large diff, models drift into narrating
// before the notes (observed in real `mint commit` runs; the notes prompt shares
// the same shape, so it gets the same hardening). It applies under a full
// [release].prompt override too: whatever the instructions, the transport's
// contract is "the body IS the notes", so the reminder is part of the compose,
// not the instructions.
const OutputReminder = "Now output ONLY the release notes themselves, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes."

// ComposePrompt assembles the final AI input from its parts, in EXACTLY this
// order and nothing else:
//
//	{instructions} + {Change Map preamble} + {post-exclusion, capped diff} + {OutputReminder}
//
// instructions is the resolved prompt (the default prompt, the default plus an
// injected [release].context, or a full [release].prompt override — see
// ResolveInstructions). changeMap is the diff-derived salience preamble from
// BuildChangeMap. diff is the post-exclusion, max_diff_lines-capped diff from
// AssembleDiff. The function is PURE: it only orders and joins, performing no IO
// and no transport — it produces the AI INPUT, never parses the AI output.
//
// The parts are separated by a blank line so the AI sees clearly delimited
// blocks; no labels or machine-parseable wrappers are added (the AI returns the
// presentation format directly).
func ComposePrompt(instructions, changeMap, diff string) string {
	return strings.Join([]string{instructions, changeMap, diff, OutputReminder}, "\n\n")
}

// contextHeader labels the injected [release].context block appended to the
// default prompt so the AI reads it as supplementary project guidance, not as part
// of the diff.
const contextHeader = "Additional project context (apply the rules above with this in mind):"

// ResolveInstructions produces the "instructions" half of the AI input from
// config's two prompt-control knobs, reading files relative to root. This is the
// IO side of prompt assembly (it may read files); ComposePrompt is the pure side.
//
// Resolution, in precedence order:
//
//   - rel.Prompt set → it is a FILE PATH. Its contents fully OVERRIDE the default
//     prompt (the default is replaced entirely). Context is ignored in this case —
//     context injects into the default prompt, and a full override has no default to
//     inject into. A missing prompt file is an error, not a silent fall-back: a
//     configured prompt is an explicit operator choice.
//   - else → the DEFAULT prompt, with rel.Context (if set) INJECTED (appended, not
//     replacing). Context is "string OR file": if its value names an existing file
//     (resolved relative to root) the file's CONTENTS are injected; otherwise the
//     value is treated as a literal string.
//   - neither set → the default prompt verbatim.
func ResolveInstructions(root string, rel config.Release) (string, error) {
	if rel.Prompt != "" {
		return readPromptOverride(root, rel.Prompt)
	}
	if rel.Context == "" {
		return DefaultPrompt, nil
	}

	context, err := resolveContext(root, rel.Context)
	if err != nil {
		return "", err
	}
	return injectContext(DefaultPrompt, context), nil
}

// readPromptOverride reads the full-override prompt file at path (relative to root)
// and returns its contents verbatim as the instructions. A read failure (missing
// file or otherwise) is surfaced — a configured prompt must exist.
func readPromptOverride(root, path string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return "", fmt.Errorf("reading release prompt override %q: %w", path, err)
	}
	return string(data), nil
}

// resolveContext implements the string-or-file detection for [release].context: if
// value names a file that exists under root, its contents are returned; otherwise
// value is returned as the literal context string. A stat error other than
// not-exist (e.g. a permission problem) is surfaced rather than silently treated as
// a literal string, so a genuine IO fault is not masked.
func resolveContext(root, value string) (string, error) {
	path := filepath.Join(root, value)
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return value, nil // not a file — literal string.
	case err != nil:
		return "", fmt.Errorf("resolving release context %q: %w", value, err)
	case info.IsDir():
		return value, nil // a directory is not a context file — literal string.
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading release context file %q: %w", value, err)
	}
	return string(data), nil
}

// injectContext appends the resolved context to the default prompt under a labelled
// header, injecting (never replacing) so every default rule survives alongside the
// project-specific guidance.
func injectContext(prompt, context string) string {
	return prompt + "\n\n" + contextHeader + "\n\n" + context
}
