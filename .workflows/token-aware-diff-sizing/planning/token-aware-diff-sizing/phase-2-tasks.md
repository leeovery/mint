# Phase 2: Per-call size cap and chunked release notes — 10 tasks

## token-aware-diff-sizing-2-1

### Task 2-1: `max_prompt_bytes` config key

**Problem**: mint has no byte measure of what an AI call carries. Its one size setting, `max_diff_lines`, counts lines, and one minified or generated line can be any size, so a diff under the line ceiling can still overflow the model (§1.1). Every size decision this phase adds reads one per-call byte cap: when release skips the single call, how big each chunk is, when commit's editor opens, and when the Change Map condenses (§4.2, §4.3, §6.3).

**Solution**: Add `max_prompt_bytes` at the shared top level only. It is an integer count of bytes, defaulting to 1,500,000, that bounds the composed prompt of any single AI call. It takes the strict-config path: a schema entry, a friendly type error, a config-metadata row and the README tables. A value of zero or less is rejected when config loads.

**Outcome**: Every run resolves a positive per-call byte cap, 1,500,000 when unset. A bad value stops every command at load with a message naming the key. The README, `mint setup`'s config reference and CLAUDE.md all show the key as shared-only.

**Acceptance Criteria**:
- [ ] Given no `.mint.toml`, or one that does not set `max_prompt_bytes`, config loads with a cap of 1,500,000 bytes. Given `max_prompt_bytes = 800000` at the top level, the cap is 800,000 bytes (§4.1)
- [ ] Given `max_prompt_bytes = 0` or `max_prompt_bytes = -1`, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: max_prompt_bytes must be a positive number of bytes`. Zero never means "no cap" (§4.1)
- [ ] Given a non-integer `max_prompt_bytes`, such as `"1500000"` or `1.5`, loading fails with a friendly type error in the `invalid .mint.toml: …` shape that names `max_prompt_bytes` and its integer type, as the other typed keys' errors do (§4.1)
- [ ] Given `max_prompt_bytes` under `[release]` or `[commit]`, loading fails with the as-built unknown-key message naming the key and its table (§4.1)
- [ ] `mint setup`'s config reference lists `max_prompt_bytes` at the shared top level with its 1,500,000 default, described as a cap, in bytes, on the composed prompt of any single AI call (§4.1, §18.1)
- [ ] The README's shared engine keys table lists `max_prompt_bytes` with its 1,500,000 default and the same meaning (§18.2)
- [ ] CLAUDE.md's config seam names `max_prompt_bytes` as shared-only at the top level (§18.3)

**Do**:
- The key takes the strict-config path CLAUDE.md's config seam names, in `internal/config`: a field in the top-level decode shape (`fileShape` in `config.go`), a `typeErrorMessages` entry, and a `config.MetadataRows()` row (`metadata.go`). The README row goes in its shared engine keys table (§4.1, §18.2).
- The positive-value check runs in `config.Load`, beside the load-time checks behind the other `invalid .mint.toml: …` rejections (§4.1).

**Context**:
> §4.1: the cap bounds the composed prompt mint sends in any single AI call, measured in bytes: instructions, Change Map, diff and closing reminder together. The AI tool's own overhead is invisible to mint and sits outside the cap. No call mint makes is ever bigger than the cap. Zero never means "no cap", unlike `timeout = 0`, which means no deadline: the cap sizes every chunk and keeps every call under claude's stdin limit. An operator who wants a cap mint never reaches sets a large one.
>
> §4.5: on the lean command, diff content runs about 2.2 bytes per token, so 1,500,000 bytes is about 675k tokens. That stays under a 1,000,000-token window even at about 1.8 bytes per token, and far below claude's 10 MB stdin limit. The default leans high: too high for the model is loud and corrects itself, while too low is silent.
>
> One cap covers both verbs; regenerate rides on it with release. Its consumers land later in this phase: commit's editor fallback (task 2-2), the Change Map's share (task 2-3), and release's size check and chunk sizing (task 2-4). `max_diff_lines` keeps loading until task 2-5 removes it.
>
> As built, the shared scalar defaults in `MetadataRows()` are sourced from exported config constants and pinned in `metadata_test.go`, and the zero-config path (no `.mint.toml`) takes its values from `defaults()` in `config.go`. The drift test fails until the metadata SoT and the decode shape biject over `(level, key)`. The README tripwire test (`internal/config/readme_tripwire_test.go`) fails until every schema key name appears in the README. The `internal/initgen` template carries no per-key entries, so the key is not added there (CLAUDE.md config seam).
>
> CLAUDE.md's config seam reads "`max_diff_lines`/`diff_exclude` stay shared-only at the top level". Task 2-5 drops `max_diff_lines` from that line, and Phase 7 gives `diff_exclude` its per-verb levels.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §4.1 The key; §4.2 What the cap does; §4.5 The default's grounds; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-2-2

### Task 2-2: Commit's per-call cap

**Problem**: `mint commit` decides whether to call the AI by counting the post-exclusion diff's lines against `max_diff_lines`. A line count says little about what the AI receives (§1.1), and it ignores the instructions and closing reminder that ride in the same prompt. Commit has no chunking, so its cap is the one thing that keeps an oversized prompt from being sent (§4.1, §4.3). That includes `r` at the review gate, where a nudge can push a regeneration over a cap the first generation fit under.

**Solution**: Measure commit's composed prompt, its instructions, the would-be-committed diff and the closing reminder together, in bytes against `max_prompt_bytes`. That holds on the first generation and on every `r` regeneration. A prompt over the cap is never sent: the editor opens empty with the existing `diff too large to summarise — opening editor` note, as an over-limit diff does today. The line count no longer applies to commit. The README's editor-fallback line names the new trigger.

**Outcome**: A commit whose composed prompt fits the cap gets an AI message however many lines its diff has. One over the cap, first time or after a nudge, goes to the editor without an AI call, or fails loud when unattended, exactly as an over-limit diff does today.

**Acceptance Criteria**:
- [ ] Given no `max_prompt_bytes` set and a would-be-committed diff of more than 50,000 lines whose composed prompt is at most 1,500,000 bytes, `mint commit` makes the AI call and shows the generated message at the review gate (§4.1, §4.3)
- [ ] Given `max_prompt_bytes` set to exactly the composed prompt's size, the AI call is made. Set one byte lower, no AI call is made and the editor fallback applies (§4.1, §4.3)
- [ ] Given an attended `mint commit` whose composed prompt is over the cap, no AI call is made. The run shows `diff too large to summarise — opening editor`, then opens the editor on an empty buffer, and the save becomes the accept as built (§4.3)
- [ ] Given a diff that alone is under the cap but whose composed prompt goes over it, for example through an outsized `[commit].context`, no AI call is made and the editor opens with the same note (§4.1, §4.3)
- [ ] Given an over-cap prompt on an unattended run (`-y`, or a non-TTY stdin), no AI call is made. The run fails with `no AI message and no interactive editor available`, with no note before it and nothing beneath it (§4.3, §17.1)
- [ ] Given a message at the review gate whose first prompt fit the cap, when `r` with a nudge pushes the regeneration's composed prompt over the cap, no regeneration call is made. The run shows `diff too large to summarise — opening editor` and opens the editor empty, not on the prior message, as it does after a failed regeneration; the save becomes the accept (§4.3, Corrigendum 2026-10-19 on §4.3)
- [ ] The README's commit editor-fallback line names a composed prompt over `max_prompt_bytes` where it named a diff over `max_diff_lines` (§18)

**Do**:
- The byte measure replaces commit's line-count guard in its generator (`internal/commit/generate.go`), which the first generation and `r`'s regeneration both run (§4.3).
- An over-cap first generation takes the existing oversized editor-fallback branch in `Run`. An over-cap regeneration takes the route a failed regeneration takes to the empty editor fallback, out of `reviewLoop` and through `Run` (`internal/commit/run.go`) (§4.3, Corrigendum 2026-10-19 on §4.3).

**Context**:
> §4.1: no call mint makes is ever bigger than the cap. For commit the composed prompt is three parts: its instructions (the default prompt or the `[commit].prompt` override, with `[commit].context` and any `r` nudge), the would-be-committed post-exclusion diff, and the closing reminder (`ComposePrompt` in `internal/commit/prompt.go`).
>
> §1.3, §17.2: commit gets the size cap but no chunking, no failure classification, no exclusion check and no message file. Over the cap, the message is written in `$EDITOR` as today.
>
> §17.1: the oversized trigger carries no AI output, because no call was made, and its line is unchanged.
>
> The `r` case (Corrigendum 2026-10-19 on §4.3) is settled by the as-built routing of a failed commit regeneration to the empty editor fallback (`internal/commit/run.go:413`). As built, a regeneration that returns the size guard's error is not an AI transport failure, so `reviewLoop` surfaces it as an abort. Under the line count it could not arise, because `r` re-reads the same diff. `r` is interactive-only, so the over-cap `r` route is always attended.
>
> As built, commit consumes `notes.CheckDiffSize` / `notes.ErrDiffTooLarge` for its guard. An all-excluded diff is caught before the size guard and routes to the same editor fallback with its own note, unchanged. Task 2-5 removes `max_diff_lines` and whatever of the line guard is left unused; CLAUDE.md's layout line names the pair. The comments that describe commit's line-count guard (`internal/commit/generate.go`, `run.go`, `prompt.go`) go stale with this change (§18.3).

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §4.1 The key; §4.3 Per consumer; §17.1 The AI's output, verbatim; §17.2 What commit does not get; §18 Documentation and the Setup Guide; §18.3 CLAUDE.md and code comments; Corrigenda (2026-10-19, §4.3)

## token-aware-diff-sizing-2-3

### Task 2-3: Change Map condenses by folder over its share

**Problem**: The Change Map lists every removed and every renamed path on its own line. A release that deletes a vendored tree or moves thousands of files gets one `Removed:` or `Renamed:` line per path, enough to crowd out any diff (§8.2). Every call that carries the map would spend its room on file names: the single-pass call now, and the chunk and merge calls later in this phase.

**Solution**: Once the rendered Change Map would take more than a tenth of `max_prompt_bytes` (150,000 bytes at the default), condense its long runs of removed or renamed files by folder, one summary line per folder. The largest runs collapse first, until the map is under its share. A map under its share renders exactly as today. Every call that carries the map receives the condensed form, single-pass included.

**Outcome**: A big release's Change Map shrinks to summary lines per folder and leaves the diff its room. A release whose map already fits sees no change.

**Acceptance Criteria**:
- [ ] Given a release whose Change Map takes at most a tenth of the cap, the map every call carries is byte-identical to the as-built rendering (§6.3, §8.2)
- [ ] Given a release that removes 4,312 files under `vendor/`, whose map would take more than a tenth of the cap, the map carries `Removed: 4,312 files under vendor/` in place of those 4,312 `Removed:` lines (§8.2)
- [ ] Given a long run of renamed files under one folder in a map over its share, that run collapses to one summary line for that folder (§8.2)
- [ ] Given a map over its share with runs of different lengths, the largest collapse first and condensing stops once the map is under its share. For example, given a 4,312-file run under `vendor/` and a 12-file run under `docs/`, where collapsing `vendor/` alone brings the map under its share, the `docs/` files stay listed one per line (§6.3)
- [ ] Given the same release on two runs, the condensed map is byte-identical both times (§6.3)
- [ ] Given `max_prompt_bytes` set, the share is a tenth of the configured cap: a map that renders in full under the default condenses under a cap small enough that the map takes more than a tenth of it (§6.3)
- [ ] Given a release whose single-pass prompt is under the cap and whose map is over its share, the single-pass call carries the condensed map (§8.2)
- [ ] Given `mint release regenerate <version>` from a fresh source on a version whose map is over its share, its call carries the map condensed the same way (§8.2)

**Do**:
- The Change Map is rendered in `internal/notes/changemap.go` (`BuildChangeMapForRange`), which the forward release and regenerate's fresh source both build (§8.2).

**Context**:
> §6.3 sets one rule for both texts that condense by folder: this task's Change Map, and Phase 7's exclusion-check list, which reuses it.
> - A text condenses once it would take more than a tenth of the size cap, which is 150,000 bytes at the default.
> - Condensing replaces a long run of entries under one folder with one summary line for that folder, until the text is under its share.
> - A text under its share renders exactly as it does today.
> - The result must be deterministic for the same input and must collapse the largest runs first. Which folders collapse, and in what order, is otherwise the implementer's.
>
> §8.2: every chunk call and the merge call receive the same whole-release Change Map, so each chunk knows the whole release's shape and the merge can rank across all of it. The default prompt's salience rules depend on the map: one bullet per notable change, related changes combined, ranked using the Change Map. After condensing, the notes still learn the tree went, which is all they would say about it anyway.
>
> §8.7: the map can still leave a call no room for diff once condensed; task 2-9 handles that case.
>
> As built, the map's per-path lines are the `Removed:` and `Renamed:` entries under "Structural novelty". Its other sections roll up by leading directory, or name only top-level entries and the single largest file. Task 2-4 measures the single-pass prompt with this map in it and carries the map to the chunk and merge calls.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §6.3 Condensing by folder; §8.2 The Change Map in every call; §4.1 The key; §8.7 Calls that cannot be made

## token-aware-diff-sizing-2-4

### Task 2-4: Over-cap release notes: chunk calls and one merge

**Problem**: A release whose diff is too big for one call cannot get AI notes today. Over `max_diff_lines` it aborts as `diff too large`, with nothing pointing a way forward, and under it a dense diff can still overflow the model (§1.1). The goal is AI-written notes for a release of any size.

**Solution**: Measure the composed single-pass prompt (instructions, Change Map, diff and closing reminder) against `max_prompt_bytes`. At or under the cap, make the single call as built. Over it, skip the single call and write the notes by chunked generation:
- pack the diff's files in path order into chunk calls that each fit the cap
- each chunk call carries the release's instructions, the whole-release Change Map, part-k-of-n framing, its slice and the closing reminder
- exactly one merge call follows, over every chunk's notes in chunk order, and its output ships as the body.

The line count no longer applies to release notes. Forward releases and regenerate's fresh source take the same path, per version under `--all`. The README and the setup guide document the cap and chunking.

**Outcome**: A release of any size gets AI-written notes through the normal AI path, with no setting and no prompt, while a release that fits one call behaves as before.

**Acceptance Criteria**:
- [ ] Given no `max_prompt_bytes` set and a release whose composed single-pass prompt is at most 1,500,000 bytes, including one whose diff runs past 50,000 lines, the AI tool receives exactly one call, carrying the single-pass prompt (§4.2, §7.2)
- [ ] Given a release whose diff alone is under the cap but whose composed single-pass prompt is over it, for example through a long `[release].context`, no single-pass call is made and the notes are chunked (§7.2)
- [ ] Given a release whose single-pass prompt is over the cap, no single-pass call is made. The AI tool receives chunk calls, none bigger than the cap, each carrying (§4.1, §8.2, §8.6):
  - the release's instructions: the default prompt or the `[release].prompt` override, with `context`
  - the whole-release Change Map
  - framing that names its part (part k of n)
  - its slice of the diff
  - the closing reminder, last
- [ ] Given files whose diffs each fit a chunk's room, they are packed in path order, each chunk filled until the next file's diff would not fit. Together the chunks carry every file's diff exactly once (§8.3)
- [ ] Given the chunk calls succeed, exactly one merge call follows. It carries the release's instructions, the same Change Map, every chunk's notes in chunk order, framing that asks for one set of release notes, and the closing reminder, and none of the diff. Its output ships byte-identical as the tag message, changelog entry and provider release (§8.1, §8.6)
- [ ] Given chunked notes on an attended run, they reach the review gate as AI notes, offered the same choices as single-pass notes, `r` included (§15.1)
- [ ] Given `mint release regenerate <version>` from a fresh source on a version over the cap, or that version under `regenerate --all`, its notes are chunked and merged the same way, over that version's own range (§1.3, §7.2)
- [ ] Chunking has no setting and no consent prompt: no config key or flag turns it off, and an over-cap release asks nothing before its chunk calls (§8.8)
- [ ] The README documents chunking with `mint release`, and `mint setup`'s AI-transport paragraph covers the per-call cap and chunking (§18.1, §18.2)

**Do**:
- The size check and chunked generation sit on the release-notes generation path that the forward release and regenerate's fresh source both reach (`generateFromDiffWithContext` in `internal/notes/generate.go`) (§7.2).
- Chunk and merge calls go through the same release-notes transport as the single-pass call, with the release's `ai_command` and timeout (§2.2, §11.6).
- The README documents chunking in its `release` command section. The setup guide paragraph is the "The AI transport" text in `overview()` in `internal/setupguide/setupguide.go` (§18.1, §18.2).

**Context**:
> §7.2, the AI path in order: the exclusion check (Phase 7), cache reuse (as built), the size check, then one single-pass call under the cap, or chunked generation over it with the single-pass call skipped. Regenerate's fresh source follows the same path without the check and cache steps. The size check measures the prompt with task 2-3's condensed map in it.
>
> §4.2: the cap is a size policy, not a fit prediction. A prompt under it can still overflow the model; Phase 4's failure classifier and halving handle that.
>
> §8.1: refine is out. Sequential calls each handed the notes so far are serial by construction and biased toward early chunks. Concatenating the chunk notes is neither the plan nor a fallback.
>
> §8.3: a chunk call's room for diff is the cap minus its fixed part: the instructions, Change Map, chunk framing and closing reminder. Path order keeps each area's files together (the Change Map's area is a path's leading directory), and each folder's.
>
> §8.6: a chunk call returns notes for its part, in the release's notes format. The merge framing asks for one set of release notes, combining related changes across parts, dropping duplicates and ranking by the Change Map. mint's framing is fixed text of its own; the exact wording is the implementer's. Chunk notes are held in memory for the run (§8.7).
>
> §8.8: chunking only happens when a release cannot go in one call, so switching it off would only mean such releases get no AI notes. A key is cheap to add and costly to remove under strict config, so one can be added later if a real case appears. `max_prompt_bytes` stays the only size setting.
>
> Later tasks in this phase build the rest of the chunked path: four calls at once (2-6), a file too big for one call (2-7), progress narration (2-8), calls that cannot be made (2-9) and a failed chunk (2-10). Until Phase 6, `r` on chunked notes re-runs the whole AI path; Phase 6 makes it re-run only the merge and carries per-chunk notes in the dry-run cache. As built, the dry-run cache keys on the post-exclusion diff, the version and the resolved instructions (§15.2).
>
> The comments that state release's line-count guard go stale with this change (§18.3): `internal/notes/generate.go`, `size.go`, `assemble.go`, `prompt.go`, `degenerate.go`, and `internal/engine/regenerate_fresh.go`, `regenerate_batch.go`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §1.3 The consumers; §4.1 The key; §4.2 What the cap does; §4.3 Per consumer; §7.2 The AI path, in order; §8.1 The shape: map-reduce; §8.2 The Change Map in every call; §8.3 Packing chunks; §8.6 What each call is told; §8.8 Always on; §15.1 `r` on chunked notes; §15.2 The dry-run cache; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-2-5

### Task 2-5: Remove `max_diff_lines`

**Problem**: Once tasks 2-2 and 2-4 land, `max_diff_lines` governs nothing, so a `.mint.toml` can still set it and have it silently ignored. Its config-metadata row, its README row, and the `diff too large` cause phrase and skip reason still describe a line ceiling that no longer exists (§4.6, §12.4).

**Solution**: Drop `max_diff_lines` from the schema. A `.mint.toml` still setting it fails at load with a specific message naming its replacement, as the `[hooks]` placement error does. Its metadata row and README row go. No release-notes failure and no `regenerate --all` skip reads `diff too large` any more. CLAUDE.md and the code comments that name the removed key or guard are kept true.

**Outcome**: A repo still carrying the old key learns at load what replaced it. Nothing mint prints or documents describes a line ceiling, and no release-notes failure reads `diff too large`.

**Acceptance Criteria**:
- [ ] Given a `.mint.toml` setting `max_diff_lines` at the top level, each of `mint release`, `mint release regenerate <version>` and `mint commit` fails before any step runs with `invalid .mint.toml: max_diff_lines has been replaced by max_prompt_bytes, a per-call size cap in bytes` (§4.6)
- [ ] `mint setup`'s config reference and the README's per-key tables carry no `max_diff_lines` row (§18.1, §18.2)
- [ ] Given a forward release or `mint release regenerate <version>` whose diff runs past 50,000 lines and whose notes call fails, the ✗ line carries the AI failure's cause phrase, never `diff too large` (§12.4)
- [ ] Given `regenerate --all` where a version's notes fail, its skip warning and the end-of-run summary never read `diff too large` (§12.3)
- [ ] Given a `mint commit` whose composed prompt is over the cap, its editor note still reads `diff too large to summarise — opening editor` (§4.3)
- [ ] CLAUDE.md names no symbol this task removes: its config seam no longer names `max_diff_lines`, and its layout line for `internal/notes` names no removed export (§18.3)

**Do**:
- The key leaves the top-level decode shape, `typeErrorMessages` and `config.MetadataRows()` in `internal/config`, and the README's shared engine keys table. Its replaced-key message sits beside the `[hooks]` placement message in `unknownKeyMessage` (`internal/config/config.go`) (§4.6, §18.2).

**Context**:
> §4.6: under strict config, a `.mint.toml` still setting `max_diff_lines` fails at load. Like the `[hooks]` placement error, it gets a specific message naming its replacement.
>
> One of the user's repos still sets the key, and that line comes out before it upgrades: `/Users/leeovery/Code/agentic-workflows/.mint.toml`. That repo is outside this one, and editing it is not this task's work.
>
> §12.4: release-notes failures no longer use `diff too large`. Commit's editor note is a different string and stays (§4.3).
>
> As built, `diff too large` reaches the ✗ line through `notes.CauseText` (`internal/notes/resolve.go`), and reaches the `regenerate --all` skip through `reasonDiffTooLarge` / `classifyNotesFailure` (`internal/engine/regenerate_batch.go`). Phase 4 brings §12.4's cause phrases, and Phase 5 makes each skip name its real cause. Until then a skipped version reads the as-built generic reason.
>
> CLAUDE.md's layout line reads "`internal/notes` — … exposes the reusable `CheckDiffSize`/`ErrDiffTooLarge`", and its config seam names `max_diff_lines` beside `diff_exclude` and, since task 2-1, `max_prompt_bytes`. §18.3 keeps any line naming a renamed or removed symbol true, and the code comments that state the line-count guard are rewritten in the same change.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §4.6 Removing `max_diff_lines`; §4.3 Per consumer; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves; §18.1 The setup guide; §18.2 The README; §18.3 CLAUDE.md and code comments

## token-aware-diff-sizing-2-6

### Task 2-6: At most four chunk calls in parallel

**Problem**: A chunked release can need many chunk calls. Run one after another, they take as long as all of them together, and the steer is parallel first, never a chain of serial AI calls (§8.1). Run all at once, they put the whole wave on the machine, and on the provider's rate and usage limits, at the same moment (§8.5).

**Solution**: Run chunk calls concurrently, never more than four at once. The bound is fixed, not a setting. The merge starts only once every chunk call has finished, and receives their notes in chunk order whatever order they finished in. Each call keeps its own per-attempt timeout, and the wave has no overall deadline.

**Outcome**: A chunked release takes roughly as long as its rounds of up to four chunk calls plus the merge. Its notes are the same whichever call finishes first, and no wave ever has more than four chunk calls in flight.

**Acceptance Criteria**:
- [ ] Given an over-cap release that packs into six chunk calls, four are in flight together, and at no point are more than four in flight (§8.5)
- [ ] Given an over-cap release that packs into three chunk calls, all three are in flight together (§8.1, §8.5)
- [ ] No config key or flag changes the bound of four (§8.5)
- [ ] Given chunk calls that finish in a different order from chunk order, the merge call carries their notes in chunk order (§8.6)
- [ ] Given a wave, the merge call starts only after every chunk call has finished (§8.1)
- [ ] Given a per-attempt timeout, and a wave whose every call finishes inside it but whose rounds together run longer than it, the notes are written. Each chunk call and the merge call has its own per-attempt deadline, and no deadline spans the wave (§11.6)

**Context**:
> §8.1: chunk calls run in parallel, then exactly one merge call produces the final notes. The merge is the one serial step.
>
> §8.5: the bound is fixed, not a setting. Unbounded fan-out would put the whole wave on the machine, and on the provider's rate and usage limits, at the same moment.
>
> §11.6: the per-attempt timeout applies to each call on its own, so a wave takes roughly as long as its rounds of chunk calls plus the merge. There is no overall deadline across the calls: one would be a second timing rule whose right value grows with the release, which is exactly what the per-call cap exists to avoid predicting. A chunk call's timeout ends the wave (task 2-10).
>
> Task 2-4 built the chunk calls and the merge this task makes concurrent, and task 2-7's pieces run under the same bound. The transport applies its per-attempt deadline and single retry inside each call, as built. As built, the notes tests inject a recording fake `Transport` rather than scripting the AI command through the runner, and `runner.FakeRunner` holds no lock. The gates run `go test -race ./...`.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.1 The shape: map-reduce; §8.5 Concurrency; §8.6 What each call is told; §11.6 Timeouts across a wave

## token-aware-diff-sizing-2-7

### Task 2-7: Pieces for a file too big for one call

**Problem**: Packing whole files into chunks cannot place a file whose diff alone exceeds a chunk call's room, such as a large generated file or a minified bundle on one line (§8.4). No call under the cap could carry it whole.

**Solution**: Split such a file's diff into pieces. Each piece is a chunk call of its own, run in parallel with the others and stitched by the merge like any other chunk's notes. Cuts fall at hunk boundaries, else at line boundaries, and a single line longer than the room is cut at a character boundary. Each piece carries the file's diff header and says which piece of how many it is. Nothing in the diff is dropped or stood in for.

**Outcome**: Every file in an over-cap release is read by the AI, whatever its size, in calls that each fit the cap.

**Acceptance Criteria**:
- [ ] Given an over-cap release with a file whose diff alone exceeds a chunk call's room, that file is sent as piece calls. None is bigger than the cap, and each carries only that file's diff, never another file's (§8.4)
- [ ] Given that file's hunks each fit a piece, every cut falls between hunks (§8.4)
- [ ] Given a hunk bigger than a piece's room, it is cut between lines (§8.4)
- [ ] Given a single line longer than the room, such as a minified bundle, it is cut at a character boundary, never inside a multi-byte character (§8.4)
- [ ] Each piece opens with the file's diff header, and its framing names the file and which piece of how many it is (§8.4, §8.6)
- [ ] Given a file sent as pieces, the pieces' diff content joined in order, with the file's header kept once, reproduces the file's diff byte for byte (§8.4)
- [ ] Given a release whose other files are packed into chunks, the pieces run alongside those chunk calls under the four-at-once bound, and their notes reach the merge in chunk order like any other chunk's (§8.4, §8.5, §8.6)

**Context**:
> §8.4: the exclusion check (Phase 7) asks beforehand whether each file belongs, so a file still in the diff is wanted, and it is read. Nothing is dropped or stood in for.
>
> §8.3: a chunk call's room for diff is the cap minus its fixed part: the instructions, Change Map, chunk framing and closing reminder. Files are packed in path order.
>
> §8.6: a chunk call's framing names its part of the release: part k of n, and for a piece, the file and which piece of how many. mint's framing is fixed text of its own; the exact wording is the implementer's.
>
> §10.1: Phase 4's halving re-cuts a failed slice by these same packing and piece rules.
>
> A character boundary is a boundary between characters of the diff text, so a cut never splits a multi-byte UTF-8 character.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §8.3 Packing chunks; §8.4 A file too big for one call: pieces; §8.5 Concurrency; §8.6 What each call is told; §10.1 When it applies

## token-aware-diff-sizing-2-8

### Task 2-8: Chunked-run progress narration

**Problem**: A chunked run can take minutes: rounds of chunk calls a few at a time, then the merge. A spinner that sits that long looks the same whether mint is working or hung (§16.1). CI and agents reading plain output get no live line at all, so a long run goes quiet until it ends (§16.2).

**Solution**: Keep the single notes line and update it as a chunked run moves: the release's size and part count with parts done, then the merge, then a completion line naming the part count. Sizes are written in decimal units. Plain mode prints each update as its own line as it happens. The updates are new presenter events, and a single-pass run narrates as today.

**Outcome**: Whoever watches a chunked run, at a terminal or in a log, sees it advance part by part and knows when it is merging.

**Acceptance Criteria**:
- [ ] Given a chunked run in pretty mode, the one notes line reads `Writing notes: {size} release, in {n} parts · {k} of {n} done`, with k rising as each part finishes. It then reads `Writing notes: merging {n} parts` while the merge runs, and the run ends `✓ Notes written ({n} parts, merged) ({elapsed})`, keeping the elapsed time every blocking stage's success line carries. No other line appears per part (§16.1)
- [ ] Given that run, `{size}` is the composed single-pass prompt the size check measured against the cap: a 3,100,000-byte prompt reads `3.1 MB` (§16.1, §7.2)
- [ ] Sizes are written in decimal units, 1 MB being 1,000,000 bytes: one decimal place below 10 and a whole number from 10 up, in KB below 1 MB and in plain bytes below 1 KB. 1,500,000 bytes reads `1.5 MB`, 38,000,000 `38 MB`, 850,000 `850 KB` and 640 `640 B` (§16.1)
- [ ] Given a release whose parts include a file's pieces, `{n}` counts each piece as one part (§8.4, §16.1)
- [ ] Given the same run in plain mode, each update prints as its own line as it happens, one per finished part included, then the merge and the completion (§16.2)
- [ ] Given a run whose notes go in one call, the narration is as built, with no `Writing notes:` line (§16.1)
- [ ] Given `mint release regenerate <version>` from a fresh source, or a version under `regenerate --all`, whose notes are chunked, that version's notes line narrates the same way (§1.3, §16.1)

**Do**:
- The updates are new presenter events in the existing model (`internal/presenter`): the pretty presenter renders them on the running notes line, the plain presenter as lines of its own, and `presentertest.RecordingPresenter` records them. The engine never touches spinners or TTY state (§16.4).

**Context**:
> §16.1: there is no per-part detail beyond the count. Which folders a part covers, or each part's own status, would be noise unless something fails, and on a failure the ✗ line names the part (Phase 4).
>
> §16.4: spinners are a pretty-only concern spanning a stage's start and its success or failure, and plain mode renders the same events as terse lines. Each presenter renders the new events in its own way.
>
> As built, the engine narrates notes production as a blocking `StageStarted` with the text `generating release notes…` and a `StageSucceeded` with the sentence `Generated release notes`, at three sites: `internal/engine/release.go`, `regenerate_interactive.go` and `regenerate_batch.go`. The `Presenter.StageStarted` doc's stream-discipline note forbids other narration between a blocking start and its completion, so the new events have to render without breaking the running line. Plain's synthesised narration is byte-pure printable ASCII (`internal/presenter/bytepurity_test.go`), so its lines cannot carry `·`, `…` or `✓`; their exact wording is the implementer's. `internal/notes` does not import the presenter today. Chunk calls finish concurrently (task 2-6), and the gates run `go test -race ./...`.
>
> Later phases add to the same line: Phase 4's halving lines, which grow the part total by the parts a re-cut adds, and Phase 7's `Checking for files that don't belong in the notes…`. Phase 7's exclusion-check list and pauses write sizes in this same format.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §16.1 Pretty mode: one notes line, updated; §16.2 Plain mode: a line per event; §16.4 How it is built; §7.2 The AI path, in order; §8.4 A file too big for one call: pieces; §1.3 The consumers; Corrigenda (2026-10-19, §16.1)

## token-aware-diff-sizing-2-9

### Task 2-9: Calls that cannot be made

**Problem**: Some chunked runs need a call that no prompt under the cap can carry. An outsized `[release].prompt` or `context` can leave a chunk call no room for diff, even with the Change Map condensed. And a release's chunk notes together can make a merge prompt bigger than the cap (§8.7). Sending either call would break the rule that no call is bigger than the cap (§4.1), and mint never trims operator text to make room.

**Solution**: Send no call in either case. When a chunk call's fixed part leaves no room for diff because of operator text, make no AI call at all and fail the notes as `no room for the diff under max_prompt_bytes`. When the merge prompt would exceed the cap, do not send the merge and fail the notes as `part notes too large to merge`. Both failures route like any other notes failure.

**Outcome**: No over-cap prompt ever leaves mint, operator text is never cut, and the ✗ line names which of the two stopped the notes.

**Acceptance Criteria**:
- [ ] Given a release whose single-pass prompt is over the cap, with a `[release].context` or `[release].prompt` override file large enough that a chunk call's fixed part leaves no room for diff, the AI tool receives no call at all. The fixed part is the instructions, the condensed Change Map, the framing and the closing reminder (§8.3, §8.7)
- [ ] Given that release under `on_notes_failure = abort`, the run stops before the tag with the ✗ line `no room for the diff under max_prompt_bytes` and no AI output beneath it (§8.7, §12.4)
- [ ] Given chunk calls that succeed with notes whose merge prompt would exceed the cap, no merge call is sent. The merge prompt is the instructions, the Change Map, every chunk's notes, the framing and the closing reminder. Under `abort` the run stops before the tag with the ✗ line `part notes too large to merge` and no AI output beneath it (§8.7, §11.3, §12.4)
- [ ] Given either failure under `on_notes_failure = fallback`, the fallback body ships, as on a failed single-pass call (§12.1)
- [ ] Given either failure on `mint release regenerate <version>` from a fresh source, the regenerate aborts with the same ✗ line. Under `regenerate --all`, that version is skipped and the batch carries on (§12.3)

**Do**:
- The two phrases join the cause phrases `notes.CauseText` maps (`internal/notes/resolve.go`), which the ✗ line renders as built (§12.4).

**Context**:
> §8.7: no room for diff happens with an outsized `[release].prompt` or `context` (operator text mint never trims), or with a halving round aimed below the fixed part. Halving is Phase 4, which renders that second case as `too large for the model, even in smaller parts`. Phase 5's floor guidance names which of the two it was, and for operator text says to shorten whichever of `[release].prompt` or `context` is outsized.
>
> A single-pass prompt at or under the cap is sent as one call whatever the size of its instructions (§7.2), so the no-room case arises only once a release is chunked.
>
> §12.4: these phrases replace `AI returned empty/invalid notes after retry` for these causes. Phase 4 ends a chunked run's phrase with where the failure happened, `(merging)` for the merge, and saves a failed merge's chunk notes to a file the failure names (§11.4).
>
> §12.3: regenerate does not read `on_notes_failure`. As built, a failed single-version regenerate aborts and `--all` skips the version with a generic reason; Phase 5 has the skip name its cause phrase. Phase 3 makes the commit-subject list the fallback body everywhere.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §4.1 The key; §7.2 The AI path, in order; §8.3 Packing chunks; §8.7 Calls that cannot be made; §11.3 A failed merge; §12.1 What applies, by mode; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves

## token-aware-diff-sizing-2-10

### Task 2-10: A failed chunk ends the wave

**Problem**: A chunked run can fail partway. A chunk call can meet an expired login, a usage limit, a timeout or a missing binary, or the merge can fail after the wave (§11.1, §11.3). Merging what came back would ship notes that read as complete while leaving out a slice that may hold the release's headline change, and under `-y` they would ship unreviewed (§11.1). Carrying on with the wave would send more calls at a service already refusing them (§11.2).

**Solution**: When any chunk call fails, run no merge and start no further chunk calls; calls already running finish. Surface the first failure with its captured output verbatim, and route it as a single-pass failure is routed: through `on_notes_failure` on release, to an abort on a single-version regenerate, and to a skip under `regenerate --all`. A failed merge fails the notes the same way. Chunk notes are never concatenated into a body. A Ctrl-C mid-wave cancels every call in flight and never reaches `on_notes_failure`.

**Outcome**: A chunked run either produces merged notes from every slice or fails loud with the AI's own words. It never ships a partial or glued-together body.

**Acceptance Criteria**:
- [ ] Given a wave in which one chunk call fails, whether after the transport's retry, by timeout or by a missing binary, no merge call is made and no chunk call that had not started is started. Calls already running finish (§11.1, §11.2, §11.6)
- [ ] Given that failure under `on_notes_failure = abort`, the run stops before the tag. Its ✗ line carries the failure's cause phrase, with the failed call's captured output verbatim beneath it (§11.2, §12.1)
- [ ] Given two chunk calls in flight together that both fail, the ✗ line carries the output of the one that failed first (§11.2)
- [ ] Given chunk calls that succeed and a merge call that fails, the notes fail the same way, with the merge call's captured output verbatim beneath the ✗ line (§11.3)
- [ ] Given a chunk or merge failure under `on_notes_failure = fallback`, the fallback body ships. Chunk notes never become the body, alone or joined (§11.1, §11.3, §12.1)
- [ ] Given a chunk or merge failure on `mint release regenerate <version>` from a fresh source, the regenerate aborts. Under `regenerate --all`, that version is skipped and the batch carries on with the next (§12.3)
- [ ] Given a Ctrl-C while chunk calls are in flight, every call in flight is cancelled, no further call starts, and the run aborts cleanly. `on_notes_failure` does not apply, so under `fallback` no fallback body ships (§11.5)

**Context**:
> §11.1: the missing slice can be the release's headline feature. Notes that read as complete while quietly leaving out the biggest change are the very failure this feature exists to end.
>
> §11.2: once one chunk has failed for good, the generation has already failed, and letting in-flight calls finish costs no extra requests. In this phase every chunk failure is final. Phase 4 adds the classifier, after which a SIZE answer with halving left does not end the wave, and saves the finished chunks' notes, in-flight ones included, to a file the failure names (§11.4).
>
> §11.3: the merge is the last call after a parallel wave, so it is the one most likely to meet a usage limit the wave itself drew down, an expired login, a timeout, or an error that survives the transport's retry. There is no concatenation rung: gluing the chunk notes together would ship a visibly broken body, worst exactly when nobody is reviewing.
>
> §11.5: an interrupted chunked run behaves like an interrupted single-pass one. It stops, and a re-run starts over; mint keeps no resume state. CLAUDE.md's AI seam: `context.Canceled` propagates unchanged and is never routed to a fallback. As built, the forward selector hands any generator error to `ResolveFailure` (`resolveNormalPathFailure` in `internal/notes/select.go`).
>
> As built, the ✗ line's cause phrase comes from `notes.CauseText`, and the captured output from the `ai.GenerationError` the transport returns, composed by `notesFailureOutput` (`internal/engine/release.go`). A timeout or a missing binary carries no captured output. Phase 4 replaces the phrases with §12.4's and ends a chunked run's phrase with where it failed, such as `(part 3 of 6)`. Phase 5 builds the floor's choices and guidance on top, and Phase 3 makes the commit-subject list the fallback body.

**Spec Reference**: `.workflows/token-aware-diff-sizing/specification/token-aware-diff-sizing/specification.md` — §11.1 One failed chunk fails the generation; §11.2 The first final failure ends the wave; §11.3 A failed merge; §11.5 Ctrl-C mid-wave; §11.6 Timeouts across a wave; §12.1 What applies, by mode; §12.3 Regenerate at the floor
