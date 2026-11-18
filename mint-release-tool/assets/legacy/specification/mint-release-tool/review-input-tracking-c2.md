---
status: complete
created: 2026-06-08
cycle: 2
phase: Input Review
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Input Review

## Findings

### 1. Why diff-exclusion is path-based (range diffs can't subtract commits) — rationale dropped

**Source**: `discussion/mint-release-tool.md` — "AI release notes — quality" › Diff exclusion tiers, lines 658–659 ("Why regenerate needs this and forward doesn't")
**Category**: Enhancement to existing topic
**Affects**: Stage 4 — AI Release Notes › Diff exclusion (two tiers + strategy-aware version file)

**Details**:
The discussion closes the diff-exclusion tiers with a structural rationale the spec drops: the forward path diffs at a HEAD that *predates* mint's bookkeeping commits, whereas regenerate diffs a tag range (`vX-1..vX`) that *already contains* them — so path-exclusion (CHANGELOG always; `version_file` in plain mode) is how regenerate reproduces the forward path's *source* view. The discussion then pins the key constraint that makes this the only available mechanism: *"a 🌿/mint commit prefix is cosmetic only — diffs are range-based and can't subtract commits, so exclusion stays path-based."*

The spec states *what* is excluded per path/mode but never explains *why exclusion must be path-based rather than commit-based* — i.e. that a range diff cannot drop mint's own bookkeeping commits even though they carry a recognisable `commit_prefix`, because git range diffs operate on paths/content, not commit identity. This is load-bearing: without it, a future reader could reasonably ask "why not just exclude mint's release-bookkeeping commits from the range?" and the spec gives no answer. It also ties the `commit_prefix`'s "cosmetic only" status (spec line 607) to a concrete consequence.

**Current**:
- **`version_file` — NOT blanket-excluded (strategy-aware):**
  - *Forward path:* nothing to exclude — notes generate (Stage 4) *before* the version write (Stage 5), so the file is inherently unchanged at notes time. (The whole concern is therefore **regenerate-only**.)
  - *Regenerate, plain mode* (whole file is the version, e.g. `release.txt`): **exclude** the file — pure bookkeeping.
  - *Regenerate, embedded mode* (`version_pattern` in a real source file like `main.go`): **do not exclude** — it's source we want in notes. The lone version-line bump is negligible and neutralised by the default prompt's "ignore version-number bumps" instruction, not by hiding real code.

**Proposed Addition**:
Add a short rationale note to the diff-exclusion section, e.g.: "Exclusion is **path-based, never commit-based**: the forward path diffs a HEAD that predates mint's release-bookkeeping commits, while regenerate diffs a tag range (`vX-1..vX`) that already contains them. A git range diff operates on paths/content and cannot subtract commits, so even though mint's bookkeeping commit carries a recognisable `commit_prefix` (cosmetic only), it cannot be dropped from the range — path exclusion is what reproduces the forward path's source view on the regenerate path."

**Resolution**: Approved
**Notes**: Added path-based-exclusion rationale note to the Diff exclusion section. (User selected auto mode at this finding.)

---
