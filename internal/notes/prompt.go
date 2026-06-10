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
Read the WHOLE diff and produce human-facing release notes in the exact format
described below.

Output structure:

1. A TL;DR at the very top (one line, or a few lines if needed). The TL;DR is a
   unified cross-change narrative synthesised from the WHOLE diff — what this
   release is really about — and sits ABOVE the categorized sections below. It is
   not a list; it is the through-line that ties the changes together.

2. Categorized sections using the Keep a Changelog taxonomy, each with an emoji
   header, in this order:

     ✨ Added       — new features and capabilities
     🔧 Changed     — changes to existing behaviour
     ⚠️ Deprecated  — features marked for future removal
     🗑️ Removed     — removed features
     🐛 Fixed       — bug fixes
     🔒 Security    — security-relevant changes

   Omit empty sections entirely — only emit a section that has at least one item.

Item rules:

- The unit of an entry is the notable item, NOT a file, hunk, or commit. Read the
  diff, extract each notable item, and file it under its category. One change that
  adds a feature AND fixes a bug yields TWO items in two sections.
- Bold the notable features and describe them — celebrate them, do not bury them
  in a flat list.
- Ignore version-number bumps and other trivial bookkeeping churn — they are not
  notable items and must not appear.

Salience discipline:

- rank importance using the Change Map: it tells you which areas are new or large,
  and new structure (a whole new package or directory) is the strongest headline
  signal — weight it above raw size. Let the Change Map decide what leads.
- DESCRIBE every change from the DIFF, never from the Change Map. The Change Map is
  salience metadata, not content: never narrate a file as a feature merely because
  it is large or new. The diff is the source of truth for what each item says.

Deprecated and Security are opportunistic:

- Only emit a Deprecated or Security item on an explicit textual marker in the diff
  — a @deprecated / deprecation annotation for Deprecated; an obvious security
  surface (authentication, cryptography, input validation, or a CVE-referencing
  dependency bump) for Security. Do NOT infer them speculatively. These sections
  are expected to be empty in most releases.

Output discipline:

- Return the notes directly in this presentation format. Do NOT wrap them in any
  machine-parseable labels, headings, or markers beyond the format above.
- Strict: no preamble and no meta-commentary. Do not explain what you are doing, do
  not restate these instructions, do not add a sign-off. Output only the notes.`

// ComposePrompt assembles the final AI input from its three parts, in EXACTLY this
// order and nothing else:
//
//	{instructions} + {Change Map preamble} + {post-exclusion, capped diff}
//
// instructions is the resolved prompt (the default prompt, the default plus an
// injected [release].context, or a full [release].prompt override — see
// ResolveInstructions). changeMap is the diff-derived salience preamble from
// BuildChangeMap. diff is the post-exclusion, max_diff_lines-capped diff from
// AssembleDiff. The function is PURE: it only orders and joins, performing no IO
// and no transport — it produces the AI INPUT, never parses the AI output.
//
// The parts are separated by a blank line so the AI sees three clearly delimited
// blocks; no labels or machine-parseable wrappers are added (the AI returns the
// presentation format directly).
func ComposePrompt(instructions, changeMap, diff string) string {
	return strings.Join([]string{instructions, changeMap, diff}, "\n\n")
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
