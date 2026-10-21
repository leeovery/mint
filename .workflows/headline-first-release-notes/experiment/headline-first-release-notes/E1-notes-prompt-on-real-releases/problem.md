# Problem

mint writes a release's notes with one AI call (or, for a diff too large for one call, chunk calls and a merge) from its default notes prompt. The notes ship byte-identical as the annotated tag's body, a `CHANGELOG.md` section and the provider release body. On real releases they often fail to portray what the release did.

An audit of 31 AI-written releases across nine projects on this machine — every one on the default prompt, all since the 2026-06-13 format — compared each release's notes with what the release actually was, judged from full history: `.workflows/headline-first-release-notes/imports/release-notes-audit.md`. It found the main work scattered over many bullets and sections with no line stating the whole; work that reshapes existing behaviour buried under the fixed section order; inversions within sections; false or stale claims drawn from documentation, comment or test edits; work built in the release presented as a change or fix to the previous one; internal and dev-only changes presented to users; misfiled sections; buried breaking changes; and Change Map salience signals that barely work.

The discussion has decided part of the fix — a one-sentence headline, with rules for its shape, when it appears and how it is picked (Headline Shape, No Clear Headline, Picking The Headline in `.workflows/headline-first-release-notes/discussion/headline-first-release-notes.md`) — and has taken the ordering and accuracy failures into the same feature (Scope, same file). The user's bar for the result: notes that are good, readable, coherent, useful, accurate and well ordered.

What we need to learn: whether a rewritten notes prompt can produce notes that meet that bar on these real releases, and which of the audited failures a prompt cannot fix on its own because the AI lacks something mint does not hand it today.

The space around it:

- The audited releases and the auditors' account of each one's ground truth and failures.
- What the AI sees today, and nothing else: the instructions (the default prompt, `DefaultPrompt` in `internal/notes/prompt.go`, with any `[release].context`), the Change Map (`internal/notes/changemap.go`), the post-exclusion diff, and the closing `OutputReminder`; on a chunked release, the chunk and merge framing (`internal/notes/chunk.go`).
- Fixed ground: bodies ship byte-identical and mint never edits them; the diff is the sole source of truth for what changed (commit history was ruled out as an input in the mint-release-tool work); every call fits under `max_prompt_bytes`.

What we hope: a prompt the specification can take as tested rather than guessed, and a clear answer on which failures need mint to change what it hands the AI.

Spawned from the "headline-first-release-notes" discussion, at the audit of 31 released notes, on 2026-10-21.
