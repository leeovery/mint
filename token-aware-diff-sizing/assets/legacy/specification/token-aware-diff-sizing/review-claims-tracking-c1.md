# Review Tracking: token-aware-diff-sizing - Claims Verification

## Findings

### 1. Regenerate does not read `on_notes_failure` today

**Source**: Tree measurement — `grep -rn "ResolveFailure(\|\.OnNotesFailure" internal cmd`; `internal/engine/regenerate_interactive.go:205-208`; `internal/engine/regenerate_batch.go:286-289`
**Category**: Source defect
**Move**: route
**Affects**: §1.3 The consumers; §4.4 The release timeout; §6.6 Recording the answers (commit-subject bodies bullet); §12.1 What applies, by mode; §12.3 Regenerate at the floor; §12.4 The ✗ line and the next moves (the unattended `fallback` move)

**Problem**:
The specification is written as if `mint release regenerate` already reads `[release].on_notes_failure`. It says regenerate rides on all `[release]` config as built. It offers the single-version regenerate's floor choice "under `abort`". It says the commit-subject body shipped under `fallback` leaves out mint's answers commit "on a later regenerate". It also lists `on_notes_failure = "fallback"` as an unattended next move with no verb scoping.

As built, regenerate never reads the key. A single-version fresh regenerate aborts on any AI failure, whatever the key is set to. `--all` skips the version. Only the forward release's notes path reads `on_notes_failure`.

That leaves a behaviour change nobody has decided: under `fallback`, should a failed regenerate now ship a commit-subject list onto a version that is already out, replacing its published notes and changelog entry, or keep today's abort, which leaves them untouched? Take an operator with `fallback` set who regenerates a published version and hits an expired login. Depending on the implementer's guess, they either keep their published notes or find them overwritten with a list of commit subjects. An unattended regenerate would also be told to set a key that does nothing there today.

**Evidence**:
Claims (specification):
- §1.3: "Regenerate rides on `[release]` config throughout, as built."
- §4.4: "Regenerate rides on it, as on all `[release]` config."
- §12.3: "**`regenerate <version>`, attended, `abort`:** the same choice, finishing in the same run."
- §6.6: "So the floor's `c` and `fallback` never list mint's own answers commit, on the forward run or on a later regenerate."

Measurements:
- `grep -rn "ResolveFailure(\|\.OnNotesFailure" internal cmd | grep -v _test.go | grep -v -E "^[^:]+:[0-9]+:\s*//"` →
  ```
  internal/config/config.go:432:	if err := validateOnNotesFailure(shape.Release.OnNotesFailure); err != nil {
  internal/config/config.go:681:		OnNotesFailure: shape.OnNotesFailure,
  internal/notes/resolve.go:65:func ResolveFailure(ctx context.Context, r runner.CommandRunner, failure error, lastTag string, rel config.Release) (string, error) {
  internal/notes/resolve.go:66:	if rel.OnNotesFailure == "fallback" {
  internal/notes/select.go:205:	body, err := ResolveFailure(ctx, s.runner, failure, lastTag, rel)
  ```
- `grep -rn "NewSelector\|SelectBody" internal cmd | grep -v _test.go | grep -v -E "^[^:]+:[0-9]+:\s*//"` → callers only at `internal/engine/release.go:846` and `:861`, which is the forward release.
- `internal/engine/regenerate_interactive.go:205-208` (single version):
  ```
  	body, err := req.ProduceBody(ctx, source)
  	if err != nil {
  		return surface(p, "notes", err)
  	}
  ```
- `internal/engine/regenerate_batch.go:286-289` (`--all`):
  ```
  	body, err := req.ProduceBody(ctx, req.Source, res, preReadBody)
  	if err != nil {
  		return reportSkip(p, res.Tag, classifyNotesFailure(err))
  	}
  ```
- Two code comments describe single-version regenerate as following "the on_notes_failure default abort": `internal/engine/regenerate_fresh.go:25-27` and `internal/engine/regenerate_batch.go:183`. No regenerate path reads the key.

Source carrying the claim: discussion, Remediation at the Floor → Decision ("Single-version regenerate asks too"): "An attended single-version `regenerate` that reaches the floor under `abort` offers the same `c` / `e` / `a` choice". Also Oversize Knob → 2026-10-17: "regenerate rides `[release]`".

**Current**:

**Proposed Text**:

**Resolution**: Routed
**Notes**: Measurement re-run and landed in the discussion as a dated revision of Remediation at the Floor's Decision (regenerate does not read `on_notes_failure`; an attended single-version regenerate offers `c` / `e` / `a` whatever it holds; `fallback` stays forward release's), with Floor Wording and Guidance's unattended `fallback` move amended to forward release. Specification re-aligned in §1.3, §4.4, §6.6, §12.1, §12.3 and §12.4.

---

### 2. `regenerate --all` drops the AI's output too, not only `mint commit`

**Source**: Tree measurement — `internal/engine/regenerate_batch.go:286-289, 381-383`
**Category**: Source defect
**Move**: route
**Affects**: §17.1 The AI's output, verbatim; §12.3 Regenerate at the floor (`regenerate --all` bullet)

**Problem**:
The specification says `mint commit` is the only place an AI failure's own output is dropped, and restores it only there.

`mint release regenerate --all` drops it too. When a version's notes fail, the skip warning carries only a reason phrase. No captured output appears beneath it, and no ✗ line is drawn. The specification's `--all` floor upgrades that phrase to the real cause, but still shows nothing of what the AI said.

So an operator whose batch hits an expired login or a usage limit sees `skipped v1.2.0` with `AI call failed`, never claude's own `Not logged in · Please run /login`. To learn why, they have to re-run that version on its own.

**Evidence**:
Claim (§17.1): "Commit is the one site that breaks the rule that subprocess failure diagnostics travel verbatim, and it matters for any commit failure".

Measurement, `internal/engine/regenerate_batch.go:286-289`: a failed notes call goes to `reportSkip(p, res.Tag, classifyNotesFailure(err))`. Then `:381-383`:
```
func reportSkip(p presenter.Presenter, tag, reason string) (RegeneratedVersion, *skippedVersion, error) {
	p.Warn(presenter.Warning{Label: "skipped " + tag, Message: reason})
	return RegeneratedVersion{}, &skippedVersion{Tag: tag, Reason: reason}, nil
```
The `Warning` has no `Output`. The stage comment above `:286` says a skip "closes the stage with no StageSucceeded — the skip Warn narrates it instead", so no `StageFailed` carries `notesFailureOutput` either. By contrast, single-version regenerate renders the output through `surface` (`internal/engine/release.go:1682-1690`).

Source carrying the claim: discussion, Commit AI-Failure Output → Journey: "commit is the one site that drops the output."

**Current**:

**Proposed Text**:

**Resolution**: Routed
**Notes**: Measurement re-run. The discussion's Commit AI-Failure Output Journey was corrected in place, and a new subtopic, `regenerate --all`'s Skip Output, landed the rule derived from CLAUDE.md's verbatim-diagnostics rule: each skip warning carries the failed call's captured output, the first final failure's in a chunked run. The specification was re-aligned in §1.3, §12.3 and §17.1.

---

### 3. The README tripwire cannot see the new `diff_exclude` levels

**Source**: Tree measurement — `internal/config/readme_tripwire_test.go:46-72`
**Category**: Enhancement to existing topic
**Move**: settled
**Affects**: §18.2 The README (per-key tables bullet)

**Problem**:
The specification says the README tripwire test covers the four per-key table additions. It does not. The test checks only that each distinct schema key name appears somewhere in the README text.

That catches a missing `max_prompt_bytes` or `diff_keep`. But `diff_exclude` is already in the README, so the new `[release]` and `[commit]` rows can be left out with every gate green. Operators would then find nothing in the README on keeping test or spec folders out of release notes while commit messages still read them, which is the reason the split exists.

**Proposal**:
Correct the sentence to say what the tripwire actually checks, and which additions it does and does not cover, with the measurement. The test's own code settles it: distinct key names, matched by substring.

**Evidence**:
Claim (§18.2): "The tripwire test that asserts every schema key appears in the README covers the additions."

Measurements:
- `grep -n 'strings.Contains(readme, key)' internal/config/readme_tripwire_test.go` → `68:		if !strings.Contains(readme, key) {`
- `grep -n "seen\[row.Key\]" internal/config/readme_tripwire_test.go` → `49:		if seen[row.Key] {` and `52:		seen[row.Key] = true`. Key names are deduped across levels.
- `grep -c 'diff_exclude' README.md` → `1`. The name is already present, so a `[release].diff_exclude` or `[commit].diff_exclude` row adds no new name for the test to require.
- The file header states the design: "this tripwire is deliberately COARSER: it asserts every distinct schema key NAME appears somewhere in the README text".

**Current**:
The tripwire test that asserts every schema key appears in the README covers the additions.

**Proposed Text**:
The tripwire test checks only that each distinct schema key name appears somewhere in the README (`grep -n 'strings.Contains(readme, key)' internal/config/readme_tripwire_test.go` → `68:		if !strings.Contains(readme, key) {`). It catches a missing `max_prompt_bytes` or `diff_keep`. It does not catch a missing `[release].diff_exclude` or `[commit].diff_exclude` row, because the README already names `diff_exclude` (`grep -c 'diff_exclude' README.md` → `1`).

**Resolution**: Approved
**Notes**:

---

## Observations

- §12.2's "accepting it ships an empty tag message": mint's tag annotation is always `{commit_prefix} Release {tag}`, a blank line, then the body (`internal/release/release.go:124-127`). An empty body gives a subject-only annotation, never an empty one. The refusal still stands on the empty changelog entry and provider release.
- §14.1 calls the regenerate range "as built". `FallbackBody` only runs `git log --format=%s {lastTag}..HEAD` (`internal/notes/resolve.go:27`), and no regenerate path builds a commit-subject list. So the version's own range is new code. Reusing the helper with the previous tag would list every later version's commits too.
- §16.4's "the engine never touches spinners": the engine calls the presenter's `SuspendSpinner`/`ResumeSpinner` hooks around a gate shown mid-stage (`cacheReuse`, `internal/engine/release.go:923-925`). `PrettyPresenter.Prompt` does not suspend a spinner itself (`internal/presenter/pretty.go:788-803`).
- §9.1's cancel rule is new code. As built, the notes selector routes any generator error, a cancel included, into `ResolveFailure` (`internal/notes/select.go:205`; there is no `context.Canceled` check in `internal/notes`). Under `fallback`, that goes on to build a body.
- §3.3's list of whitespace-split comments is incomplete. The split is also described at `internal/config/config.go:535` (`AICommandFor`), `internal/ai/transport.go:92` (`Config`) and `internal/aitransport/aitransport.go:30`.
- §3.3's "Only a backslash in an existing command could change meaning": a literal quote character in a working command (an apostrophe, say) would also change meaning or fail at load. This is moot: `grep -HE '^[[:space:]]*ai_command[[:space:]]*=' ~/Code/*/.mint.toml` → no output.
- `on_notes_failure`'s config-metadata row (`internal/config/metadata.go:146`) and README entry (`README.md:222`) still name the fixed-string body that dropping `[release].fallback` removes.
- Not re-run: the token counts in §2.1 (105,994 and 628) and §4.5 (54,654) each need a live claude call against the subscription. §2.5's unknown-option rejection would need an older claude. The 10 MB stdin claim matches a separate `Error: piped stdin input exceeds …MB. Pass large content as a file path in your prompt instead.` string in the claude 2.1.302 binary. The threshold there is interpolated and can't be read.
