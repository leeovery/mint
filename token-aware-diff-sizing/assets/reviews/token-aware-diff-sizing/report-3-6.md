TASK: One non-AI body: remove `[release].fallback` (token-aware-diff-sizing-3-6)

ACCEPTANCE CRITERIA:
- Given a `.mint.toml` setting `fallback` under `[release]`, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: release.fallback has been removed — every non-AI body is the commit-subject list; supply hand-written notes with --notes-file` (§14.2)
- Given `--no-ai` on a release with commits since the last tag, the body is the subject list `git log --format=%s {lastTag}..HEAD` returns. Given `on_notes_failure = "fallback"` and a failed AI call, the same list ships (§14.1)
- Given either route and a commit-subject list that comes back empty or whitespace-only, the maintenance-release body ships instead, so the tag body is never empty (§14.1)
- Given `on_notes_failure` set to `abort` or `fallback`, config loads as before. Any other value is still rejected at load as built (§14.1)
- `mint setup`'s config reference and the README's `[release]` table carry no row for `fallback` (§14.2, §18.1, §18.2)
- `on_notes_failure`'s description, as `mint setup`'s config reference renders it, and its README row both say `fallback` ships the commit-subject list, naming no fixed string (§14.2, §18.1)
- `mint setup`'s AI-transport paragraph names the commit-subject list as the one non-AI body, in place of "fall back to the commit-subject / fallback body" (§18.1)

STATUS: complete

SPEC CONTEXT: §14.1 drops the fixed `[release].fallback` body so `--no-ai`, `on_notes_failure = fallback` and the floor's `c` all give the release's commit-subject list (`git log --format=%s {lastTag}..HEAD`), with the maintenance-release body when that list is empty; `on_notes_failure` keeps abort/fallback. §14.2 has a lingering key fail at load with the exact removed-key message beside the `[hooks]` placement message, and removes the key's metadata/README rows and the fixed-string wording from `on_notes_failure`'s descriptions. §18.1 has the setup guide's AI-transport paragraph name the commit-subject list as the one non-AI body; §18.3 has comments stating the fixed fallback string rewritten in the same change.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:349-366 — `releaseShape` no longer carries `fallback`; `Release` (config.go:209-226) has no `Fallback` field
  - internal/config/config.go:553-555 — removed-key message in `unknownKeyMessage`, beside the `[hooks]` (config.go:547-549) and `max_diff_lines` (config.go:550-552) messages; reached through `translateStrict` (config.go:452-463), which wraps it as `invalid .mint.toml: …`
  - internal/config/config.go:817-830 — `validateOnNotesFailure` still admits only abort/fallback (closed set at config.go:64)
  - internal/config/metadata.go:134 — `on_notes_failure` row: "fallback ships the commit-subject list (the maintenance-release body when it is empty)"; no `fallback` row remains
  - internal/notes/resolve.go:54-71 — `ResolveFailure` fallback mode and `resolveFallbackBody` build only the commit-subject list via `rangeSubjects` (resolve.go:21-27), floored to `StubBody()` by `subjectBody` (resolve.go:99-105)
  - internal/notes/noai.go:25-27 — `NoAIBody` takes no `config.Release` and shares `resolveFallbackBody`
  - internal/notes/select.go:202 — `--no-ai` caller updated
  - internal/setupguide/setupguide.go:180-182 — AI-transport paragraph: "or fall back to the commit-subject list, the one non-AI body"
  - README.md:256 — `on_notes_failure` row says `fallback` ships the commit-subject list; the `fallback` row is gone
  - Verb entry points: internal/engine/release.go:370 (release), internal/commit/run.go:304 (commit), cmd/mint/main.go:163-166 (regenerate) — all load through the one `config.Load`, before any pipeline step
- Notes: Every stale comment §18.3 names was rewritten (config.go:14-25, :61-64, :172-176, :445-451; metadata.go:122-125; noai.go:9-24; resolve.go:29-48). The remaining "fallback body" wording in cmd/mint/flags.go:87, cmd/mint/usage.go:40 and README.md:99 names the commit-subject list, not a fixed string, so it stays true. As built, regenerate surfaces a load error as `mint: invalid .mint.toml: …` on stderr with exit 2. That path predates this feature (cmd/mint/main.go:163-166, June commits) and this task did not change it.

TESTS:
- Status: Adequate
- Coverage:
  - Removed-key message, exact text: internal/config/fallback_removed_test.go:14-43. It covers a string value, an empty string, a non-string value, and the key set beside on_notes_failure. Lines 45-75 check that `fallback` at the top level and under `[commit]` keeps the generic unknown-key message.
  - Verb-level, before any step: internal/engine/release_malformed_aicommand_test.go:50-54. Only `git rev-parse --show-toplevel` runs, and the single ✗ config line carries the exact message. internal/commit/run_malformed_aicommand_test.go:55-59 shows zero invocations and the exact message.
  - Commit-subject list for both routes: internal/notes/noai_test.go:14-36 and internal/notes/resolve_test.go:77-100. Both assert exact argv `log --format=%s {lastTag}..HEAD` and a verbatim body. noai_test.go:72-116 checks that the two routes give identical bodies.
  - Empty and whitespace-only list → maintenance-release body on both routes: noai_test.go:144-193 asserts exactly `StubBody()` and the single git log.
  - on_notes_failure values: fallback_removed_test.go:77-96 and config_test.go:1112-1133 for both values loading; config_test.go:349-365 and :1092-1110 for rejection.
  - Metadata and README rows: fallback_removed_test.go:98-152.
  - Setup guide config reference and AI-transport paragraph: internal/setupguide/setupguide_test.go:782-821.
  - The bijection, census and drift tests were updated to 24 pairs.
- Notes:
  - No verb-level test drives `regenerate(...)` with the removed key, and none exists for any other config-load error on regenerate. Regenerate prints the load error straight to os.Stderr from the cmd layer. Its behaviour holds structurally: the same `config.Load` and exact message are pinned at config level, and cmd/mint/main.go:163 loads config before preflight or any mutation.
  - fallback_removed_test.go:77-96 (`TestLoad_OnNotesFailure_KeepsBothValues`) repeats config_test.go:1112-1133 (`TestLoad_OnNotesFailureValidValues_Load`) case for case. This is minor redundancy, not a defect.

CODE QUALITY:
- Project conventions: Followed. The change follows the strict-config seam: decode shape, SoT row, README row and the friendly removed-key message all moved together. Messages are lowercase with no trailing punctuation. Tests are external packages, table-driven, use `t.Parallel()`, and assert exact argv and exact messages.
- SOLID principles: Good. A single shared selector serves every non-AI route, and `NoAIBody` dropped its now-unused `config.Release` dependency.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: None

BLOCKING ISSUES:
- None

FINDINGS:
- None

UNSETTLED:
- None
