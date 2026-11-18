# Consolidation Tasks: Token Aware Diff Sizing (Phase 9)

## Task 1: Corrections
placement: phase 9
severity: corrections

**Problem**: Spec §18.1 and §18.2, as corrected on 2026-10-21 (consolidation-findings-p9.md S1), have the setup guide's point-of-no-return line and the README's unwind promises name both ways the exclusion check's answers survive an unwind: their commit, re-committed on the starting commit, and their uncommitted write to an untracked `.mint.toml` git ignores. Three sites written to the old §18 wording still name only the commit: the setup guide's pipeline section (internal/setupguide/setupguide.go:263-265), the README's feature list (README.md:30) and its surgical-unwind bullet (README.md:351). They now contradict CLAUDE.md:42 and README.md:117. An agent reading `mint setup` is told an ignored `.mint.toml` write is unwound, when it is kept; an operator whose release aborts in a repo that ignores `.mint.toml` finds their local config changed with no doc saying it would be. The code keeps that write: internal/engine/release.go:1103-1115 records the answers, and when `record.IsIgnored` holds it clears `a.edit` without committing, so the unwind has nothing to revert. internal/engine/unwind.go:11-14 already names both survivals. The old wording is in exactly these three places: `grep -rn "exclusion check's answers commit" --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns 3 matches, README.md:30, README.md:351 and internal/setupguide/setupguide.go:264.

**Solution**: Three one-sentence edits, each naming the ignored write beside the commit; no test pins any of the three sentences.
- internal/setupguide/setupguide.go:263-265: replace "The one exception: after an abort or failure before the push, the exclusion check's answers commit stays, re-committed on the starting commit." with "The one exception: after an abort or failure before the push, the exclusion check's answers stay: their commit is re-committed on the starting commit, and where git ignores an untracked .mint.toml, their uncommitted write is left in place." The sentence sits inside a raw string, so `.mint.toml` carries no backticks, and the lines wrap as their neighbours do.
- README.md:30: replace "The one exception is the exclusion check's answers commit: after an abort or failure before the push it stays on the branch, re-committed on the starting commit." with "The one exception is the exclusion check's answers: after an abort or failure before the push their commit stays on the branch, re-committed on the starting commit, and where git ignores an untracked `.mint.toml`, their uncommitted write is left in place."
- README.md:351: replace "The one exception is the exclusion check's answers commit: once you have answered, it stays on the branch, re-committed on the starting commit, and the unwind says it kept it." with "The one exception is the exclusion check's answers: once you have answered, their commit stays on the branch, re-committed on the starting commit, and the unwind says it kept it; where git ignores an untracked `.mint.toml`, their uncommitted write is left in place."

Derivation: §18.1 and §18.2 as corrected, matching the wording CLAUDE.md:42 and README.md:117 already carry.

**Outcome**: The setup guide's pipeline section and both README unwind promises name both ways the answers survive a pre-push abort or failure: the commit re-committed on the starting commit, and the uncommitted write to an ignored, untracked `.mint.toml` left in place. They now agree with CLAUDE.md:42, README.md:117 and the engine's behaviour. Behaviour is unchanged and every existing test passes as it is.

**Acceptance Criteria**:
- [ ] Running `mint setup` prints, in the pipeline section's point-of-no-return bullet after "nothing after it is.", the sentence "The one exception: after an abort or failure before the push, the exclusion check's answers stay: their commit is re-committed on the starting commit, and where git ignores an untracked .mint.toml, their uncommitted write is left in place." Its `.mint.toml` has no backticks, its lines wrap at the width of the bullet's other lines with the same two-space continuation indent, and the rest of the guide is unchanged.
- [ ] README.md's "One atomic point of no return" feature-list bullet ends with the replacement sentence the Solution gives for README.md:30, and its earlier sentences are unchanged.
- [ ] README.md's "Surgical unwind before the point of no return" bullet ends with the replacement sentence the Solution gives for README.md:351, and its earlier sentences are unchanged.
- [ ] After the change, `grep -rn "exclusion check's answers commit" --include='*.go' --include='*.md' --exclude-dir=.workflows .` returns nothing.
- [ ] The five gates in CLAUDE.md pass with no test added, removed or changed.

**Do**:
- Apply the three replacements the Solution lists, word for word, in internal/setupguide/setupguide.go (the raw string in `pipelineSection`) and README.md. Edit nothing else.
- Leave CLAUDE.md:42, README.md:117 and internal/engine/unwind.go:11-14 as they are. They already carry the corrected wording.
