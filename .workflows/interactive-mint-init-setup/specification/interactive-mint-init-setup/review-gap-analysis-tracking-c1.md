---
status: in-progress
created: 2026-06-16
cycle: 1
phase: Gap Analysis
topic: interactive-mint-init-setup
---

# Review Tracking: interactive-mint-init-setup - Gap Analysis

## Findings

### 1. README "Configuration" section still describes/embeds the commented template — strip-to-minimal makes it wrong, and no README section reconciles it

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Generated config: strip to minimal", "README — entry point", "README — config reference verification"

**Details**:
The strip-to-minimal decision changes `mint init` to emit an empty body + header `.mint.toml`. The current README "Configuration" section opens with *"`mint init` writes a **commented** `.mint.toml` at the repo root"* and then embeds the **entire pre-strip commented template** as a fenced TOML block (the full `ai_command`/`timeout`/`[release]`/`[release.hooks]`/`[commit]` sample, matching today's `initgen` output verbatim). After strip-to-minimal both the sentence and the embedded sample become false — the README would document a file `mint init` no longer produces.

The spec has three README sub-sections (entry-point prompt, "any AI" framing, config-reference verification) and they are detailed about what to ADD (the entry prompt, the per-key tripwire), but none of them addresses this **existing stale content that the strip directly invalidates**. The "config reference verification" section says the README is "verified to declare every config key + its default" and "stays manual narrative" — but it does not say whether the embedded commented-template sample is kept, replaced with the new minimal (empty body + header) sample, or removed. An implementer would have to guess whether to (a) leave the now-wrong sample, (b) swap it for the minimal template, or (c) delete it and rely on the per-key tables. This is in-scope because README updates are an explicit deliverable and strip-to-minimal is the change that breaks it.

**Proposed Addition**:
Added a bullet to "README — config reference verification": reconcile the existing `## Configuration` section with strip-to-minimal — (a) correct the "writes a commented `.mint.toml`" framing to minimal (empty body + header), (b) replace/drop the embedded full-template TOML block (the per-key tables are the authoritative human reference + tripwire surface), (c) correct the Commands-section line the same way.

**Resolution**: Approved
**Notes**: Auto-approved. Grounded against README:62, :176, :178-213. The per-key tables (README:215-262) are the existing human config reference.

---

### 2. "Updated `initgen` tests" is underspecified — the existing suite has ~12 tests that assert the commented-template shape and will fail under strip-to-minimal

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "`initgen` scope of change", "Definition of done" (Updated `initgen` tests)

**Details**:
The spec says only `MintTOML()` changes (to "empty body + header") and lists "Updated `initgen` tests — for the minimal (empty body + header) `.mint.toml` template; `ReleaseShim()` tests untouched." But the current `internal/initgen/initgen_test.go` is built almost entirely around the commented-template contract and would FAIL wholesale after the strip: `TestMintTOML_IncludesCommonKeysAtDefaults`, `TestMintTOML_AICommandIsPinnedSonnetDefault`, `TestMintTOML_AICommandValueEqualsConfigConstant`, `TestMintTOML_ScaffoldsActiveSharedTimeout`, `TestMintTOML_TimeoutValueEqualsConfigConstant`, `TestMintTOML_PerVerbAICommandAndTimeoutOverridesShownCommented`, `TestMintTOML_CommentsNameNoModelOrStrongerModelSteer`, `TestMintTOML_TimeoutHintFramedAroundLatency`, `TestMintTOML_OptionalKeysPresentButCommentedWithExplanation`, `TestMintTOML_OptionalKeysEachHaveAComment`, `TestMintTOML_PreTagShowsBothStringAndArrayForms`, `TestMintTOML_MentionsPromptOverrideInCommentOnly`, `TestMintTOML_HooksOnlyUnderReleaseHooks`, and `TestMintTOML_UncommentedLoadsCleanly`.

"Updated" reads as a light touch, but the reality is that the bulk of these tests must be **deleted** (they assert content the minimal template no longer has), not edited. Two specific consequences the spec leaves unstated and an implementer must guess at:
- **The two drift-pin tests** (`AICommandValueEqualsConfigConstant`, `TimeoutValueEqualsConfigConstant`) pin the scaffold's literal default values to `config.DefaultAICommand` / `config.DefaultTimeout`. With an empty body there is no scaffolded value to pin — does this drift discipline move to the new SoT drift test, or is it dropped from `initgen` entirely? (The spec's CLAUDE.md-level invariant is that scaffolded defaults are drift-pinned; the new SoT table now carries the `default` column, so the natural answer is "the SoT drift test subsumes it" — but the spec never says so.)
- **What the new tests assert about the header** — the spec decides "empty body + a short header comment" pointing to GitHub docs and `mint setup`, but doesn't state whether a test pins those two pointers' presence. The "recovery net" rationale strongly implies the header pointers are load-bearing (the cold-arrival safety net), which argues for a test, but the DoD list doesn't name one.

**Proposed Addition**:
Expanded "`initgen` scope of change" with two bullets: (1) the commented-template-shape tests are removed (not edited), new tests assert the minimal shape (empty body + header carrying both pointers, pinned); (2) the scaffold-value drift-pin (`ai_command`/`timeout` == config constants) is subsumed by the new SoT drift test, since the minimal template carries no values to pin. Also tightened the DoD "Updated `initgen` tests" bullet to match.

**Resolution**: Approved
**Notes**: Auto-approved.

---

### 3. SoT `default` column has no defined rendering for empty/sentinel/non-scalar defaults — the "real compiled default" rule only covers the scalar keys

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Config-metadata source of truth (SoT)", "Emitted guide — minimalism (only set what varies)"

**Details**:
The SoT `default` column is specified as "the **real** compiled default (e.g. `claude -p --model sonnet`, `60`, `v`, `true`), not an illustrative example." Every example given is a scalar with a concrete non-empty default. But a large share of the canonical schema's keys do NOT have a concrete scalar default:
- **Empty-string defaults**: `context`, `prompt`, `fallback`, `version_file`, `version_pattern` (default `""`).
- **Sentinel-empty "auto" defaults**: `release_branch` (`""` = auto-derive from `origin/HEAD`), `provider` (`""` = auto-detect from remote host). The empty string here MEANS "auto," which is semantically different from "unset/no value."
- **Empty collection**: `diff_exclude` (`[]`).
- **No scalar default at all**: the three `[release.hooks]` keys (`preflight`, `pre_tag`, `post_release`) are absent-or-a-command; there is no compiled default value to print.
- **"Inherit" defaults**: the per-verb `ai_command`/`timeout` overrides default to "fall through to the shared value" — the README renders these as the literal word `shared`, not a value.

The minimalism section leans hard on this column ("The agent judges 'is the default fine?' against that real default"), so an empty or ambiguous `default` cell directly weakens the feature it exists for: for `release_branch`/`provider` an agent that reads a blank cell can't tell "no default" from "default = auto," and for hooks there is nothing to compare against. The spec needs to say how the `default` column represents empty (`""` vs blank vs the word `auto`/`none`/`inherit`), how it represents the hooks keys (which arguably are "activate-only" with no default), and how it represents the per-verb override "inherit-the-shared" case. This is decided behavior the agent depends on, not a planning-only rendering detail.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 4. Drift-test "every key" set boundary is ambiguous for the two-level keys, the hooks sub-table, and the sentinel keys

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Drift test (the anti-drift enforcement)", "Config-metadata source of truth (SoT)"

**Details**:
The drift test is specified as "a key present in the schema but missing from the SoT, or vice versa" fails the build. But the canonical schema's key set is not a flat list of distinct names, and the spec doesn't define what counts as one "key" for the bijection:
- **`ai_command` and `timeout` exist at BOTH levels** — top-level shared AND under `[release]`/`[commit]`. The README models these as four distinct rows (one shared, one per verb table). Does the SoT carry one row or one-per-level for these? The drift test's "present in the schema" check has to know whether `[release].ai_command` is a separate key from the top-level `ai_command` or the same key surfaced twice. The `level` column suggests per-level rows, but the test's matching rule against `config`'s Go struct fields (where the override is `Release.AICommand`, a distinct field from `Config.AICommand`) is left to the implementer to define.
- **The `[release.hooks]` sub-table keys** (`preflight`, `pre_tag`, `post_release`) live on a nested struct (`Hooks`), not the top-level/verb tables. Are they in the drift-test's "schema" set? The `level` column explicitly lists `[release.hooks]` as a value, so they presumably are — but the test needs a defined traversal of the nested struct, which the spec doesn't mention.
- **What "the schema" is, mechanically.** The existing `initgen↔config` drift discipline pins literal *values* against exported constants. This new drift test asserts *key-set membership*. There is no machine-readable enumeration of "all config keys" in `internal/config` today (keys live as struct fields with `toml:"..."` tags across `fileShape`, `releaseShape`, `commitShape`, `hooksShape`). The spec says "drift-tested against the real `config` schema" but the *mechanism* of deriving the authoritative key set (reflect over the shape structs' tags? a hand-maintained list the test also pins?) is undefined, and that mechanism is what determines whether the test can actually catch a drift rather than comparing two copies of the same hand-list. (Package layout is deferred, granted — but "what the test compares against" is a behavioral contract, not layout.)

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 5. Structural test "asserts it emits the required sections" — the detection contract (how a section's presence is proven) is undefined

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Definition of done" (Structural test on `mint setup` output)

**Details**:
The DoD lists a "Structural test on `mint setup` output — asserts it emits the required sections: pipeline/hook model, etiquette, minimalism, the if-exists/upgrade branch, and the config table." The guide *prose itself* is explicitly deferred to planning. That creates a tension the spec doesn't resolve: a test that "asserts the required sections are present" needs SOME stable anchor to detect each section, but the prose those anchors live in isn't decided. Without a defined contract (e.g. each section emits a stable marker/header string the test greps for, vs. the test asserts representative substrings of free prose), the implementer must invent the anchoring scheme — and an anchoring scheme chosen in the test couples the (deferred) prose to fixed markers, a design decision the spec should make rather than leave to the test author. The config-table sub-assertion is concrete (the table renders from the SoT), but the other four sections (pipeline/hook model, etiquette, minimalism, if-exists/upgrade) are pure prose with no decided anchor. State whether the emitted guide carries stable section markers/headers the structural test keys on.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 6. The `setup` rootUsage command-list line — its one-line description is required output but never decided (and the coverage test pins it)

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "The `mint setup` subcommand" (Help-surface wiring), "Definition of done" (Help-contract coverage test)

**Details**:
The spec mandates "A `rootUsage` command-list line for `setup`" and a "Help-contract coverage test … extended to pin `mint setup`." Every existing `rootUsage` entry is a verb name plus a curated one-line description (e.g. `init    scaffold .mint.toml and the release shim into a repo`). The coverage test (`TestUsageTexts_CoverTheirFlagSets`) asserts the verb name appears in `rootUsage`, so the bare presence is testable — but the human-facing **description text** for the `setup` line is content the README/help surface ships to users and the spec never states it. This is distinct from the deferred "exact command name": even taking `setup` as given, the description ("…?") is undecided. It is small, but it is shipped curated help text, not a planning-only mechanical detail, and the `mint help` "frozen curated text gains only the setup command line" instruction implies a specific line exists to be added. (Note: this is genuinely tiny; flagging as Minor for completeness — an implementer would otherwise invent the wording.)

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 7. "Read the config reference" ordered step depends on `mint setup` output the agent is already reading — the bootstrapping/how is implicit

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: "Emitted guide — setup procedure" (step 3), "Render targets and layering"

**Details**:
Step 3 of the emitted procedure is "**Read the config reference** — an explicit, ordered early step, performed before any inspect/edit … the flow depends on the agent holding the config reference from `mint setup`'s SoT table." But the config reference IS part of the `mint setup` output the agent is already reading to get the procedure (per "What it emits": item 3 is "The config reference — rendered from the config-metadata source of truth"). So "read the config reference" as a procedure step is the agent re-reading a section of the very document it is executing. The intent is probably "internalise the config-reference section before acting" — but as written, an implementer drafting the prose could read it as instructing the agent to fetch the reference from somewhere external (it is not external — `mint help` deliberately omits it, and the README is the *human* surface). The relationship between "the config reference embedded in this guide" and "the step that says read the config reference" should be stated explicitly so the guide prose doesn't send the agent looking for a separate artifact. Minor, but it affects how the (deferred) prose frames a step the spec calls load-bearing ("closes the cold-arrival gap").

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---
