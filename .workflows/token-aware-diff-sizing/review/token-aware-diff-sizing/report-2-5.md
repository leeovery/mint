TASK: Remove `max_diff_lines` (token-aware-diff-sizing-2-5)

ACCEPTANCE CRITERIA:
- Given a `.mint.toml` setting `max_diff_lines` at the top level, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes` (§4.6)
- `mint setup`'s config reference and the README's per-key tables carry no `max_diff_lines` row (§18.1, §18.2)
- Given a forward release or `mint release regenerate <version>` whose diff runs past 50,000 lines and whose notes call fails, the ✗ line carries the AI failure's cause phrase, never `diff too large` (§12.4)
- Given `regenerate --all` where a version's notes fail, its skip warning and the end-of-run summary never read `diff too large` (§12.3)
- Given a `mint commit` whose composed prompt is over the cap, its editor note still reads `diff too large to summarise — opening editor` (§4.3)
- CLAUDE.md names no symbol this task removes: its config seam no longer names `max_diff_lines`, and its layout line for `internal/notes` names no removed export (§18.3)

STATUS: issues_found

SPEC CONTEXT: §4.1/§4.6: `max_prompt_bytes` replaces `max_diff_lines`. Under strict config the old key fails at load with a message naming its replacement, as the `[hooks]` placement error does. §12.3/§12.4: release-notes failures and `regenerate --all` skips no longer use `diff too large`. Commit's editor note is a different string and stays (§4.3). §18.1–§18.3: the setup guide's config reference (rendered from the SoT), the README tables, CLAUDE.md and code comments stay true to the removal.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:321-328 — `fileShape` has no `MaxDiffLines` field. `DefaultMaxDiffLines`, `Config.MaxDiffLines`, `resolveMaxDiffLines` and the `typeErrorMessages` entry are gone.
  - internal/config/config.go:550-551 — the replaced-key message sits beside the `[hooks]` placement case in `unknownKeyMessage`, limited to the top-level key (`len(key) == 1`).
  - internal/config/metadata.go:117-120 — the shared SoT rows hold ai_command, max_prompt_bytes, timeout and diff_exclude, with no max_diff_lines row. The setup guide renders the bare key from this SoT (internal/setupguide/setupguide.go:434-444).
  - README.md:238-243 — the shared table has no `max_diff_lines` row.
  - internal/notes/size.go:7 — only `ErrDiffTooLarge` survives. `CheckDiffSize` and `countDiffLines` are removed. In production code the only producer of `ErrDiffTooLarge` is commit's size guard (internal/commit/generate.go:166), so no release-notes path can raise it.
  - internal/notes/resolve.go:139-189 — `ClassifyFailure`/`CauseText` have no `ErrDiffTooLarge` case, so no ✗ line maps to `diff too large`.
  - internal/engine/regenerate_batch.go:386-392 — `notesSkipCause` returns the generic `reasonNotesFailed` for an unclassified failure. `reasonDiffTooLarge` and `classifyNotesFailure` are gone.
  - internal/commit/run.go:56 — `oversizedNoteMessage` is unchanged.
  - CLAUDE.md:25 — the `internal/notes` line names only `ErrDiffTooLarge`, which still exists, with an accurate gloss. CLAUDE.md:37 — the config seam names `max_prompt_bytes` as shared-only and gives `diff_exclude` its three additive levels, with no `max_diff_lines`.
  - Regenerate's load site is cmd/mint/main.go:163. It runs right after root resolution and ahead of request validation, release-branch resolution, preflight and dispatch. The error prints as `mint: invalid .mint.toml: …` on stderr. That cmd-layer shape is the same for every regenerate config error.
- Notes: A search of cmd/, internal/, README.md and CLAUDE.md finds no production comment that still describes a line-count guard. The remaining `max_diff_lines` mentions in config.go doc comments (lines 17 and 450) correctly describe the replaced-key handling. As the task's Context says, `/Users/leeovery/Code/agentic-workflows/.mint.toml:10` still sets `max_diff_lines = 60000` and will fail at load when that repo upgrades. That edit is outside this task by design.

TESTS:
- Status: Adequate (one gap, below)
- Coverage:
  - Load rejection: internal/config/max_diff_lines_removed_test.go:10 covers an integer value, a non-integer value (proving the strict-unknown path wins over a type error) and the key beside valid keys, each with an exact-string assertion.
  - Release: internal/engine/release_malformed_aicommand_test.go:46 — exact ✗ config message, git timeline limited to `rev-parse --show-toplevel`, no AI call.
  - Commit: internal/commit/run_malformed_aicommand_test.go:51 — exact error and StageFailed message, zero invocations.
  - README row absent: internal/config/max_diff_lines_removed_test.go:41. Setup guide row absent: internal/setupguide/setupguide_test.go:717. The `rowLineFor` cell-0 match works because the renderer emits the bare key.
  - ✗ line past 50,000 lines: internal/engine/notes_failure_no_line_ceiling_test.go:24 (forward release, exact `AI call failed` plus verbatim output) and :50 (regenerate fresh source via `RegenerateFreshBody`, exact `AI returned empty notes`, one transport call). Both use a 50,001-line diff under the byte cap, so a reintroduced line guard would break the exact-message assertions.
  - `--all` skip: internal/engine/notes_failure_no_line_ceiling_test.go:80 — skip warning and summary asserted exactly, plus an explicit "never contains diff too large" check across three failure shapes.
  - Commit editor note: internal/commit/run_oversized_test.go:30 constant, asserted exactly at :159, :260, :504, :554.
- Notes: No test proves that `mint release regenerate <version>` rejects the key before any step. See FINDINGS.

CODE QUALITY:
- Project conventions: Followed. Strict config gets a targeted unknown-key message beside the `[hooks]` case, the SoT drift/bijection test is kept in step, the README tripwire is unaffected, and tests are external-package, table-driven and `t.Parallel()`.
- SOLID principles: Good
- Complexity: Low. The change is a removal plus one guarded branch.
- Modern idioms: Yes
- Readability: Good. Comments edited in this change (config.go package doc, `fileShape`, `translateStrict`, `resolveTimeout`, `notesFailureOutput`, `skippedVersion.Reason`) hold true against the code.
- Issues: None beyond the finding below

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/release_malformed_aicommand_test.go:46 — The criterion names three verbs, but only `mint release` (this file) and `mint commit` (internal/commit/run_malformed_aicommand_test.go:51) have a test. `mint release regenerate <version>` loads config on a separate path (cmd/mint/main.go:163), and no test drives it with an invalid `.mint.toml` (a search for `invalid .mint.toml` in cmd/mint tests finds nothing). Fix: add a cmd/mint test that writes `max_diff_lines = 50000` into the temp root and calls `regenerate(t.Context(), rec, f, req)` with the runner seeded only for `rev-parse --show-toplevel`. It asserts a non-zero exit code and that the runner saw exactly that one git call, the same "nothing after load" proof the release and commit tests use. The message itself is already pinned by internal/config/max_diff_lines_removed_test.go. — FAILS: if regenerate's config load moved after release-branch resolution, preflight or the interactive prompts, or were rerouted through a path that tolerates unknown keys, no test fails. Regenerate would then run steps against a repo still carrying the dead key, against the criterion's "before any step runs".

UNSETTLED:
- None
