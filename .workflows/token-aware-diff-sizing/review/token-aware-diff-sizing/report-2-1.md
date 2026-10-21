TASK: `max_prompt_bytes` config key (token-aware-diff-sizing-2-1, tick-8f3b67)

ACCEPTANCE CRITERIA:
- With no `.mint.toml`, or one that does not set `max_prompt_bytes`, config loads with a cap of 1,500,000 bytes. With `max_prompt_bytes = 800000` at the top level, the cap is 800,000 bytes (§4.1)
- `max_prompt_bytes = 0` or `-1` makes each of `mint release`, `mint release regenerate <version>` and `mint commit` fail before any step runs with `invalid .mint.toml: max_prompt_bytes must be a positive number of bytes`. Zero never means "no cap" (§4.1)
- A non-integer `max_prompt_bytes` (`"1500000"`, `1.5`) fails loading with a friendly `invalid .mint.toml: …` type error that names `max_prompt_bytes` and its integer type (§4.1)
- `max_prompt_bytes` under `[release]` or `[commit]` fails loading with the as-built unknown-key message naming the key and its table (§4.1)
- `mint setup`'s config reference lists `max_prompt_bytes` at the shared top level with its 1,500,000 default, described as a cap, in bytes, on the composed prompt of any single AI call (§4.1, §18.1)
- The README's shared engine keys table lists `max_prompt_bytes` with its 1,500,000 default and the same meaning (§18.2)
- CLAUDE.md's config seam names `max_prompt_bytes` as shared-only at the top level (§18.3)

STATUS: issues_found

SPEC CONTEXT: §4.1 says `max_prompt_bytes` replaces `max_diff_lines`. It is an integer count of bytes, defaults to 1,500,000, sits at the shared top level only, and bounds the composed prompt of every single AI call. It follows the strict-config path: a schema entry, a friendly type error, a metadata row and the README tables. A value of zero or less is rejected at load in the malformed-`ai_command` error shape, and zero never means "no cap". §4.5 explains the default's grounds (about 675k tokens at 2.2 bytes per token, chosen high on purpose). §18.1–18.3 name the documentation surfaces: the setup guide, the README and CLAUDE.md.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - internal/config/config.go:70 — `DefaultMaxPromptBytes = 1_500_000`, exported so the SoT row and the setup guide derive the value from it
  - internal/config/config.go:129 — `Config.MaxPromptBytes int`; internal/config/config.go:305 — zero-config seed in `defaults()`
  - internal/config/config.go:325 — `fileShape.MaxPromptBytes *int`. The pointer keeps an explicit 0 apart from an absent key, so the 0 reaches the positive-value check
  - internal/config/config.go:431-433 — `validateMaxPromptBytes` called in `Load` beside the other load-time checks; internal/config/config.go:834-839 — rejects `<= 0` with the exact AC message
  - internal/config/config.go:439, :685-690 — `resolveMaxPromptBytes` applies the default when the key is absent
  - internal/config/config.go:473 — `typeErrorMessages` entry `"max_prompt_bytes must be an integer (bytes)"`. internal/config/config.go:494-496 and :515-539 — `topLevelIntegerKeys`/`nonIntegerKey` cover the float and boolean mismatches. go-toml v2.3.1 reports these with no struct-field path: `unmarshalFloat` raises "float cannot be assigned to %s" and `unmarshalBool` raises "cannot assign boolean to a %t", so the field-path lookup alone cannot see them. The fallback only fires when `max_prompt_bytes` is present with a non-int64 value, so a valid cap is never blamed for another key's error.
  - Per-verb rejection comes free from `DisallowUnknownFields`, because `releaseShape` and `commitShape` carry no such field. `unknownKeyMessage` (internal/config/config.go:562-563) renders `unknown key "max_prompt_bytes" in [release]` / `[commit]`
  - internal/config/metadata.go:118 — shared SoT row; its Default comes from `strconv.Itoa(DefaultMaxPromptBytes)` and its description matches the AC wording. The setup guide renders the config reference from this row
  - README.md:235 (shared-only sentence) and README.md:241 (table row: default `1500000`, the cap meaning, must be positive)
  - CLAUDE.md:37 — "`max_prompt_bytes` is shared-only at the top level"
  - Verb wiring: `engine.Release` and `commit.Run` abort at config load, and the tests below prove it. `regenerate` in cmd/mint/main.go:163-167 calls `config.Load` before validation, branch resolution or any dispatched step, and prints `mint: <err>`, so the same message surfaces.
- Notes: Nothing is added to `internal/initgen`, as the plan requires. Every criterion is met in substance.

TESTS:
- Status: Adequate
- Coverage:
  - internal/config/max_prompt_bytes_test.go:12 — no file, file without the key, 800000, and the 1-byte positive boundary
  - internal/config/max_prompt_bytes_test.go:46 — 0 and -1 rejected with the exact message
  - internal/config/max_prompt_bytes_test.go:68 — string, float and boolean all give the friendly integer-type message. This exercises both the field-path branch and the `nonIntegerKey` branch.
  - internal/config/max_prompt_bytes_test.go:99 — the exact unknown-key message under `[release]` and `[commit]`
  - internal/config/max_prompt_bytes_test.go:130 — SoT row default and description, and no per-verb row
  - internal/config/max_prompt_bytes_test.go:159 — README shared-table row
  - internal/setupguide/setupguide_test.go:785 — rendered config reference row at top level with no verb-level row
  - Verb-level proof of AC2: internal/engine/release_malformed_aicommand_test.go:35-44 asserts only `git rev-parse --show-toplevel` runs, with no AI call and one config StageFailed. internal/commit/run_malformed_aicommand_test.go:40-49 asserts zero invocations.
- Notes: The change-set has no cmd-level test for `mint release regenerate <version>` with a non-positive cap. Reading settles that path: the check lives in the shared `config.Load`, and regenerate's pre-existing `config.Load` call precedes every step. A new test would only re-guard that pre-existing wiring, so I raise no finding. Overlap is small and deliberate: the config unit tests pin the message, and the verb tests prove the wiring.

CODE QUALITY:
- Project conventions: Followed. This is the strict-config path from CLAUDE.md seam 6: schema field, `typeErrorMessages` entry, SoT row, README row, error messages in `invalid .mint.toml: …` shape, and lowercase with no trailing punctuation.
- SOLID principles: Good. Validation and resolution are separate single-purpose helpers.
- Complexity: Low
- Modern idioms: Yes
- Readability: Good
- Issues: One test comment is stale (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/config/metadata_test.go:355-356, :371-375 — The comments on `TestMetadataRows_ConcreteScalarDefaultsRenderVerbatim` say the test "proves every key with a concrete scalar default renders that default's string form verbatim" and that the table is kept "complete". This work removed the `max_diff_lines` row (and the file's `strconv` import) without adding `max_prompt_bytes`. `max_prompt_bytes` is now the only concrete scalar default the table does not carry. Fix: add `{configtest.RowKey{Level: config.LevelShared, Key: "max_prompt_bytes"}, strconv.Itoa(config.DefaultMaxPromptBytes)}` beside the `ai_command` row and restore `"strconv"` to the import block, which mirrors the deliberate overlap the comment already describes for `ai_command`. The alternative is to reword both comments to say the `max_prompt_bytes` render pin lives in `TestMetadataRows_SharedMaxPromptBytesRow` (internal/config/max_prompt_bytes_test.go:130). — FAILS: the comment makes a claim the code falsifies, against CLAUDE.md's "true to as-built" rule. A maintainer who trusts the "complete" table believes every scalar default's render is pinned there, and looks there in vain for the cap's pin.

UNSETTLED:
- None
