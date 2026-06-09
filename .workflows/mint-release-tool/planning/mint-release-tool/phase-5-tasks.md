---
phase: 5
phase_name: Regenerate / Backfill (Heal & History Rewrite)
total: 13
---

## mint-release-tool-5-1 | approved

### Task mint-release-tool-5-1: regenerate command skeleton & two-axis flag parsing

**Problem**: `mint release regenerate` does not yet exist as a command. Before any source/target axis logic, version resolution, or write path can be built, mint needs the subcommand wired under `release` with its flag surface parsed, plus the two structural argument-presence rules (a target must be named, and `<version>` vs `--all` are mutually exclusive).

**Solution**: Add `regenerate` as a subcommand of `mint release` (not a top-level verb), parsing its flag set — `--reuse` / `--fresh` (source, default fresh), `--target release|changelog|both` (one flag taking a value), `--all`, and `-y`/`--yes` — plus the positional `<version>` argument. Enforce the two presence rules: a bare `regenerate` with neither `<version>` nor `--all` is an error; supplying both `<version>` and `--all` is an error (mutually exclusive). Reject an unknown `--target` value.

**Outcome**: `mint release regenerate <version>` and `mint release regenerate --all` parse into a populated request struct; `regenerate` alone and `regenerate <version> --all` both fail loud with clear messages; `--target` accepts exactly the three values and rejects anything else.

**Do**:
- Register `regenerate` as a subcommand of the existing `release` command (mirroring how `release` itself is wired in Phase 1's command layer). Do not add a top-level `notes` or `regenerate` verb — it nests under `release`.
- Parse a `regenerateRequest` (or equivalent) holding: `Version string` (positional, optional), `Source` (an enum: reuse | fresh, default fresh when `--fresh`/neither given), `Target` (an enum: release | changelog | both | unset), `All bool`, `Yes bool`.
- `--target` is a single flag taking one value (`release`, `changelog`, or `both`); do NOT model it as separate `--release`/`--changelog` boolean flags. An unrecognised value (e.g. `--target tag`, `--target foo`) errors: "invalid --target value `<v>` (expected release, changelog, or both)".
- `--reuse` and `--fresh` are mutually exclusive source selectors; default is fresh when neither is given. (Combining `--reuse --fresh` is an error — they pick the same axis.)
- Presence rule A: neither `<version>` nor `--all` given → error "specify a version or --all".
- Presence rule B: both `<version>` and `--all` given → error "cannot combine a version with --all" (mutually exclusive).
- Leave the *semantic* axis-contract validation (reuse⇒release-only, changelog-disabled, fresh -y needs target) to task 5-2; this task is parse + the two presence rules + unknown-value rejection only.
- Wire all external command execution (none needed yet at this layer) through the existing CommandRunner seam established in Phase 1; the command handler is testable with FakeRunner + RecordingPresenter.

**Acceptance Criteria**:
- [ ] `regenerate` is reachable as `mint release regenerate ...`, not as a top-level command
- [ ] `--target` parses a single value and rejects any value other than `release` / `changelog` / `both`
- [ ] `--reuse` / `--fresh` parse into a source enum defaulting to fresh; `--all` and `-y` parse to booleans; `<version>` parses to the positional
- [ ] Bare `regenerate` (no `<version>`, no `--all`) errors "specify a version or --all"
- [ ] `regenerate <version> --all` errors (mutually exclusive)
- [ ] No mutation or network call occurs during parse/validation

**Tests**:
- `"it parses regenerate <version> with --reuse and --target release into the request"`
- `"it defaults source to fresh when neither --reuse nor --fresh is given"`
- `"it parses --target both as a single-flag value"`
- `"it errors on an unknown --target value"`
- `"it errors on bare regenerate with neither version nor --all"`
- `"it errors when both <version> and --all are supplied"`
- `"it errors when --reuse and --fresh are combined"`

**Edge Cases**:
- Bare `regenerate` (neither `<version>` nor `--all`) → error
- Both `<version>` and `--all` → error (mutually exclusive)
- `--target` value parsing for the three valid surfaces
- Unknown `--target` value → error

**Context**:
> CLI Surface: `mint release regenerate <version> [flags]` and `mint release regenerate --all [flags]`. "`regenerate` is a subcommand of `release`, not a top-level `notes` verb — `notes` is the wrong noun and ages badly." Flags: `--reuse` (implies `--target release`), `--fresh` (default), `--target release|changelog|both` (default asked interactively), `--all`, `-y/--yes`.
> Version arg / argument presence: "a bare `regenerate` with neither `<version>` nor `--all` is an error ('specify a version or `--all`'); supplying both `<version>` and `--all` is also an error (mutually exclusive)."
> Two-axis contract: "Canonical spelling is `--target <surface>` (one flag, a value) — not separate `--release`/`--changelog` flags."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes (non-destructive)" (Two-axis contract; Version argument & diff base resolution) and "CLI Surface & Flags" (`regenerate` flags).

## mint-release-tool-5-2 | approved

### Task mint-release-tool-5-2: Source×target axis contract validation

**Problem**: The two regenerate axes (source × target) are not independent — `--reuse` can only write the provider release, `changelog`/`both` targets are meaningless when the changelog is disabled, and a fresh unattended run has no default surface to write. Without enforcing these, mint could write a file from itself, create a CHANGELOG a project opted out of, or guess which live surface to rewrite.

**Solution**: After parse (5-1), validate the source×target contract: `--reuse` is release-only (errors on `--target changelog` AND `--target both`); `--target changelog`/`both` with `changelog = false` in config errors "changelog is disabled in config"; fresh + `-y` without an explicit `--target` errors "--target is required with --fresh -y"; and `--reuse` *implies* `--target release` (so an unset target under reuse resolves to release, unaffected by `-y`).

**Outcome**: Every illegal source×target combination fails loud with a precise message before any preflight or work begins; `--reuse` with no `--target` resolves to release; fresh `-y` runs are forced to name a surface.

**Do**:
- Implement an axis-contract validator that runs after 5-1 parse and has access to the loaded config (`changelog` bool, default true).
- `--reuse` ⇒ release-only: if source is reuse and target is `changelog` or `both` → error. Message: "`--reuse` writes the provider release only; it cannot target the changelog" (reuse's source *is* the notes record, so reuse→changelog would write a file from itself, a no-op).
- `--reuse` implies `--target release`: if source is reuse and target is unset, resolve target to `release`. This holds regardless of `-y` (reuse has a deterministic default surface).
- changelog-disabled: if target resolves to `changelog` or `both` and config `changelog = false` → error "changelog is disabled in config". This applies whether the target came from a flag (single mode) or will be batch-validated (5-12 reuses this same check up front for `--all`).
- fresh + `-y` without `--target`: if source is fresh AND `-y` is set AND target is unset → error "--target is required with --fresh -y". (`-y` skips the interactive target prompt and fresh has no default surface, so mint must be told.) This also covers `--fresh --all -y` without `--target`.
- Order the checks so the most specific message wins; do not silently default a fresh target.
- Tested with FakeRunner + RecordingPresenter and an in-memory config; no mutation.

**Acceptance Criteria**:
- [ ] `--reuse --target changelog` and `--reuse --target both` both error
- [ ] `--reuse` with no `--target` resolves target to `release`
- [ ] `--reuse` target resolution to release is unaffected by `-y`
- [ ] `--target changelog` or `--target both` with `changelog = false` errors "changelog is disabled in config"
- [ ] `--fresh -y` (and `--fresh --all -y`) without `--target` errors "--target is required with --fresh -y"
- [ ] `--fresh` without `-y` and without `--target` does NOT error here (interactive prompt resolves it later, task 5-10)

**Tests**:
- `"it errors on --reuse --target changelog"`
- `"it errors on --reuse --target both"`
- `"it resolves --reuse with no target to release"`
- `"it resolves --reuse to release even with -y"`
- `"it errors on --target changelog when changelog is disabled in config"`
- `"it errors on --target both when changelog is disabled in config"`
- `"it errors on --fresh -y without --target"`
- `"it errors on --fresh --all -y without --target"`
- `"it allows --fresh without -y and without --target (deferred to interactive)"`

**Edge Cases**:
- `--reuse --target changelog` → error
- `--reuse --target both` → error
- `--target changelog` with `changelog = false` → error
- `--target both` with `changelog = false` → error
- `--fresh -y` without `--target` → error
- `--reuse` implies `--target release`

**Context**:
> "`--reuse` ⇒ release-only. Its source *is* the notes record, so 'reuse → write changelog' would write a file from itself (a no-op) → mint errors on `--reuse --target changelog`."
> "`--target changelog`/`both` when `changelog = false` → error (fail-loud: 'changelog is disabled in config'). mint never silently creates a CHANGELOG the project opted out of."
> "Fresh + `-y` requires an explicit `--target`. Since `-y` skips the interactive target prompt and the fresh path has no default surface, `regenerate --fresh -y` (and `--fresh --all -y`) without `--target` is a fail-loud error ('`--target` is required with `--fresh -y`'). mint never guesses which live surface(s) to rewrite unattended. (`--reuse` is unaffected — it implies `--target release`.)"
> Scope fence: full config schema fail-loud validation (unknown keys/bad types) is Phase 6; the regenerate-specific `changelog=false` target error IS in scope here.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Two-axis contract; Interactive by default, flags to skip).

## mint-release-tool-5-3 | approved

### Task mint-release-tool-5-3: Version argument & diff-base resolution

**Problem**: The `<version>` a user passes may or may not carry the `tag_prefix`, may not correspond to any real tag, and (for the oldest release) may have no predecessor to diff against. Regenerate must normalise the argument, resolve it to an existing tag, and compute the fresh diff base — failing loud when there is no matching tag and mirroring the forward first-release rule when there is no predecessor.

**Solution**: Normalise `<version>` with or without `tag_prefix` (`regenerate v1.4.0` ≡ `regenerate 1.4.0`, including monorepo prefixes like `pkg-name/v`), resolve it against the repo's existing tags, and fail loud ("no tag `vX.Y.Z` found") when no matching tag exists. For the fresh path, compute the diff base as `vX-1..vX` (previous tag → target tag); for the oldest release (no `vX-1`), mark it as the first-release case so the notes path uses the fixed body "Initial release." with no AI.

**Outcome**: A version argument resolves deterministically to a real tag (prefix-agnostic); a non-existent version fails loud; the fresh diff base is `vX-1..vX`; the oldest release resolves to the first-release fixed-body path with no AI.

**Do**:
- Reuse the tag grammar / normalisation built in Phase 1 (`^{tag_prefix}(\d+)\.(\d+)\.(\d+)$`, prefix read off existing tags). Normalise the supplied `<version>`: strip a leading `tag_prefix` if present, parse the strict 3-part semver, then re-apply `tag_prefix` to form the canonical tag string. `regenerate v1.4.0` and `regenerate 1.4.0` must resolve identically; honour a configured monorepo `tag_prefix` (e.g. `pkg-name/v`).
- Resolve the canonical tag against the set of existing tags (the same global tag set Phase 1 reads). No matching tag → fail loud: "no tag `<tag>` found". (Mirror the forward path's tag-as-truth tag resolution.)
- Compute the fresh diff base: find the previous matching SemVer tag (the next-lower version in the sorted set) → range `vX-1..vX`. The predecessor is the numerically previous *matching* tag, consistent with the forward path's "previous latest" notion, not `git describe` ancestry.
- Oldest release (no predecessor matching tag) → flag the resolution result as `firstRelease = true` so the fresh source path (5-6) skips the AI and emits the fixed body "Initial release." This mirrors the forward first-release rule exactly.
- This task resolves the version/base only; consuming it (reuse read in 5-5, fresh diff in 5-6) is separate. Return a struct carrying the canonical tag, the resolved diff range (or first-release marker).
- All git reads (tag listing) go through the CommandRunner; tested with FakeRunner + RecordingPresenter.

**Acceptance Criteria**:
- [ ] `regenerate v1.4.0` and `regenerate 1.4.0` resolve to the same canonical tag
- [ ] A configured monorepo `tag_prefix` (e.g. `pkg-name/v`) is honoured in both stripping and re-applying
- [ ] A `<version>` with no matching tag fails loud "no tag `<tag>` found"
- [ ] The fresh diff base is the `vX-1..vX` range (previous matching tag → target tag)
- [ ] The oldest release (no predecessor) resolves to the first-release marker (no AI, fixed body "Initial release.")
- [ ] No version *computation* (next-version bump) happens — regenerate targets an existing version

**Tests**:
- `"it normalises a version given with the tag_prefix"`
- `"it normalises a version given without the tag_prefix"`
- `"it resolves a monorepo tag_prefix version"`
- `"it fails loud when no tag matches the version"`
- `"it computes the vX-1..vX diff base from the previous matching tag"`
- `"it marks the oldest release as first-release (no AI, fixed body)"`
- `"it ignores non-matching tags when finding the predecessor"`

**Edge Cases**:
- Version with `tag_prefix`
- Version without `tag_prefix`
- No matching tag → fail loud
- Oldest release (no `vX-1`) → fixed "Initial release.", no AI
- Monorepo `tag_prefix`

**Context**:
> "`<version>` argument may be given with or without `tag_prefix` (`regenerate v1.4.0` ≡ `regenerate 1.4.0`); mint normalises it. A `<version>` with no matching tag → fail loud ('no tag `vX.Y.Z` found')."
> "Fresh diff base = `vX-1..vX` (previous tag → target tag). For the oldest release (no predecessor tag — a single regenerate of the first release, or the first version in an `--all` backfill), there is no `vX-1`, so mint mirrors the forward path's first-release rule: no AI, fixed body 'Initial release.'"
> Tag grammar (Stage 1): "`tag_prefix` config, default `'v'` … The same knob covers component/monorepo prefixes, e.g. `tag_prefix = 'pkg-name/v'`." Regenerate does no version compute (it targets an existing version).

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Version argument & diff base resolution) and "Stage 1 — Version Determination & Tag Grammar".

## mint-release-tool-5-4 | approved

### Task mint-release-tool-5-4: Regenerate preflight subset per verb

**Problem**: Regenerate is far lighter than a forward release — it never cuts a tag and, in reuse mode, never mutates git at all. Running the full forward preflight gate set would needlessly block heal runs (e.g. demanding a clean tree for a reuse-only provider re-create). Preflight is a gate *set*; regenerate must run only the relevant subset.

**Solution**: Apply the regenerate-specific preflight subset. `--reuse` (release-only, no git mutation) runs gh-auth ONLY. Fresh with a changelog/both target (commits + pushes) runs gh-auth + clean-tree + branch + remote-sync. Neither path runs the tag-free gate (tags exist and are untouched), and neither computes a version.

**Outcome**: A reuse run passes preflight with only `gh` auth checked; a fresh changelog/both run additionally checks clean-tree, branch, and remote-sync; the tag-free gate never runs; no version is computed.

**Do**:
- Reuse the Phase 1/4 preflight gate implementations (gh-auth, clean-tree, on-branch, remote-sync, tag-free) as a composable set; select the subset by the resolved request.
- `--reuse` (always release-only) → run gh-auth ONLY. It must run gh-auth even though it does not commit/push — "a dead `gh` auth is the usual reason you're healing". No clean-tree, branch, or remote-sync (no git mutation occurs in reuse).
- Fresh + target `changelog` or `both` (commits the CHANGELOG and pushes) → clean-tree + branch + remote-sync; plus gh-auth when a provider write will occur (`both`).
- General selection rule to encode: calls `gh` (a provider write — `release`/`both`) → gh-auth; commits + pushes (`changelog`/`both`) → clean-tree + branch + remote-sync; cuts a new tag → tag-free (never true for regenerate). A pure `--target changelog` fresh run pushes a commit but writes no provider release → clean-tree/branch/remote-sync but no gh-auth. A fresh `--target release` run writes only the provider release (no changelog commit) → gh-auth only.
- NEVER run the tag-free gate in any regenerate mode — the target tag is *supposed* to exist; regenerate operates on existing tags and never cuts a new one.
- No version compute step runs (5-3 resolves an existing version; there is no bump).
- All gates run behind CommandRunner; tested with FakeRunner + RecordingPresenter asserting exactly which gates ran.

**Acceptance Criteria**:
- [ ] `--reuse` runs gh-auth only — no clean-tree, branch, remote-sync, or tag-free
- [ ] Fresh `--target changelog`/`both` runs clean-tree + branch + remote-sync (commits + pushes)
- [ ] gh-auth runs whenever a provider write occurs (`release`/`both`); not for pure `changelog`
- [ ] The tag-free gate never runs in any regenerate mode
- [ ] No version computation occurs
- [ ] A failing applicable gate aborts cleanly before any work

**Tests**:
- `"it runs only gh-auth for --reuse"`
- `"it runs gh-auth + clean-tree + branch + remote-sync for fresh --target both"`
- `"it runs clean-tree + branch + remote-sync for fresh --target changelog"`
- `"it does not run gh-auth for fresh --target changelog (no provider write)"`
- `"it runs gh-auth only for fresh --target release"`
- `"it never runs the tag-free gate in any regenerate mode"`
- `"it computes no version"`
- `"it aborts cleanly when an applicable gate fails"`

**Edge Cases**:
- reuse → gh-auth only
- fresh changelog/both → gh-auth + clean-tree + branch + remote-sync
- not tag-free (tags untouched)
- no version compute
- reuse skips git-mutation gates

**Context**:
> "Preflight is a *gate set*; each command runs only the relevant subset (general rule: *calls `gh` → gh-auth; commits + pushes → clean-tree / branch / remote-sync; cuts a new tag → tag-free*):
> - `regenerate --reuse` (release-only, no git mutation) → gh-auth only (it must run that — a dead `gh` auth is the usual reason you're healing).
> - `regenerate` fresh → changelog / both (commits + pushes) → gh-auth + clean-tree + branch + remote-sync; not tag-free (tags exist, untouched); no version compute."
> Ambiguity note: the spec groups gh-auth under the fresh changelog/both bucket. The underlying general rule ("calls `gh` → gh-auth") implies gh-auth is gated on a *provider write* (release/both), so a pure `--target changelog` run does not need it. The task encodes the general rule and tests both surfaces so the executor can confirm intent.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Preflight subset per verb).

## mint-release-tool-5-5 | approved

### Task mint-release-tool-5-5: Reuse source — read tag annotation body

**Problem**: The `--reuse` path heals a release from the immutable record without re-running the AI or re-diffing. mint must read the tag annotation body — its single read source — deterministically and parse-free, and fail loud (in single mode) when a tag has no usable annotation body, rather than writing an empty release.

**Solution**: For `--reuse`, read the tag's annotation body via the single deterministic git call `git for-each-ref --format='%(contents:body)' refs/tags/<tag>` and use the result whole (no parsing, no re-diff, no AI). A tag with no annotation body — lightweight (no annotation) or an empty/whitespace-only body — fails loud in single mode with "tag `vX.Y.Z` has no annotation body — use --fresh". (The `--all` skip-and-report variant of this condition is task 5-12.)

**Outcome**: `--reuse <version>` yields the exact tag annotation body to feed the provider release; a tag with no/empty annotation body fails loud in single mode with the "use --fresh" hint; no AI or diff runs.

**Do**:
- Implement a `readTagBody(tag)` reading via `git for-each-ref` with `--format` selecting `%(contents:body)` for `refs/tags/<tag>` (the single read source established in earlier phases — the same call the forward path's annotation is the source of truth for). Run through CommandRunner.
- Use the returned body **whole** — no parsing, no splitting, no validation transform. (The body was already presentation-format when written.)
- Detect "no annotation body": a lightweight tag (no annotation object) and an annotated tag whose body is empty or whitespace-only both surface as an empty/whitespace `contents:body`. Treat both as "no annotation body".
- Single mode (a `<version>` reuse) with no annotation body → fail loud: "tag `<tag>` has no annotation body — use --fresh". Do NOT write an empty provider release body.
- Do NOT implement the `--all` skip-and-report behaviour here — that lives in 5-12; this task owns the single-mode read + single-mode error. (Structure `readTagBody` so 5-12 can call it and branch on the empty result.)
- No AI invocation, no diff assembly on the reuse path.
- Tested with FakeRunner (canned `for-each-ref` output) + RecordingPresenter.

**Acceptance Criteria**:
- [ ] The annotation body is read via a single `git for-each-ref … contents:body` call and used whole (no parse)
- [ ] A lightweight tag (no annotation) → single-mode fail-loud "has no annotation body — use --fresh"
- [ ] An empty or whitespace-only annotation body → single-mode fail-loud (same message)
- [ ] No AI call and no diff assembly occur on the reuse path
- [ ] A non-empty body is returned verbatim for downstream provider write

**Tests**:
- `"it reads the tag annotation body via for-each-ref and returns it whole"`
- `"it errors loud with use --fresh hint on a lightweight tag (no annotation body)"`
- `"it errors loud on an empty annotation body"`
- `"it errors loud on a whitespace-only annotation body"`
- `"it invokes no AI and assembles no diff on the reuse path"`

**Edge Cases**:
- Lightweight tag → no annotation body error
- Empty/whitespace body → error
- "use --fresh" hint present
- Body used whole (deterministic, no parse)

**Context**:
> "`--reuse` — read the tag annotation body (the single source of truth; deterministic git read, no parsing, config-independent). No AI, no re-diff — can't drift."
> Body distribution: "Tag annotation … This is the single source mint ever reads — `regenerate --reuse` reads the annotation body via one deterministic git call (`git for-each-ref … contents:body`), no parsing."
> "`--reuse` with no annotation body: a tag not created by mint may be lightweight (no annotation) or carry an empty/whitespace body (`git for-each-ref … contents:body` returns empty). `--reuse` against such a tag is a fail-loud error in single mode ('tag `vX.Y.Z` has no annotation body — use `--fresh`'); in `--all` mode it is skipped and reported … never written as an empty release body."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Two-axis contract; Version argument & diff base resolution) and "Body Distribution: Tag vs Changelog vs Provider Release".

## mint-release-tool-5-6 | approved

### Task mint-release-tool-5-6: Fresh source — re-diff vX-1..vX + AI notes

**Problem**: The fresh path regenerates genuinely better notes by re-diffing a historical release range and re-running the AI — and it is the first consumer that actually *exercises* the strategy-aware `version_file` exclusion rule (built in 3-10) on a range that contains mint's own bookkeeping commit. It must reproduce the forward path's *source* view from a tag range using path-based exclusion alone.

**Solution**: For the fresh source, assemble the `vX-1..vX` diff with the SAME `diff_exclude` tiers as the forward path — built-in `CHANGELOG.md` always-exclude, the configurable `diff_exclude` globs, and the strategy-aware `version_file` exclusion (plain mode excludes the file; embedded mode does not) — compute the Change Map, and run the AI engine. The oldest release (first-release marker from 5-3) skips all of this and emits the fixed "Initial release." body.

**Outcome**: A fresh regenerate of a normal version produces AI notes from the `vX-1..vX` diff with the forward path's exact exclusion view (bookkeeping churn excluded by path); the oldest release emits "Initial release." with no AI.

**Do**:
- Reuse the Phase 2 diff context assembly + AI transport and the Phase 2/3 Change Map, but drive them with the `vX-1..vX` range from 5-3 instead of `last_tag..HEAD`. The assembly already supports "an arbitrary range" (Phase 4 note); pass the regenerate range.
- Apply the exclusion tiers via git `:(exclude)` pathspec exactly as the forward path:
  - Built-in always-exclude `CHANGELOG.md`.
  - Configurable `diff_exclude` globs.
  - Strategy-aware `version_file` (the 3-10 rule, now actually exercised on this path): **plain mode** (whole file is the version, e.g. `release.txt`) → **exclude** the version_file; **embedded mode** (`version_pattern` in a real source file) → do **NOT** exclude (it's source; the lone version-line bump is neutralised by the prompt's "ignore version bumps" instruction). No `version_file` configured → nothing extra excluded.
- Critically: the `vX-1..vX` range *already contains* mint's release-bookkeeping commit (`{commit_prefix} Release {tag}`). Exclusion is **path-based, not commit-based** — a range diff cannot subtract a commit even though it carries a recognisable prefix. Path exclusion (CHANGELOG.md + version_file-in-plain-mode) is precisely what reproduces the forward path's *source* view here. Do not attempt to drop the commit.
- Compute the Change Map *after* exclusion (same ordering as forward), prepend to the AI input.
- Run the AI transport; honour `max_diff_lines` and validation/retry as on the forward path. (Failure routing: single-mode fresh follows `on_notes_failure` default abort; the `--all` skip-and-continue override is task 5-12 — keep the failure surfaced so 5-12 can intercept.)
- Oldest release (first-release marker) → emit the fixed body "Initial release.", no AI, no diff. (Mirrors the forward first-release rule.)
- All git through CommandRunner; AI through the fake `ai_command`; tested with FakeRunner + RecordingPresenter.

**Acceptance Criteria**:
- [ ] Fresh notes are generated from the `vX-1..vX` range, not `last_tag..HEAD`
- [ ] `CHANGELOG.md` is always excluded from the regenerate diff
- [ ] Plain-mode `version_file` is excluded; embedded-mode `version_file` is NOT excluded
- [ ] Exclusion is path-based — the bookkeeping commit inside the range is not subtracted as a commit
- [ ] The Change Map is computed after exclusion and prepended to the AI input
- [ ] The oldest release emits "Initial release." with no AI and no diff
- [ ] `max_diff_lines` and AI validation/retry behave as on the forward path

**Tests**:
- `"it assembles the vX-1..vX diff and runs the AI"`
- `"it always excludes CHANGELOG.md from the regenerate diff"`
- `"it excludes the version_file in plain mode"`
- `"it does NOT exclude the version_file in embedded mode"`
- `"it excludes by path even though the range contains the bookkeeping commit"`
- `"it prepends the Change Map computed after exclusion"`
- `"it emits Initial release. with no AI for the oldest release"`

**Edge Cases**:
- Strategy-aware `version_file` exclusion (plain excludes, embedded doesn't)
- `CHANGELOG.md` always-excluded
- Range contains bookkeeping commit (path exclusion, not commit)
- Change Map reused
- Oldest release fixed body

**Context**:
> "fresh (default) — re-diff `vX-1..vX` (with the same `diff_exclude` tiers + Change Map as the forward path) and re-run the AI for genuinely better notes."
> "Regenerate, plain mode (whole file is the version, e.g. `release.txt`): exclude the file — pure bookkeeping. Regenerate, embedded mode (`version_pattern` in a real source file like `main.go`): do not exclude — it's source we want in notes."
> "Exclusion is path-based, never commit-based. … regenerate diffs a tag range (`vX-1..vX`) that already contains them. A git range diff operates on paths/content and cannot subtract commits, so even though mint's bookkeeping commit carries a recognisable `commit_prefix` (cosmetic only), it cannot be dropped from the range — path exclusion is what reproduces the forward path's source view on the regenerate path."
> Oldest release → first-release rule (no AI, "Initial release.").

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Stage 4 — AI Release Notes" (Diff exclusion two tiers + strategy-aware version file) and "Regenerate / Backfill Notes" (Two-axis contract; Version argument & diff base resolution).

## mint-release-tool-5-7 | approved

### Task mint-release-tool-5-7: Provider release create-or-update probe

**Problem**: Regenerate's release target serves two situations with one command — "refresh existing release text" (update) and "mass-heal a missing release" (create). The user must never have to pick; mint must probe per version and dispatch the right Publisher call, so an `--all` batch transparently mixes updates and creates.

**Solution**: For a `release`/`both` target, probe whether a provider release already exists at `{tag}` and dispatch `UpdateRelease` if it does, `CreateRelease` if it doesn't — behind the existing `Publisher` interface (both methods already built in Phases 1/4). Resolution is per version, so each version in a batch is independently an update or a create.

**Outcome**: An existing provider release at `{tag}` is updated; an absent one is created; the decision is made per version with no user input, behind the Publisher interface.

**Do**:
- Add a probe to the Publisher seam (e.g. `ReleaseExists(tag) (bool, error)` or reuse a get/lookup) — for the GitHub driver, query via `gh` whether a release exists at `{tag}`. Run through CommandRunner.
- Dispatch: exists → `UpdateRelease(tag, body)`; absent → `CreateRelease(tag, body)`. Both methods already exist on the Publisher interface (4-9/4-10).
- Resolve **per version** so an `--all` batch (5-11) calls the probe for each version and transparently mixes updates and creates. The single-version path (5-9) uses the same probe.
- The user never selects create vs update — it is always derived from the probe.
- Keep this behind the Publisher interface; no GitHub specifics leak into the regenerate orchestration. Tested with a fake Publisher (and FakeRunner for the gh probe) asserting which method was dispatched.

**Acceptance Criteria**:
- [ ] A probe determines whether a provider release exists at `{tag}`
- [ ] Existing release → `UpdateRelease` dispatched
- [ ] Absent release → `CreateRelease` dispatched
- [ ] The decision is made per version (works independently across a batch)
- [ ] The probe + dispatch live behind the Publisher interface (no driver specifics in orchestration)
- [ ] The user is never prompted to choose create vs update

**Tests**:
- `"it dispatches UpdateRelease when a release exists at the tag"`
- `"it dispatches CreateRelease when no release exists at the tag"`
- `"it resolves create-vs-update per version"`
- `"it surfaces a probe error"`
- `"it keeps create-or-update behind the Publisher interface"`

**Edge Cases**:
- Release exists → `UpdateRelease`
- Release absent → `CreateRelease`
- Resolved per version
- Behind Publisher interface

**Context**:
> "Provider target: create-or-update is automatic. For `--target release`/`both`, mint probes whether a provider release exists at `{tag}` and dispatches `UpdateRelease` if it does, `CreateRelease` if it doesn't. The user never picks — 'refresh existing text' and 'mass-heal missing releases' are the same command, resolved per version (so an `--all` batch transparently mixes updates and creates)."
> Publisher abstraction: "Behind a small `Publisher` interface (`CreateRelease` / `UpdateRelease`)."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Write path — Provider target) and "Stages 6–7 — Tag, Push & Publish" (Publishing: provider driver abstraction).

## mint-release-tool-5-8 | approved

### Task mint-release-tool-5-8: Single-version changelog write (in-place idempotent replace)

**Problem**: A single-version regenerate that targets the changelog must replace that one version's section in place (not append a duplicate, not rebuild the whole file), commit it with a regenerate-specific subject (not the forward release subject), make no empty commit when nothing changed, and never cut a tag.

**Solution**: For a single-version `changelog`/`both` regenerate, write the new body into `CHANGELOG.md` using the Phase 1/3 idempotent in-place section-replace by version key, and stage at most ONE CHANGELOG commit with subject `docs(changelog): regenerate notes for {tag}`. If the replace yields no net change, make no commit. No tag is ever created.

**Outcome**: The target version's `## [x.y.z] - date` section is replaced in place with the regenerated body; a single commit `docs(changelog): regenerate notes for {tag}` is staged; an unchanged file produces no empty commit; no tag is cut.

**Do**:
- Reuse the Phase 1/3 changelog writer's single-version in-place section-replace by version key (KaC format, newest-on-top, created-if-absent) — the same idempotent replace the forward path uses when re-recording an existing version.
- Write the regenerated body under that version's `## [x.y.z] - date` header. (Whole-file rebuild is the `--all` variant, task 5-13 — NOT this task.)
- Commit subject: `docs(changelog): regenerate notes for {tag}` — explicitly NOT the forward `{commit_prefix} Release {tag}` subject. Nothing is being released.
- At most ONE CHANGELOG commit. No hook-artifact commit (hooks do not run on regenerate).
- No-op safety: if the in-place replace produces no net change to the file (the regenerated body is byte-identical to what's there), make no commit (no empty commit).
- No tag is ever cut — regenerate touches only the mutable surface.
- This task owns building/staging the changelog commit; the push, recovery, and provider sequencing are task 5-9.
- All git through CommandRunner; tested with FakeRunner + RecordingPresenter.

**Acceptance Criteria**:
- [ ] An existing version section is replaced in place (not duplicated, not appended)
- [ ] The commit subject is `docs(changelog): regenerate notes for {tag}`
- [ ] At most one CHANGELOG commit is staged
- [ ] A no-net-change write produces no commit (no empty commit)
- [ ] No tag is cut

**Tests**:
- `"it replaces the existing version section in place"`
- `"it uses the subject docs(changelog): regenerate notes for {tag}"`
- `"it does not reuse the forward release commit subject"`
- `"it makes no commit when the changelog content is unchanged"`
- `"it cuts no tag"`

**Edge Cases**:
- Existing version section → replaced in place
- Subject `docs(changelog): regenerate notes for {tag}`
- No net change → no empty commit
- No tag cut

**Context**:
> "Commit graph (changelog target). A `--target changelog`/`both` run writes at most one CHANGELOG commit (hooks don't run on regenerate, so there is no hook-artifact commit). Subject: `docs(changelog): regenerate notes for {tag}` for a single version … It does not reuse the forward `{commit_prefix} Release {tag}` subject — nothing is being released."
> "A single-version regenerate uses the idempotent in-place section replace (per Stage 5)."
> Stage 5: "Idempotent by version key — writing an existing version's section replaces it in place rather than appending a duplicate. This applies on the regenerate path." "No empty commits — if the changelog yields no net change … mint skips that commit."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Write path) and "Stage 5 — Record (Changelog & Version Recording)".

## mint-release-tool-5-9 | approved

### Task mint-release-tool-5-9: Single-version write, push & recovery

**Problem**: The single-version regenerate write needs its own point-of-no-return model: a plain (no-tag) push, reset-on-abort recovery lighter than the forward surgical unwind, warn-only on a post-push provider failure, a non-atomic `both` ordering, and the right confirm/gate behaviour per source.

**Solution**: Wire the single-version write/push/recovery. Push form is plain `git push origin HEAD` (no tag). The changelog commit's `git push` is the point of no return: abort at the review gate (fresh) or any pre-push failure resets the local CHANGELOG commit; a provider create/update failure AFTER the changelog push warns only. `--target both` writes changelog (commit + push) FIRST, then the provider release (non-atomic). Fresh runs the notes-review gate before writing; reuse is a simple confirm (no review gate); `-y` skips the confirm.

**Outcome**: A single-version regenerate pushes the changelog with a plain push, resets the commit cleanly on pre-push abort/failure, warns only on a post-push provider failure, sequences `both` changelog-then-provider non-atomically, and gates correctly by source.

**Do**:
- Push form: plain `git push origin HEAD` — NOT the forward `--atomic origin HEAD {tag}` (no tag is involved). Through the lock-resilient git wrapper (4-1).
- Point of no return = the changelog commit's `git push`. Before it everything is local.
- Recovery (pre-PONR): abort at the review gate (fresh) OR any pre-push failure → reset the local CHANGELOG commit, returning to the clean starting state. This is a *lighter unwind than the forward surgical unwind* — just the commit reset, no tag involved. (Reuse-only and release-only-fresh runs make no commit, so there is nothing to reset.)
- Post-PONR: a provider create/update failure AFTER the changelog push → warn only (the changelog is already pushed; re-heal the provider with `--target release`). Same post-PONR principle as the forward path; never unwind.
- `--target both` is NOT atomic across surfaces: write the changelog (commit + push) FIRST, then the provider release. A provider failure after the changelog push is the warn-only case above, not a rollback.
- Gating by source: fresh runs the notes-review gate (Phase 2 `y`/`n`/`e`/`r`) before writing — backfilled notes reviewable before they overwrite live surfaces; answering abort routes to the commit-reset recovery. reuse is a *simple confirm* (deterministic, no new notes → no review gate). `-y` skips the confirm/gate in both cases. (The full interactive prompt orchestration is 5-10; this task wires the gate/confirm into the write path and the recovery routing.)
- Order: gate/confirm → (fresh writes changelog commit) → push (PONR) → provider write (probe via 5-7).
- All git through the lock-resilient CommandRunner wrapper; tested with FakeRunner + RecordingPresenter + fake Publisher.

**Acceptance Criteria**:
- [ ] The push is plain `git push origin HEAD` (no tag, not `--atomic … {tag}`)
- [ ] A gate abort (fresh) resets the local CHANGELOG commit to the clean starting state
- [ ] Any pre-push failure resets the local CHANGELOG commit
- [ ] A provider failure after the changelog push warns only (no reset)
- [ ] `--target both` writes the changelog (commit + push) first, then the provider release (non-atomic)
- [ ] reuse is a simple confirm (no review gate); fresh runs the notes-review gate; `-y` skips the confirm

**Tests**:
- `"it pushes with plain git push origin HEAD (no tag)"`
- `"it resets the changelog commit when the user aborts at the review gate"`
- `"it resets the changelog commit on a pre-push failure"`
- `"it warns only when the provider write fails after the changelog push"`
- `"it writes changelog then provider for --target both (non-atomic)"`
- `"it runs the notes-review gate for fresh and a simple confirm for reuse"`
- `"it skips the confirm with -y"`

**Edge Cases**:
- Gate abort → reset commit
- Pre-push failure → reset commit
- Provider failure after changelog push → warn only
- `--target both` writes changelog then provider (non-atomic)
- reuse → simple confirm, no gate
- `-y` skips confirm

**Context**:
> "Push form. Plain `git push origin HEAD` (no tag, so not the forward path's `--atomic … {tag}`)."
> "Point of no return & recovery. The changelog commit's `git push` is the point of no return … Abort at the review gate (fresh) or any pre-push failure → mint resets the local CHANGELOG commit, returning to the clean starting state (no tag is involved, so the unwind is just the commit reset). A provider create/update failure after the changelog push → warn only … `--target both` is not atomic across surfaces: mint writes the changelog (commit + push) first, then the provider release; a provider failure after the changelog push is the warn-only case above, not a rollback."
> "fresh regeneration runs the same notes-review gate before writing … reuse is deterministic (no new notes) → a simple confirm, no review gate. Flags skip the questions but still confirm unless `-y`."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Write path — commit, push & recovery; Interactive by default).

## mint-release-tool-5-10 | approved

### Task mint-release-tool-5-10: Interactive default flow (source/target prompts + plan + confirm)

**Problem**: With no flags, regenerate must be fully interactive — asking for the source, asking for the target, showing the plan, and confirming — while flags skip the relevant questions but still confirm unless `-y`. The fresh path runs the notes-review gate; reuse is a simple confirm.

**Solution**: Implement the interactive default flow through the Presenter: no flags → ask source (reuse/fresh), ask target (release/changelog/both), show the plan summary, confirm. Supplied flags skip the corresponding questions but the run still confirms unless `-y`. The fresh path runs the notes-review gate before writing; reuse is a simple confirm only.

**Outcome**: A bare `regenerate <version>` walks the user through source → target → plan → confirm; flags pre-fill answers and skip those prompts; `-y` skips the confirm; fresh shows the review gate, reuse shows a simple confirm.

**Do**:
- Drive prompts through the Presenter interface (recording fake in tests); the concrete rendering is the CLI Presentation spec's concern.
- No `--reuse`/`--fresh` → ask the source. No `--target` → ask the target (subject to the axis contract from 5-2; e.g. a reuse answer forces release). A supplied flag skips its question.
- After source + target are known, show a plan summary (which version(s), source, target surface(s), create-vs-update is resolved later per version) and confirm.
- `-y` skips both the prompts' confirm AND, for fresh, the notes-review gate (consistent with the forward `-y`). Flags-without-`-y` skip the *questions* but still confirm.
- fresh → run the notes-review gate (`y`/`n`/`e`/`r`, Phase 2) before writing (wired in 5-9); reuse → a simple confirm (no review gate).
- Respect the 5-2 contract: when interactive, a fresh run without `--target` is resolved by the target *prompt* (not the `--fresh -y` error, which only fires when `-y` is set and target is unset).
- Tested with RecordingPresenter asserting the prompt sequence and FakeRunner.

**Acceptance Criteria**:
- [ ] No flags → asks source, asks target, shows plan, confirms
- [ ] A supplied `--reuse`/`--fresh` skips the source question; a supplied `--target` skips the target question
- [ ] Flags without `-y` still confirm
- [ ] `-y` skips the confirm
- [ ] fresh runs the notes-review gate before writing; reuse runs a simple confirm only

**Tests**:
- `"it asks source then target then shows plan then confirms with no flags"`
- `"it skips the source question when --reuse or --fresh is given"`
- `"it skips the target question when --target is given"`
- `"it still confirms when flags are given without -y"`
- `"it skips the confirm with -y"`
- `"it runs the notes-review gate for fresh and a simple confirm for reuse"`

**Edge Cases**:
- fresh runs notes-review gate before write
- reuse → simple confirm only
- flags skip questions, still confirm
- `-y` skips confirm

**Context**:
> "`mint release regenerate <ver>` with no flags → interactive: asks source, asks target, shows the plan, confirms."
> "fresh regeneration runs the same notes-review gate (see Interactive Review) before writing — backfilled notes are reviewable before they overwrite live surfaces. reuse is deterministic (no new notes) → a simple confirm, no review gate. Flags skip the questions but still confirm unless `-y`."
> Gate rendering is owned by the CLI Presentation spec (cross-spec dependency); the engine drives it through the Presenter interface (recording fake in tests).

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Interactive by default, flags to skip) and "Interactive Confirmation & Notes Review".

## mint-release-tool-5-11 | approved

### Task mint-release-tool-5-11: Batch --all single-version regeneration loop

**Problem**: `regenerate --all` backfills every version in a repo. It must iterate oldest→newest (so the changelog can be rebuilt in natural order), generate notes per version, gate per version by default (`-y` opts out for unattended runs), and resolve create-vs-update per version — with no resume state so it is freely re-runnable.

**Solution**: Implement the `--all` loop: enumerate every matching version oldest→newest, and for each version run the per-version notes generation (reuse read or fresh diff+AI), the per-version review gate by default (`-y` opts out), and the create-or-update probe (5-7). The loop holds no resume state — it is re-runnable.

**Outcome**: `regenerate --all` processes every version oldest→newest, generating and (by default) reviewing notes per version, with create-vs-update resolved per version and no persisted resume state.

**Do**:
- Enumerate matching versions (the global SemVer-sorted tag set) and order oldest→newest.
- For each version, run the source path: reuse → read the tag body (5-5); fresh → resolve base (5-3, including the oldest-version first-release rule) and re-diff+AI (5-6).
- Per-version review gate by default (fresh shows the notes-review gate; reuse a simple confirm) — consistent with "notes never go out unseen". `-y` opts out to run fully unattended (no per-version gate/confirm).
- Resolve create-vs-update per version via the probe (5-7) so the batch transparently mixes updates and creates.
- No resume state / no checkpoint file — a re-run simply re-processes. `--reuse --all` is fully deterministic; `--fresh --all` re-generates (stochastic but harmless).
- This task owns the loop + per-version processing for the `release` surface and per-version notes; the skip-and-continue + summary is 5-12 and the whole-file changelog rebuild + single end commit is 5-13. Structure the loop so 5-12 wraps each iteration in skip-and-continue and 5-13 collects per-version bodies for the end-of-batch changelog write.
- Tested with FakeRunner + RecordingPresenter + fake Publisher; assert ordering and per-version gate/probe.

**Acceptance Criteria**:
- [ ] Versions are processed oldest → newest
- [ ] Notes are generated per version (reuse read or fresh diff+AI)
- [ ] A per-version review gate runs by default; `-y` opts out to run unattended
- [ ] Create-vs-update is resolved per version (mixed across the batch)
- [ ] The loop holds no resume state and is re-runnable

**Tests**:
- `"it processes versions oldest to newest"`
- `"it generates notes per version"`
- `"it runs a per-version review gate by default"`
- `"it runs unattended with -y (no per-version gate)"`
- `"it mixes update and create across the batch per version"`
- `"it is re-runnable with no resume state"`

**Edge Cases**:
- ordering oldest→newest
- per-version review gates
- `-y` opts out
- mixed update/create across batch
- re-runnable, no resume state

**Context**:
> "Ordering: oldest → newest (lets mint rebuild `CHANGELOG.md` in natural order)."
> "Review gates per version by default (consistent with 'notes never go out unseen'); `-y` is the opt-out to run fully unattended."
> "Re-runnable, no resume state. `--reuse --all` (mass-heal from tags) is fully deterministic; `--fresh --all` re-generates (stochastic but harmless)."
> "create-or-update resolved per version (mixed updates/creates)."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Batch `--all` semantics).

## mint-release-tool-5-12 | approved

### Task mint-release-tool-5-12: Batch --all skip-and-continue & end summary

**Problem**: A batch backfill must not be killed by one bad release (e.g. a single huge release tripping `max_diff_lines`). Per-version failures must skip-and-continue with an end summary — consciously overriding the single-version `on_notes_failure = abort` default — while truly static config-level errors must abort the whole batch up front before any work.

**Solution**: Wrap each `--all` iteration in skip-and-continue: a per-VERSION failure (notes failure, diff-too-large, or a reuse tag with no annotation body) is recorded and skipped, the batch continues, and an end summary reports counts + skipped versions with reasons. CONFIG-level errors (e.g. `--target changelog`/`both` with `changelog = false`) are validated UP FRONT and abort the whole batch before it starts.

**Outcome**: One failing version is skipped and reported while the rest complete; `--reuse --all` against a body-less tag is skipped+reported (not the single-mode error); a config-level error aborts the whole batch up front; the run ends with a summary like "27 regenerated, 3 skipped: vX (diff too large), …".

**Do**:
- Per-version skip-and-continue: wrap each iteration (5-11) so a per-version failure is caught, recorded (version + reason), and the loop continues. This consciously OVERRIDES the single-version `on_notes_failure = abort` default for the batch — a per-version notes failure does not abort the batch.
  - Covered per-version failures include: notes generation failure / diff exceeds `max_diff_lines` ("diff too large"); and `--reuse --all` against a tag with no annotation body → skipped+reported (the `--all` variant of the 5-5 single-mode error — call the same `readTagBody` and branch on empty: skip, don't error).
- Config-level errors are a STATIC fact, not a per-version condition → validate UP FRONT (reuse the 5-2 contract validator, e.g. `--target changelog`/`both` with `changelog = false`) and abort the WHOLE batch BEFORE it starts. Do not skip these per version.
- End summary: emit a summary through the Presenter, e.g. "27 regenerated, 3 skipped: vX (diff too large), vY (no annotation body — use --fresh), …" — listing each skipped version with its reason so the user can re-run stragglers.
- Skip-and-continue covers per-version failures only; the up-front config abort is the sole "whole batch dies before it starts" case.
- Tested with FakeRunner + RecordingPresenter: a batch where one version fails completes the rest and reports; a config error aborts before any version is processed.

**Acceptance Criteria**:
- [ ] A per-version failure (e.g. diff too large) is skipped and the batch continues
- [ ] `--reuse --all` against a tag with no annotation body is skipped+reported (not the single-mode error)
- [ ] Per-version skip-and-continue overrides the single-version `on_notes_failure = abort` default
- [ ] A config-level error (`--target changelog`/`both` with `changelog = false`) aborts the whole batch up front, before any version is processed
- [ ] The end summary lists counts and each skipped version with its reason

**Tests**:
- `"it skips a version whose diff is too large and continues the batch"`
- `"it skips and reports a reuse --all tag with no annotation body"`
- `"it overrides on_notes_failure=abort for per-version failures"`
- `"it aborts the whole batch up front on a config-level error"`
- `"it emits an end summary listing regenerated count and skipped versions with reasons"`

**Edge Cases**:
- per-version diff-too-large skipped + reported
- `--reuse --all` missing annotation → skip + report
- config error (changelog=false target) aborts whole batch up front
- end summary lists skipped versions + reasons
- overrides `on_notes_failure=abort`

**Context**:
> "Partial failure: skip-and-continue, summarise at the end — not abort-the-batch (a single huge release tripping `max_diff_lines` shouldn't kill the others). This consciously overrides the single-version `on_notes_failure = abort` default; mint reports e.g. '27 regenerated, 3 skipped: vX (diff too large), …' so the user re-runs the stragglers. Skip-and-continue covers per-version failures only; config-level errors are validated up front and abort the whole batch before it starts — e.g. `--target changelog`/`both` with `changelog = false` (a static config fact, not a per-version condition) fails immediately rather than being skipped per version."
> "`--reuse` with no annotation body … in `--all` mode it is skipped and reported (consistent with batch skip-and-continue), never written as an empty release body."

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Batch `--all` semantics; Version argument & diff base resolution).

## mint-release-tool-5-13 | approved

### Task mint-release-tool-5-13: Batch --all whole-file CHANGELOG rebuild (one commit at end)

**Problem**: A batch `--all` that targets the changelog should produce a clean, correctly ordered file in one commit — not N noisy in-place edits. mint must rebuild `CHANGELOG.md` whole (preamble + every version's section, newest-on-top, repairing ordering/stray-section drift), commit once at the end with the `--all` subject, and make no changelog commit at all for a release-only `--all`.

**Solution**: For an `--all` `changelog`/`both` run, after all per-version notes are generated/reviewed (5-11/5-12), rebuild `CHANGELOG.md` WHOLE from the KaC preamble + every version's section newest-on-top, and make ONE CHANGELOG commit + push at the END with subject `docs(changelog): regenerate release notes`. An `--all` `--target release` run makes no changelog commit.

**Outcome**: An `--all` changelog/both run produces a fully rebuilt, correctly ordered `CHANGELOG.md` in a single end-of-batch commit (`docs(changelog): regenerate release notes`); an `--all` release-only run writes no changelog.

**Do**:
- Whole-file rebuild (NOT in-place): construct `CHANGELOG.md` from the Keep a Changelog preamble followed by every version's `## [x.y.z] - date` section in newest-on-top order, using the per-version bodies collected during the loop (5-11/5-12). This repairs ordering and removes stray-section drift — which is what "rebuild the changelog" means. (Contrast: single-version regenerate uses the idempotent in-place replace, task 5-8.)
- Only versions that were successfully regenerated contribute their (new) section; skipped versions (5-12) — decide and document: a stray/legacy section for a skipped version is dropped by a whole-file rebuild from successfully-processed versions. (See ambiguity note.)
- ONE CHANGELOG commit + push at the END — after all per-version notes are generated/reviewed — not one per version. Avoids N noisy commits for a large backfill.
- Commit subject: `docs(changelog): regenerate release notes` (the `--all` form) — distinct from the single-version `docs(changelog): regenerate notes for {tag}` (5-8).
- Push: the same plain `git push origin HEAD` PONR + reset-on-abort recovery as 5-9, but executed ONCE at the end of the batch.
- An `--all` `--target release` run makes NO changelog commit (release-only touches only provider releases).
- No-op safety: if the rebuilt file is byte-identical to the existing one, make no commit.
- Tested with FakeRunner + RecordingPresenter: assert whole-file rebuild content/order, a single end commit with the `--all` subject, and zero changelog commit for release-only.

**Acceptance Criteria**:
- [ ] An `--all` changelog/both run rebuilds `CHANGELOG.md` whole (preamble + every section, newest-on-top), not in-place
- [ ] The rebuild repairs ordering / removes stray-section drift
- [ ] Exactly one CHANGELOG commit + push is made, at the end of the batch
- [ ] The commit subject is `docs(changelog): regenerate release notes` (the `--all` form)
- [ ] An `--all` `--target release` run makes no changelog commit
- [ ] A byte-identical rebuild produces no commit

**Tests**:
- `"it rebuilds CHANGELOG.md whole newest-on-top for --all changelog"`
- `"it repairs ordering and drops stray sections via whole-file rebuild"`
- `"it makes exactly one changelog commit at the end of the batch"`
- `"it uses the subject docs(changelog): regenerate release notes for --all"`
- `"it makes no changelog commit for an --all --target release run"`
- `"it makes no commit when the rebuilt file is unchanged"`

**Edge Cases**:
- whole-file rebuild (not in-place) repairs ordering/stray-section drift
- natural-order rebuild
- one commit+push after all versions reviewed
- `--all` release-only target makes no changelog commit

**Context**:
> "`--all` rebuild strategy. An `--all` `--target changelog`/`both` run rebuilds `CHANGELOG.md` whole — regenerating the file from the Keep a Changelog preamble + every version's section, newest-on-top — rather than editing in place (whole-file rebuild is what 'rebuild the changelog' means, and it repairs ordering/stray-section drift). A single-version regenerate uses the idempotent in-place section replace (per Stage 5). The batch makes one CHANGELOG commit + push at the end (after all per-version notes are generated/reviewed), not one per version — avoiding N noisy commits for a large backfill."
> "Subject … `docs(changelog): regenerate release notes` for an `--all` rebuild."
> Ambiguity note: the spec says the rebuild is from "every version's section" but does not state how a version *skipped* in the same batch (5-12) is represented. A whole-file rebuild from successfully-processed versions naturally omits skipped ones; the executor should confirm whether a skipped version's prior section should be preserved or dropped — the task assumes drop (clean rebuild) and surfaces this for resolution.

**Spec Reference**: `.workflows/mint-release-tool/specification/mint-release-tool/specification.md` — "Regenerate / Backfill Notes" (Write path — `--all` rebuild strategy; Batch `--all` semantics) and "Stage 5 — Record (Changelog mechanics)".
