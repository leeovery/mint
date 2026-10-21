AGENT: standards
FINDINGS:
- FINDING: The README never describes the failure classifier, halving, the lower-the-cap hint or `r` on chunked notes, yet it refers to them and still says a failed release call goes straight to on_notes_failure
  SEVERITY: low
  FAILURE: Take an operator whose model accepts less than the cap: an explicit ai_command on a smaller-window model, or a cap raised above the window. Their release narrates `too big for the model, retrying in N parts`, makes one extra AI call for each failed notes call, and after the notes warns `max_prompt_bytes: set higher than this model accepts; set max_prompt_bytes = N, the largest call that succeeded`. The README, the human reference, describes none of this. Its AI Transport section says a release's failures follow on_notes_failure (README.md:293). The unattended paragraph prints suggestions "beside the lower-the-cap hint" (README.md:121), and that hint is defined nowhere. The floor paragraphs mention "chunking exhausted" and "the model rejected even the smallest parts" (README.md:109, :111), which name a ladder the page never lays out. README.md:127 says chunked notes reach the review gate "(`r` included)" but not that `r` there re-runs only the merge over the same part notes. An operator who presses `r` hoping the AI re-reads a part whose notes missed a change gets the same part notes merged again. They find all this out by watching the run or from the extra calls on an API-key bill.
  FILES: README.md:109, README.md:111, README.md:121, README.md:127, README.md:293
  DESCRIPTION: Spec §18 says the README must stay true to what is built and change in the same change as the behaviour it describes. §18.2 has chunking documented with `mint release`. The rules that ride on chunking are built: §9's one classify call per failed notes call (SIZE vs OTHER), §10's halving of a slice up to three times and its exact-byte lower-the-cap hint (once per batch under `--all`), and §15.1's merge-only `r` on chunked notes. None of them is in the README. The transport section's "release follows `on_notes_failure`" now leaves out the step a SIZE answer takes first, so on the release path it is no longer accurate.
  RECOMMENDATION: Add a paragraph next to the chunking paragraph in the README's `release` section. It should cover four points:
    - A failed notes call (single-pass, chunk or merge) whose AI output holds text is classified by one more call through the same command.
    - A SIZE answer on a single-pass or chunk call re-sends that slice as calls of half its size, at most three times. SIZE on the merge counts as a failed merge, and any other answer goes to the floor.
    - When halving wrote the notes, mint then prints `max_prompt_bytes = N`, N being the largest diff-carrying call that succeeded. Under `regenerate --all` it prints once, at the end of the batch.
    - `r` on chunked notes re-runs only the merge, with the nudge.
  Then reword README.md:293 so that a release's failed call reaches `on_notes_failure` only once the classifier and halving have not written the notes.
COMMENT_CORRECTIONS:
- internal/config/config.go:22-24 — the package doc lists Load's value checks and leaves out the malformed-ai_command rejection this feature added
  OLD: // command string nor an array of command strings is rejected naming the hook key;
// max_prompt_bytes must be positive; and on_notes_failure is constrained to the closed
// set abort | fallback.
  NEW: // command string nor an array of command strings is rejected naming the hook key;
// max_prompt_bytes must be positive; an ai_command at any level must split into a
// program and its arguments; and on_notes_failure is constrained to the closed set
// abort | fallback.
- internal/config/config.go:447-449 — translateStrict's list of special-cased keys leaves out release.fallback, which unknownKeyMessage also special-cases
  OLD: // table so the offender is unambiguous — and special-cases a TOP-LEVEL [hooks] table
// (the documented contradiction) with targeted guidance to nest under
// [release.hooks], and the removed top-level max_diff_lines with its replacement.
  NEW: // table so the offender is unambiguous — and special-cases a TOP-LEVEL [hooks] table
// (the documented contradiction) with targeted guidance to nest under
// [release.hooks], and the removed max_diff_lines and release.fallback with what
// replaced them.
SUMMARY: I re-checked the specification and its corrigenda against the code, and every behaviour decision still matches. That covers the lean command, quote-aware splitting and its load errors, the cap and the 180s timeout, the three diff_exclude levels and diff_keep, the exclusion check with its pauses, answers and unwind exception, chunking, classification, halving, the floor and its next moves, --notes-file, the one non-AI body, the narration, commit's verbatim output, and the exact message texts. What drifted is documentation only. The README never describes the classify/halve ladder, the lower-the-cap hint or `r`'s merge-only behaviour on chunked notes, though it refers to them. Two config.go comments miss validations this feature added.
