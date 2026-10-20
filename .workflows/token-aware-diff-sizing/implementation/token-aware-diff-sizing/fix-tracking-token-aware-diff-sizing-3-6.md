## Attempt 1

ISSUES:
- /Users/leeovery/Code/mint/internal/setupguide/setupguide.go:171-172 — the new text "(abort loud, or fall back to the commit-subject list, the one non-AI body, which --no-ai ships too)" adds a CLI flag to a guide that §18.1 keeps flag-free, and that says so itself at setupguide.go:123-124. An agent following the guide is pointed at a flag the guide says it never uses. The edit also broke the paragraph's wrap: line 172 now prints at about 126 columns in `mint setup` output, while the paragraph around it wraps at about 84.
  FIX: Drop ", which --no-ai ships too" so the text reads "(abort loud, or fall back to the commit-subject list, the one non-AI body) and commit drops to the user's $EDITOR." Then rewrap the last three lines of the literal to the paragraph's ~84-column width. `TestGuide_AITransportParagraphNamesTheOneNonAIBody` (setupguide_test.go:806) still passes. Optionally add `strings.Contains(flat, "--no-ai")` to its negative checks.
  CONFIDENCE: high
- /Users/leeovery/Code/mint/internal/notes/noai.go:30 — the task removed the only read of `rel` (the old `rel.Fallback`), so `NoAIBody(ctx, r, lastTag, rel config.Release)` now takes a parameter it ignores. `select.go:178` still passes `cfg.Release` in, and the doc at noai.go:24-25 still talks about `rel.OnNotesFailure`. A reader, or the Phase 5 and Phase 8 work that extends this list, will take the signature to mean the `--no-ai` body depends on release config, which is the coupling this task removed. Nothing catches it: unparam is not enabled and no test covers it. The executor kept the parameter only to avoid touching callers.
  FIX: Change the signature to `NoAIBody(ctx context.Context, r runner.CommandRunner, lastTag string)` and drop the `config` import from noai.go. Update `internal/notes/select.go:178` to `NoAIBody(ctx, s.runner, state.LastTag)`. Update the noai_test.go call sites (lines 23, 49, 65, the shared-builder case loop, and the new route table), and drop the `rel` field from that case table where it only fed `NoAIBody`. Rewrite noai.go:24-25 to "on_notes_failure is deliberately NOT consulted: it governs only the normal AI path." `TestNoAIBody_NeverAbortsEvenWhenAIWouldHaveFailed` then asserts body plus nil without a `rel`.
  ALTERNATIVE: Keep the parameter as a deliberate extension point. Nothing in the plan needs it: Phase 5 matches the bookkeeping commit by its " Release {tag}" suffix whatever the prefix, and Phase 8 keys on the answers-commit subject, so I recommend removing it.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- /Users/leeovery/Code/mint/internal/notes/resolve.go:44-47 — the diff touched this bullet. It still claims validation is a future phase, but `validateOnNotesFailure` already rejects these values at load (config.go:409). It also uses phase vocabulary, and its new last sentence is history.
  OLD: //   - any OTHER value → treated as ABORT for Phase 2. Phase 6's typed validation will
//     reject unknown on_notes_failure values up front; until then this resolver
//     defaults defensively to abort rather than tagging on a misconfigured value. No
//     value is ever taken as a literal body.
  NEW: //   - any OTHER value → ABORT. Load already rejects such values; aborting keeps an
//     unvalidated Release from tagging on a misconfigured value.
- /Users/leeovery/Code/mint/internal/notes/resolve.go:20-22 — lists the consumers, which Phase 5's floor `c` will make untrue.
  OLD: // scope for the AI input by design. FallbackBody exists purely to give the non-AI
// routes (on_notes_failure=fallback and --no-ai, both through resolveFallbackBody) an
// honest, non-hallucinated record when the AI body is unavailable.
  NEW: // scope for the AI input by design. FallbackBody exists purely to give the non-AI
// routes an honest, non-hallucinated record when the AI body is unavailable.
- /Users/leeovery/Code/mint/internal/notes/resolve.go:74-76 — "both non-AI routes … the two" is a count claim; §14.1 defines three routes, the floor's `c` among them.
  OLD: // resolveFallbackBody is the SHARED non-AI body selector both non-AI routes call —
// on_notes_failure=fallback (ResolveFailure) and --no-ai (NoAIBody) — so the two can
// never drift. It produces a NON-EMPTY body:
  NEW: // resolveFallbackBody is the SHARED non-AI body selector every non-AI route calls,
// so they can never drift. It produces a NON-EMPTY body:
- /Users/leeovery/Code/mint/internal/config/config.go:175-177 — a field doc should not carry the history of a removed key; the load rejection lives in unknownKeyMessage.
  OLD: // MODE-ONLY ("abort" → abort; "fallback" → the commit-subject list, the one non-AI
// body --no-ai also ships). There is no fixed-body key: [release].fallback was removed
// and is rejected at load naming --notes-file.
  NEW: // MODE-ONLY ("abort" → abort; "fallback" → the commit-subject list, the one non-AI
// body --no-ai also ships).
- /Users/leeovery/Code/mint/internal/config/config.go:536-537 — repeats the message string the next lines return (the sibling `max_diff_lines` branch has no comment).
  OLD: 	// [release].fallback, the fixed non-AI body, is gone: every non-AI route ships the
	// commit-subject list, and hand-written notes come through --notes-file.
  NEW: 
- /Users/leeovery/Code/mint/internal/notes/resolve_test.go:146-150 — the diff touched this comment. It still uses Phase 2/Phase 6 vocabulary for validation that already exists, and the added last sentence is history.
  OLD: 	// on_notes_failure is MODE-ONLY (abort | fallback). Any value that is not
	// "fallback" — including an unknown string — resolves to ABORT for Phase 2: it
	// returns NO body and an abort error naming the cause, and runs NO git. (Phase 6's
	// typed validation will reject unknown values up front; this resolver just treats
	// them as abort defensively.) No value is ever taken as a literal body.
  NEW: 	// on_notes_failure is MODE-ONLY (abort | fallback). Any value that is not
	// "fallback" — including an unknown string — resolves to ABORT: it returns NO body
	// and an abort error naming the cause, and runs NO git.

NOTES:
- Older comments next to this change, which the diff did not touch, still carry stale wording (out of this task's scope):
  - resolve.go:70 says "(Phase-6-rejected)".
  - noai.go:27-29 says "(a later task)… separate tasks".
  - resolve_test.go:33 and :121 say "DEFAULT fallback body", which implied a non-default fixed body that no longer exists.
- /Users/leeovery/Code/mint/internal/engine/release_malformed_aicommand_test.go:15 is now an unwrapped comment line of about 140 columns. It is cosmetic and its content is accurate.
- The README `--no-ai` row (README.md:99) and the usage line (cmd/mint/usage.go:40), "use the commit-subject fallback body", name no fixed string. Leaving them is correct.
- When Phase 5 builds the floor's `c`, it should call `resolveFallbackBody` rather than build its own `git log` and stub floor; otherwise the mismatch §14.1 removes comes back.
- The dispatch carried only the enumerated inputs.
