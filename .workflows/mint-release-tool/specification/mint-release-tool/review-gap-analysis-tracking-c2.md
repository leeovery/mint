---
status: in-progress
created: 2026-06-08
cycle: 2
phase: Gap Analysis
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Gap Analysis

## Findings

### 1. Provider auto-detection with no matching driver (non-GitHub remote, no `provider` override) is undefined

**Priority**: Important
**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Stages 6–7 — Publishing: provider driver abstraction (lines 426–428); Config — `provider` key (line 615)

**Details**:
The publishing section says mint "auto-detects the provider from the remote host (`github.com` → GitHub driver via `gh`)" (line 426), and `provider` defaults to "auto-detected from remote host" (line 615). Cycle 1 #3 resolved the case where `provider` is set to a *recognised key with an unsupported value* (warn + downgrade to tag+push-only). But the **auto-detection failure** case is different and unspecified: with `publish = true` (the default) and **no explicit `provider`**, what happens when the remote host is **not** `github.com` — e.g. a GitHub Enterprise host (`github.mycorp.com`), a GitLab/Gitea remote, an SSH remote whose host can't be matched, or **no remote at all**? Auto-detection yields no driver. The three plausible behaviours diverge sharply: (a) fail-loud abort before the tag, (b) warn + downgrade to tag+push-only (mirroring the unsupported-value resolution), or (c) silently assume GitHub. This is on the default path (publish defaults true), so it is not a rare config-only edge — any project mint releases off a non-github.com remote hits it. An implementer must pick a rule, and the wrong pick either strands tags or silently never publishes.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 2. Dry-run note cache is structurally unreachable for any project with a `pre_tag` hook

**Priority**: Important
**Source**: Specification analysis
**Affects**: Dry-Run — note caching, cache key (line 478); Dry-run skips hooks (line 470); Stage 4 — Diff base computed at post-hook HEAD (line 234)

**Details**:
The dry-run note cache exists to guarantee "what was previewed is what ships" via a key match (lines 474–477). The cache key is "hash of (post-`diff_exclude` diff + computed version + prompt/context)" (line 478). But dry-run **skips all hooks** (line 470), whereas the forward notes diff is "computed at the post-hook HEAD" — `pre_tag` hooks commit before notes generate (line 234). Therefore, for any project with a `pre_tag` hook that dirties the tree, the **dry-run computes the diff at a pre-hook HEAD while the real run computes it at a post-hook HEAD**. If the hook changes any *non-excluded* path, the two diffs differ → the cache key always misses → the real run always regenerates ("diff changed since dry-run preview"). Line 478 even cites "a `pre_tag` hook can change the tree between runs" as the *reason* the key isn't HEAD-sha-based — but the chosen key (post-`diff_exclude` diff) doesn't actually solve this, because the diff itself differs between a hook-skipped dry-run and a hook-run real run. The net effect: the cache's headline determinism guarantee silently does not hold for the exact projects (those with build hooks) most likely to use the dry-run→real-run agent workflow. The spec should either state that the cache is best-effort and reuse is not expected when a `pre_tag` hook materially changes the diff, or define how the dry-run preview accounts for hook artifacts. Without this an implementer builds a cache that appears to work in tests (no hooks) and never reuses in the motivating real-world case.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 3. `regenerate --fresh -y` with no `--target` has no defined default surface

**Priority**: Important
**Source**: Specification analysis
**Affects**: CLI — `regenerate` flags `--target` default (line 581); Regenerate — Interactive by default / flags to skip (lines 525, 528, 534); Two-axis contract (line 503)

**Details**:
`--target` is documented as "default: asked interactively" (line 581), and "Flags skip the questions but still confirm unless `-y`" (line 528). For `--reuse` the target is pinned ("implies `--target release`", line 579). But for the **fresh** path, the target is genuinely a three-way choice (`release | changelog | both`, line 503) with **no stated default**. When a user runs `regenerate <ver> --fresh -y` (or `--fresh --all -y`) — `-y` skips the interactive ask (line 583) — there is no interactive prompt to supply the target, and no documented fallback. The implementer cannot tell whether this should error ("`--target` required with `-y --fresh`"), default to `both`, default to `release`, or default to `changelog`. This is squarely on the scripted/CI path that `-y` exists to serve, so it is not a corner case. One sentence is needed to define the fresh-path target resolution under `-y` (either a default value or a required-flag error).

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 4. `mint release regenerate` invoked with neither `<version>` nor `--all`

**Priority**: Minor
**Source**: Specification analysis
**Affects**: CLI Surface — Commands (lines 552–553); Regenerate — Interactive by default (line 525)

**Details**:
The CLI lists two regenerate forms: `regenerate <version>` and `regenerate --all` (lines 552–553). The spec never states what happens when **both the `<version>` argument and `--all` are omitted** (a bare `mint release regenerate`). Possible readings: an error ("specify a version or `--all`"), an interactive prompt for which version, or regenerate the latest. The opposite collision — **both** `<version>` *and* `--all` given — is also undefined (does `--all` win, or is it an error?). Since regenerate is "interactive by default" (line 525), a reader might assume bare-regenerate prompts for the version, but the interactive default described concerns source/target, not version selection. A one-line rule for the missing-argument and conflicting-argument cases removes the guess.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 5. `regenerate --reuse` against a tag with no annotation body

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Body Distribution — tag is the single source mint reads (line 338); Regenerate — `--reuse` source (line 498); Failure model — heal path (line 416)

**Details**:
`--reuse` "reads the tag annotation body" as the deterministic single source of truth (lines 338, 498), and the post-PONR heal path tells the user to run `regenerate --reuse` to recreate a failed provider release (line 416). This assumes the tag carries an annotation body — true for any tag mint created. But `--reuse` operates on *existing* tags, which may include tags **not created by mint**: a **lightweight tag** (no annotation at all) or an annotated tag with an **empty/whitespace body**. `git for-each-ref … contents:body` on such a tag returns empty. The spec doesn't define what `--reuse` does with an empty source: error ("tag has no annotation body — use `--fresh`"), fall through to fresh, or write an empty release body. This matters most in the `--reuse --all` mass-heal case (line 515) across a repo with mixed-provenance tags, where one bodyless legacy tag would otherwise silently produce an empty provider release. A one-line rule (e.g. empty annotation → skip-and-report in `--all`, error in single) closes it.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 6. `version_pattern` matching multiple occurrences in the version file is undefined

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Stage 5 — Version-file projection (line 386); Stage 1 — version_pattern (line 96)

**Details**:
Version-file projection replaces `version_pattern` (e.g. `RELEASE_VERSION="{version}"`) in the configured file, and the spec defines the **zero-match** case (pattern matches nothing → abort before tag, line 386). It does not define the **multiple-match** case: when the pattern matches more than one line/occurrence in the file, does mint replace all occurrences, only the first, or treat multiple matches as an error? The legacy `sed`-replace model (referenced at line 101) typically replaces all, but the spec doesn't carry that forward. An implementer must choose, and "first only" vs "all" produce a file in an inconsistent state on a multi-match file. One line resolves it.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---

### 7. `e` edit behaviour when `$EDITOR` is unset

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Interactive Confirmation & Notes Review — `e` edit (line 451)

**Details**:
The `e` edit choice "opens the notes in `$EDITOR`" (line 451). This section explicitly owns "the four semantic choices and their effects" (the rendering of the gate is deferred to the CLI Presentation spec, line 454), so the *behaviour* of `e` is in scope here. The spec doesn't state what happens when **`$EDITOR` is unset** (a common case in minimal CI containers and fresh environments): does mint fall back to a sensible default (`vi`/`nano`), error, or re-show the gate? Since `-y` skips the gate for CI, an interactive run reaching `e` in an `$EDITOR`-less environment is plausible. A one-line fallback rule (default editor, or a clear error returning to the gate) avoids an implementer either crashing or guessing a fallback binary.

**Proposed Addition**:

**Resolution**: Pending
**Notes**:

---
