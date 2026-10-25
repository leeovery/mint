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
Change Map (a summary of where the release's changes landed) followed by the
actual code diff between the previous release and this one. Read the WHOLE diff
and produce concise release notes in the exact format described below. Release notes are
FOR THE PEOPLE WHO USE THE SOFTWARE: a reader should grasp what changed for them
and why it matters, without reading the diff.

Format — an optional headline, then categorized sections:

1. HEADLINE (optional): one plain sentence, only where the release has one, as
   the first line of the notes, followed by one blank line and then the first
   section. Where the release has no headline, the notes open with the first
   section.
2. SECTIONS, each with an emoji header, in this order. Omit empty sections
   entirely — emit only a section that has at least one item. Keep this order in
   every release, whatever its main work. Removed comes before Fixed, and Added
   before Changed, even where the later section holds the release's main work.

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
main work is the only restatement in the notes. A change and what it brings with
it are one change: a removal and the replacement that takes its place, a fix and
the feature that makes it, a new mechanism and the behaviour users see from it
each get one bullet, in the section where it matters most to the user.

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
- When you write the notes for one part of a release, write no headline, not
  even one for that part: the release's headline is written once, from every
  part's notes.
- When you write the release's notes from the notes of its parts, judge its main
  body of work by the merged bullets, with the Change Map beside them. A change
  whose diff spans several parts can appear in several parts' notes: it counts
  once, as it is listed once, and is no weightier for the repeats. The main work
  may sit in one part's notes alone.

Which section a change belongs in — judge every change against the PREVIOUS
release, never against earlier states inside this one:

- Something the previous release did not have is Added. Its pieces are Added
  too, even where the diff shows them adjusted, capped or fixed along the way.
  A bug in something new in this release is never Fixed, and a tweak to it is
  never Changed.
- A rework or redesign of something the previous release already had is
  Changed, and so is any smaller change to how it works that is not a fix, such
  as a speed-up users notice or a changed default that is not breaking. A
  genuinely new capability is Added, whether it ships inside a rework or is
  added to something the previous release had, such as a new option on an
  existing command.
- Something that behaved wrongly in the previous release and now behaves
  correctly is Fixed, however the diff or its comments frame it. Behaving
  wrongly covers wrong output or placement, a crash or error, dropped or lost
  data, and something not working. That includes a rework whose purpose is the
  fix, and something an earlier release lost that this one brings back.
- Something the previous release had that is gone is Removed. A removal that
  breaks existing use goes under 💥 Breaking only.
- A breaking change goes under 💥 Breaking only, never repeated in another
  section. Put a change there ONLY where the diff shows that something users
  already do stops working, or gives a different result they must act on: a
  removed or renamed command, flag, option, setting or key binding; a default
  changed so that existing setups behave differently in a way that matters; a
  changed output or file format that scripts or tools read; data or
  configuration from the previous release no longer read. A change that only
  adds, keeps more, allows more or improves is not breaking. When unsure
  whether a change is breaking, it is not: put it in its usual section.
- Each 💥 Breaking bullet says, in its one sentence, what breaks and what to do.
  Take what to do only from what the diff shows, such as a new name, a
  replacement, or a setting that keeps the old behaviour. Where the diff shows
  none, the bullet says what breaks and stops there.

Be true to the release:

- Find the changes in the code first. Go through the diff's code and note what
  each change does; only then read its documentation, READMEs, examples,
  comments and tests, and only for the words to describe changes you already
  found in the code. Behaviour you find only there is not a change of this
  release: leave it out. A new guide or docs page users read is itself a
  change users meet; what it describes is not.
- Claim only behaviour the code in this diff changes. Documentation, READMEs,
  comments, examples and tests DESCRIBE behaviour; editing them does not change
  it. When the diff only adds or rewrites documentation for something, the
  software did not change: do not announce it as new, changed or fixed. A change
  to tests alone is not a fix. Where the files ARE the product, such as prompts,
  skills or templates the software ships and uses, changing them changes the
  product.
- Never present work built in this release as a change to, or fix of, the
  previous release. Comments and docs written during the release often describe
  a "before" ("no longer", "now", "instead of"). Check the code: that before may
  be an earlier state inside this release that never shipped.
- Never present as new anything the previous release already had, wherever the
  diff shows it: in unchanged context lines, in removed lines, or in code the
  diff only moves or renames.
- Read the diff as the two releases side by side: its removed lines and its
  unchanged context lines are the previous release, its added lines are this
  one. Before you write that something is new, now possible or no longer
  happens, look for it in the removed and context lines. The previous release
  may already do it under another name, in another file, through another
  mechanism, or for fewer cases. Where it did, only what differs for the user
  is the change.
- A bullet that states a before, with words such as "now", "no longer",
  "instead of", "moved from", "not just" or "any", makes a claim about the
  previous release: check it against the removed lines. Where they show the
  previous release already behaved the new way, or never had the old one, drop
  the before and state only what this release adds, or drop the bullet.
- A README, docs page, sample output or settings table that changes in this
  release is no evidence that what it describes changed: docs are often
  brought up to date with what an earlier release already shipped. A command,
  option or setting is new only where the diff adds the code that defines it,
  where it is declared, parsed or registered, not only code that uses it: a
  setting is new where the diff adds it to the code that reads the
  configuration, not where code only uses its value. A behaviour changed only
  where the diff changes the code that decides it. Where that code is not in
  the diff, the previous release already had it: leave the bullet out.
- Docs that now say one thing where they used to say another, in the diff's
  removed and added lines, were corrected: they were wrong about the previous
  release, and the software did not change unless the code that decides it
  changes too.
- Text that leaves one file and reappears in another, split out, merged or
  renamed, is moved, not added: the previous release already had it.
- Before you keep a bullet, find the code change behind it in the diff. If every
  changed line behind it is a comment, documentation, an example or a test, drop
  the bullet. A new test that asserts a behaviour is evidence the behaviour
  already existed, not that it changed.
- When you write the notes for one part of a release, list only the changes that
  part's diff makes. The Change Map shows where the rest of the release landed,
  and the other parts cover it: a test in that part's diff is no evidence of a
  change made elsewhere.

Keep it to what users meet:

- Include only changes a user of the software meets: what it does, its
  commands, options, configuration, output, errors, performance and
  compatibility.
- Leave out how the software is built and maintained: CI and release tooling,
  linting, build scripts, test suites and harnesses, internal packages, modules
  and refactors, internal flags and plumbing commands, data-migration numbering,
  contributor docs, and repo housekeeping such as restructuring the README. A
  new internal package is not a feature: describe what it lets users do, or
  leave it out.
- The test is whether a user meets or notices the change, not whether the code
  behind it is internal: a speed-up users notice is listed. Where the shipped
  files are the product, such as prompts, skills or templates the software
  ships and uses, changes to those files count.
- Describe every change as users see and use it, never in the code's names, the
  mechanism, or a design document's jargon.
- Where every change in the release is one these rules leave out, the notes are
  ONE plain line saying the release is maintenance with no user-facing changes,
  in the shape of "` + stubBody + `". That line
  has no headline and no sections: it is the one body the format allows without
  sections. Never return empty notes.
- When unsure whether the release holds any change users meet, list the change
  in doubt in its section and do not write the maintenance line.
- When you write the notes for one part of a release and are unsure whether
  that part's diff holds any change users meet, list the change in doubt in its
  section, and write neither the line saying the part holds nothing for the
  notes nor the maintenance line.

Order within each section:

- Within each section, order the changes by what they mean to the user, most
  important first.
- Weight is how much of the product a change touches, how often users meet it,
  and how much it changes their experience. A change to something users do
  every day outranks a change to a diagnostic or rarely used command.
- The pieces of the release's main work lead their sections.
- Lines, files, commits, and how easy a change is to name, are not weight.

The Change Map shows where the changes landed. Use it to make sure nothing large
is missed, not as a measure of importance: a new folder may hold tooling, and the
largest file is often a test or a deletion. DESCRIBE every change from the DIFF;
the Change Map is metadata, never content.

Scope:

- ONE bullet per notable change — not per file, hunk, or commit. Combine
  closely-related changes into a single bullet. Ignore version-number bumps and
  other trivial bookkeeping churn. Most releases are a handful of bullets; do not
  pad.

Deprecated and Security are opportunistic — emit an item ONLY on an explicit
textual marker in the diff (a @deprecated annotation for Deprecated; an obvious
auth / cryptography / input-validation / CVE-bump surface for Security). Do NOT
infer them speculatively. These sections are empty in most releases.

Last check, before you return the notes: for each bullet that says something is
new, now works differently or no longer happens, find the added code lines that
make it so. Where you find only README, docs, sample, comment or test lines, or
the removed lines show the previous release already did it, drop the bullet or
the claim it makes about the previous release.

Output discipline:

- Return the notes directly in this format: the headline line if the release has
  one, then the sections, or the one maintenance line for a release with nothing
  users meet. Do NOT wrap them in any machine-parseable labels, headings, or
  markers beyond the format above.
- Strict: nothing else — no preamble and no meta-commentary. Do not explain what
  you are doing, do not restate these instructions, do not add a sign-off.`

// OutputReminder is the closing line ComposePrompt appends AFTER the diff. It
// restates the output contract at the very END of the prompt because recency wins:
// with the discipline rules buried above a large diff, models drift into narrating
// before the notes (observed in real `mint commit` runs; the notes prompt shares
// the same shape, so it gets the same hardening). It applies under a full
// [release].prompt override too: whatever the instructions, the transport's
// contract is "the body IS the notes", so the reminder is part of the compose,
// not the instructions. So it stays format-neutral: "the format above" defers to
// whichever instructions govern, and it names no rule of the default format, the
// headline included.
const OutputReminder = "Now output ONLY the release notes themselves, in the format above, exactly as they should be published: no preamble, no commentary, no explanation, no code fences, nothing before or after the notes."

// ComposePrompt assembles the final AI input from its parts, in EXACTLY this
// order and nothing else:
//
//	{instructions} + {Change Map preamble} + {post-exclusion diff} + {OutputReminder}
//
// instructions is the resolved prompt (the default prompt, the default plus an
// injected [release].context, or a full [release].prompt override — see
// ResolveInstructions). changeMap is the coverage preamble from BuildChangeMap —
// where the release's changes landed, never a measure of importance. diff is the
// post-exclusion diff from AssembleDiff. The function is PURE: it only orders and
// joins, performing no IO and no transport — it produces the AI INPUT, never
// parses the AI output.
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
