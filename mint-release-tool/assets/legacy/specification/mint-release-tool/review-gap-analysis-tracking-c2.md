---
status: complete
created: 2026-06-08
cycle: 2
phase: Gap Analysis
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Gap Analysis

*(Cycle 2. finding_gate_mode = auto — all findings resolved with reasoned defaults consistent with existing decisions; none were genuine design forks. #2 reduced to a clarification on analysis.)*

## Findings

### 1. Provider auto-detection with no matching driver (non-GitHub remote, no `provider` override) is undefined

**Priority**: Important
**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Stages 6–7 — Publishing: provider driver abstraction; Config — `provider` key

**Details**:
With `publish = true` (default) and no explicit `provider`, auto-detection against a non-`github.com` remote (GHE, GitLab, Gitea, unmatchable SSH host) or no remote yields no driver — behaviour undefined (fail-loud / downgrade / silently-assume-GitHub all plausible). On the default path.

**Proposed Addition**:
New Publishing bullet: auto-detection with no matching driver is treated the same as an unsupported value → warn loudly + downgrade to tag + push only; never assume GitHub, never strand a tag.

**Resolution**: Approved
**Notes**: Consistent with cycle-1 #3 (unsupported provider value → warn + downgrade).

---

### 2. Dry-run note cache is structurally unreachable for any project with a `pre_tag` hook

**Priority**: Important
**Source**: Specification analysis
**Affects**: Dry-Run — note caching cache key; Dry-run skips hooks; Stage 4 — Diff base at post-hook HEAD

**Details**:
Dry-run skips hooks while the real run's diff is computed at post-hook HEAD; the agent worried the post-`diff_exclude` cache key always misses for hook-using projects, defeating determinism.

**Proposed Addition**:
Clarifying bullet: the key is the *post-`diff_exclude`* diff, so it is invariant to hook artifacts under `diff_exclude` (the normal case — built bundles are excluded) → reuse holds despite dry-run skipping hooks. If a hook changes a non-excluded path, the key correctly misses and regenerates (the shipped diff genuinely differs from the preview).

**Resolution**: Approved
**Notes**: On analysis this is working-as-intended, not a flaw — reduced to a clarification. The motivating dogfood case (agentic-workflows) excludes its bundle, so its cache matches.

---

### 3. `regenerate --fresh -y` with no `--target` has no defined default surface

**Priority**: Important
**Source**: Specification analysis
**Affects**: CLI — `regenerate` `--target` default; Regenerate — Interactive by default / flags to skip

**Details**:
`-y` skips the interactive target ask; the fresh path is a three-way `release|changelog|both` with no stated default. `regenerate --fresh -y` (and `--fresh --all -y`) has no defined target resolution.

**Proposed Addition**:
New bullet: fresh + `-y` without `--target` is a fail-loud error ("`--target` is required with `--fresh -y`"); mint never guesses which live surface(s) to rewrite unattended. `--reuse` unaffected (implies `--target release`).

**Resolution**: Approved
**Notes**: Fail-loud-require-flag chosen over a silent default — consistent with the tool's safety ethos for live-surface rewrites.

---

### 4. `mint release regenerate` invoked with neither `<version>` nor `--all`

**Priority**: Minor
**Source**: Specification analysis
**Affects**: CLI Surface — Commands; Regenerate — Interactive by default

**Details**:
Missing-argument (bare `regenerate`) and conflicting-argument (`<version>` + `--all`) cases undefined.

**Proposed Addition**:
Added to "Version argument & diff base resolution": bare regenerate (neither) → error ("specify a version or `--all`"); both given → error (mutually exclusive).

**Resolution**: Approved
**Notes**:

---

### 5. `regenerate --reuse` against a tag with no annotation body

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Body Distribution — tag is single source read; Regenerate — `--reuse` source; Failure model — heal path

**Details**:
`--reuse` assumes an annotation body, but `--reuse` operates on existing tags that may be lightweight or empty-bodied (mixed-provenance tags in `--all` mass-heal).

**Proposed Addition**:
Added bullet: `--reuse` with no annotation body → fail-loud error in single mode ("use `--fresh`"); skipped-and-reported in `--all` mode; never written as an empty release body.

**Resolution**: Approved
**Notes**: Skip-and-report in `--all` is consistent with batch skip-and-continue.

---

### 6. `version_pattern` matching multiple occurrences in the version file is undefined

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Stage 5 — Version-file projection; Stage 1 — version_pattern

**Details**:
Zero-match is defined (abort); multiple-match (replace all / first / error) is not.

**Proposed Addition**:
Added bullet: multiple matches → replace **all** occurrences (carries forward legacy `sed`-replace semantics), keeping the file consistent.

**Resolution**: Approved
**Notes**:

---

### 7. `e` edit behaviour when `$EDITOR` is unset

**Priority**: Minor
**Source**: Specification analysis
**Affects**: Interactive Confirmation & Notes Review — `e` edit

**Details**:
The `e` choice opens `$EDITOR`; unset-`$EDITOR` behaviour (common in minimal containers) undefined.

**Proposed Addition**:
Editor resolution: `$VISUAL` then `$EDITOR`, falling back to a sensible default (`vi`); if none can launch, report and return to the gate rather than crash.

**Resolution**: Approved
**Notes**:

---
