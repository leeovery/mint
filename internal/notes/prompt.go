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
// configured. A notes-format rule belongs here, never in the text mint adds to every
// notes call: a full override replaces this prompt, but that text reaches its calls
// too. It is the "instructions" half of the composed AI input; ComposePrompt appends
// the Change Map preamble and the diff after it.
const DefaultPrompt = `You are writing the release notes for a software release. You are given a
Change Map (a diff-derived salience summary) followed by the actual code diff
between the previous release and this one. Read the WHOLE diff and produce
concise release notes in the exact format described below. Release notes are
FOR THE PEOPLE WHO USE THE SOFTWARE: a reader should grasp what changed for them
and why it matters, without reading the diff.

Format — an optional headline, then categorized sections:

1. HEADLINE (optional): one plain sentence, only where the release has one, as
   the first line of the notes, followed by one blank line and then the first
   section. Where the release has no headline, the notes open with the first
   section.
2. SECTIONS, each with an emoji header, in this order. Omit empty sections
   entirely — emit only a section that has at least one item. Keep this order in
   every release, whatever its main work.

    💥 Breaking    — changes that stop existing use working, or change it in a way users must act on
    ✨ Added       — capabilities the previous release did not have
    🔧 Changed     — changes to behaviour the previous release already had
    ⚠️ Deprecated  — features marked for future removal
    🗑️ Removed     — features the previous release had that are gone
    🐛 Fixed       — things that were broken in the previous release and now work
    🔒 Security    — security-relevant changes

Nothing goes above the first section but the headline: NO summary paragraph,
NO TL;DR, NO label, and NO other preamble.

List every change once, in one section. The headline's summary of the release's
main work is the only restatement in the notes.

Each bullet is ONE line and ONE sentence: name the change in plain language, and
where it helps add the user-facing effect after an em dash. That single line is
the whole bullet — NO second sentence, NO bold lead-in label, NO sub-bullets, NO
implementation detail, and NO rationale for how it was built. Aim for a line a
user understands at a glance, not a fragment they have to decode.

Examples of the right altitude (what to aim for):

    - Single-keypress review gates — press y/n/e/r to decide, no Enter needed.
    - Failures no longer print twice when stdout and stderr share a terminal.
    - Pin the AI model with ` + "`ai_command = \"claude -p --model sonnet\"`" + `.

The headline:

- Write a headline only when the release has a MAIN BODY OF WORK: one feature,
  rework or fix that anyone asked "what is this release?" would name. Test it
  against your own bullets: either most of the notable changes serve it, or it
  alone clearly outweighs all the other changes put together. The largest of
  several separate changes is NOT a main body of work.
- Two bodies of work of EQUAL weight are named together, in the one sentence and
  led by the benefit, only where EACH passes the test once the other is set
  aside.
- Name a second subject only when it is a co-equal body of work: never two
  pieces of one body, and never a main body plus a lesser piece, whether "and"
  or anything else joins them.
- A headline names at most two bodies of work, in one sentence, even where a
  release has more than two co-equal bodies.
- Leave the headline out where one bullet already states the whole release and
  nothing else stands beside it: there the headline would only repeat that
  bullet. Judge that by whether a single bullet states the whole release, not by
  how many bullets there are.
- Keep the headline where the main work is spread over several bullets and none
  of them names the whole, even when the notes hold nothing else: it is the one
  line that says what the release is. Keep it too for a main feature written as
  a single bullet beside unrelated changes: there is a rest to set it apart from.
- Write NO headline on a grab-bag of small unrelated changes, however large the
  largest of them, and stitch no theme across such changes.
- When in doubt, leave it out: write a headline only when one body of work, or
  two co-equal bodies, clearly stands out, and otherwise start the notes with the
  first section. A missing headline costs little; a wrong one misstates the
  release in its most-read line.
- The headline sums up the release's main body of work in ONE terse sentence, in
  plain language, at the same altitude as the bullets. It names that body of work
  and leads with what it gives the user: what they can now do, or no longer
  suffer, in the terms they meet it — never in files, formats or internals.
- It is NOT a synthesis of everything in the release, and NOT a narrative thread
  that ties the changes together.
- Lead with the benefit, not the size of the work: no "major", "overhaul",
  "powerful" or other scale or praise words, and no inventory of the work's
  pieces. The sections carry the scale.
- The main body of work may be a feature, a rework, a fix or a large epic. Name it
  whatever section its pieces sit in.
- Plain text: no bold, no emoji, and no label such as "Highlights:" or "TL;DR:".
- The headline does not relieve the sections: list every notable change below it
  exactly as you would without one, the pieces of the main work included.
- Pick the headline by what the release changes for its user, read from the diff.
  A new folder or a large file is not, of itself, the release's main thing, and
  the Change Map does not pick the headline.

Scope and salience:

- ONE bullet per notable change — not per file, hunk, or commit. Combine
  closely-related changes into a single bullet. Ignore version-number bumps and
  other trivial bookkeeping churn. Most releases are a handful of bullets; do not
  pad.
- Rank importance using the Change Map: new structure (a whole new package or
  directory) is the strongest signal — weight it above raw size. But DESCRIBE
  every change from the DIFF; the Change Map is salience metadata, never content.

Deprecated and Security are opportunistic — emit an item ONLY on an explicit
textual marker in the diff (a @deprecated annotation for Deprecated; an obvious
auth / cryptography / input-validation / CVE-bump surface for Security). Do NOT
infer them speculatively. These sections are empty in most releases.

Output discipline:

- Return the notes directly in this format: the headline line if the release has
  one, then the sections. Do NOT wrap them in any machine-parseable labels,
  headings, or markers beyond the format above.
- Strict: nothing else — no preamble and no meta-commentary. Do not explain what
  you are doing, do not restate these instructions, do not add a sign-off.`

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
//	{instructions} + {Change Map preamble} + {post-exclusion diff} + {OutputReminder}
//
// instructions is the resolved prompt (the default prompt, the default plus an
// injected [release].context, or a full [release].prompt override — see
// ResolveInstructions). changeMap is the diff-derived salience preamble from
// BuildChangeMap. diff is the post-exclusion diff from AssembleDiff. The function
// is PURE: it only orders and joins, performing no IO and no transport — it
// produces the AI INPUT, never parses the AI output.
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
