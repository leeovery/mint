TASK: Commit's per-call cap (token-aware-diff-sizing-2-2, tick-ed4e6c)

ACCEPTANCE CRITERIA:
- Given no `max_prompt_bytes` set and a would-be-committed diff of more than 50,000 lines whose composed prompt is at most 1,500,000 bytes, `mint commit` makes the AI call and shows the generated message at the review gate (§4.1, §4.3)
- Given `max_prompt_bytes` set to exactly the composed prompt's size, the AI call is made. Set one byte lower, no AI call is made and the editor fallback applies (§4.1, §4.3)
- Given an attended `mint commit` whose composed prompt is over the cap, no AI call is made. The run shows `diff too large to summarise — opening editor`, then opens the editor on an empty buffer, and the save becomes the accept as built (§4.3)
- Given a diff that alone is under the cap but whose composed prompt goes over it, for example through an outsized `[commit].context`, no AI call is made and the editor opens with the same note (§4.1, §4.3)
- Given an over-cap prompt on an unattended run (`-y`, or a non-TTY stdin), no AI call is made. The run fails with `no AI message and no interactive editor available`, with no note before it and nothing beneath it (§4.3, §17.1)
- Given a message at the review gate whose first prompt fit the cap, when `r` with a nudge pushes the regeneration's composed prompt over the cap, no regeneration call is made. The run shows `diff too large to summarise — opening editor` and opens the editor empty, not on the prior message, as it does after a failed regeneration; the save becomes the accept (§4.3, Corrigendum 2026-10-19 on §4.3)
- The README's commit editor-fallback line names a composed prompt over `max_prompt_bytes` where it named a diff over `max_diff_lines` (§18)

STATUS: issues_found

SPEC CONTEXT: §4.1 makes `max_prompt_bytes` (default 1,500,000, shared top level only) the bound on the whole composed prompt of every call; no call is ever bigger. §4.3 gives commit no chunking: a composed prompt over the cap opens `$EDITOR` with the existing `diff too large to summarise — opening editor` note, and the 2026-10-19 corrigendum extends this to `r` at the review gate (no call sent, editor opens empty, as after a failed regeneration). §17.1 keeps the oversized trigger's lines unchanged and carrying no AI output. §18.2/§18.3 require the README and the code comments describing the line-count guard to change in the same change.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/commit/generate.go:148-152 — the prompt is composed (instructions with `[commit].context` and any one-time nudge, post-exclusion diff, closing reminder), then measured by `checkPromptSize` before the transport call; the old `notes.CheckDiffSize(diff, cfg.MaxDiffLines)` call is gone
  - internal/commit/generate.go:164-169 — `checkPromptSize`: inclusive cap (`len(prompt) > maxBytes`), wraps `notes.ErrDiffTooLarge`
  - internal/commit/generate.go:125-159 — `GenerateWithContext` is the single path for both the first generation (`Generate` delegates with "") and `r`, so the nudge is inside the measured prompt
  - internal/commit/run.go:357-362 — first-generation over-cap: oversized note only when the editor will open (`editorUnavailable`), then `runEditorFallback(..., "")`, so the unattended fail-loud carries no Output
  - internal/commit/run.go:681-684 — regeneration over-cap: oversized Warn, then `errRegenerateFallback`, which Run intercepts at run.go:412-414 and routes to the empty editor fallback
  - internal/config/config.go:685-689 — the absent key resolves to `DefaultMaxPromptBytes` (1,500,000); config.go:834 rejects values of zero or less
  - README.md:199 — the editor-fallback line now names "a composed prompt over `max_prompt_bytes`"
- Notes: Matches the plan's Do items exactly: the byte measure replaces the line guard in the generator, a first-generation over-cap takes the existing oversized branch in `Run`, and a regeneration over-cap leaves `reviewLoop` through the failed-regeneration route. The regeneration Warn is not gated on `editorUnavailable`. That is correct because `r` is reachable only on an attended run (under -y the presenter auto-accepts, and on a non-TTY `Prompt` returns ErrNotInteractive). The all-excluded check still runs before the size guard (generate.go:138-140), unchanged. The comments describing the line-count guard in generate.go, run.go and prompt.go were rewritten. One adjacent comment the new branch falsifies remains (see FINDINGS).

TESTS:
- Status: Adequate
- Coverage:
  - AC1: internal/commit/run_oversized_test.go:442 `TestRun_Oversized_ManyLinesUnderDefaultCap_ShowsAIMessage`. It uses 50,001 lines with no `.mint.toml`, so the default cap applies, asserts one transport call, and asserts ShowMessage carries the AI body. A unit-level counterpart is at internal/commit/generate_test.go:395.
  - AC2: run_oversized_test.go:206 (cap == composed size → one AI call, no Warn, the commit carries the AI body) and run_oversized_test.go:112 / :239 (cap one byte lower → zero calls, editor fallback). Generator counterparts are at generate_test.go:300 and :325; the latter also asserts the sent prompt is exactly the cap's length.
  - AC3: run_oversized_test.go:112 (no call, one launch, save committed), :141 (exact note), :358 (empty buffer).
  - AC4: run_oversized_test.go:480. The cap is composedSize(diff)+100 with a 200-byte `[commit].context`. The test asserts zero calls, the exact `{Label: "diff", Message: note}` Warn, an empty buffer and a committed save. generate_test.go:350 covers the same at unit level.
  - AC5: internal/commit/run_aifail_output_test.go:240-252 (`Oversized/Unattended` under -y) asserts the exact single StageFailure `{Name: "editor", Message: failLoudMessage}` with no Output and no Warn. A reached transport would fail that assertion through the seeded failing `claude`. internal/commit/run_oversized_failloud_test.go:17 covers both -y and non-TTY stdin with no note and no mutation.
  - AC6: run_oversized_test.go:519 `TestRun_Oversized_RegenNudgeOverCap_OpensEmptyEditor`. With a 200-byte nudge against a +100 headroom, it asserts exactly one transport call (the first generation only), exactly the oversized Warn, no StageFailed, one launch on an empty buffer, and the save committed. generate_test.go:373 covers the nudge-in-measure at unit level.
  - AC7: documentation only; README.md:199 is updated.
- Notes: Every test would fail if the guard were removed or moved back to a diff-only measure: the context and nudge cases are sized so that only the composed measure trips. The at-cap/over-cap pair is built from `composedSize`, so the boundary is exact rather than approximate. The Run-level oversized file carries overlapping happy-path over-cap cases (SkipsL2_RoutesToEditor, EmitsNote, OverLimitTriggersFallback, DistinctFromGenerationFailure/OversizedSkip). These are prior tests re-pointed at the byte cap, not new bloat, and none is wrong.

CODE QUALITY:
- Project conventions: Followed. The sentinel is wrapped with %w and matched with errors.Is. Output goes only through the Presenter. The tests are external packages with t.Parallel, use FakeRunner and RecordingPresenter, and assert exact Warn payloads.
- SOLID principles: Good. `checkPromptSize` is a small pure guard. The single `GenerateWithContext` path keeps the first generation and `r` under one measure.
- Complexity: Low
- Modern idioms: Yes (`slices.Concat`, errors.Is/As)
- Readability: Good
- Issues: One stale comment, internal/commit/run.go:661-665 (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/commit/run.go:661-665 — The comment above the regeneration stage says every exit closes it via "success via the timed completion closure, a transport failure via the spinner-stopping AI-failure Warn …, any other failure via surface's StageFailed". This task added the over-cap branch at run.go:681-684, which closes the stage through the oversized Warn, not a StageFailed, so the comment's "any other failure" claim is now false. Replace the sentence with: "Every exit closes it: success via the timed completion closure, a transport failure via the spinner-stopping AI-failure Warn and an over-cap prompt via the oversized Warn (`r` is interactive-only, so both paths are always attended), any other failure via surface's StageFailed." — FAILS: a reader trusting the comment expects an over-cap regeneration to emit a StageFailed. The code and TestRun_Oversized_RegenNudgeOverCap_OpensEmptyEditor (run_oversized_test.go:557) assert the opposite, which breaks the project's comments-true-to-as-built rule (CLAUDE.md, Comments).

UNSETTLED:
- None
