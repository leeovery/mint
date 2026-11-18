---
scope: mint-release-tool
cycle: 1
source: review
total_findings: 80
deduplicated_findings: 9
proposed_tasks: 9
---
# Review Report: Mint Release Tool (Cycle 1)

## Summary
QA verdict is **Request Changes**. Across 80 verified per-task reports plus an external audit, the implementation is consistently high quality with one originally-flagged blocking issue (interactive-regenerate preflight bypass); the audit surfaced five further must-fix defects and promoted three previously-latent bugs into the blocking set, giving **nine authoritative Required Changes (#1–#9)**. All nine are normalized into self-contained, independently-executable tasks below, preserving each finding's file:line, fix, and add-a-test acceptance. The remaining ~70 findings are a long tail of non-blocking polish — zero-risk doc fixes already applied this session, plus brittleness/test-tightening recommendations that fall below the severity/clustering threshold and are discarded here.

## Dedupe / grouping notes
- **Required Change #2** absorbs report Idea #17 (the shared `ResolvePublisher` helper) and quick-fix #13 (cmd-level unresolved-publisher test) — the discarded resolve error is a crash, not cleanup, so a single task replaces all three. No separate tasks emitted for #17/#13.
- **Required Changes #7/#8/#9** were promoted from the report's former Bugs section; corroborating non-blocking notes report-2-1 (timeout misclassification), report-2-13 (whitespace `$EDITOR` panic), and report-5-12 (batch-skip spinner leak) are deduped into #7/#8/#9 respectively rather than emitted as duplicates.
- **Required Change #9** covers BOTH the `--all` notes-failure-skip spinner leak (report-5-12) AND the real-run cache-reuse/miss/unreadable Warn-inside-blocking-stage case (external audit) as one general fix.
- **Required Change #1** is the same finding in report-5-10 (blocking) and report-5-4 — one task, both sources.

## Discarded Findings
- **Do-now doc/comment fixes (report 1-1, 2-2, 7-2, 1-11, 5-8, 3-3, 4-10, 5-13, 3-8, 2-14, 5-4, 5-6, 6-1, 6-4, 6-5, 6-8)** — zero-risk; already applied this session per `report.md` "Do now". Not re-proposed.
- **gh-stderr substring-match brittleness (Ideas #18; reports 1-8, 5-7)** — design decision (harden against gh wording/locale vs accept), not a correctness defect; below threshold.
- **go-toml/v2 error-text coupling (Idea #19; reports 6-1, 6-3, 6-4, 1-2)** — fragile but test-guarded today; deriving structurally is a design call, not a defect.
- **Config-validation test-guard quick-fixes (#9; reports 6-3, 6-1, 6-2)** — test-tightening only; deferrable, does not cluster into a correctness pattern.
- **reuse `--all` double-read of tag annotation (quick-fix #7; reports 5-11, 5-12)** — performance polish on large backfills; net-correct today.
- **`gatePerVersion` inert `Target` omission (quick-fix #8; report 5-11)** — inert (gate ignores Target); fragility note only.
- **bookkeeping predicate duplication (quick-fix #6; reports 3-7, 3-8)** — DRY refactor of two in-sync code paths; no observed desync.
- **Test-coverage tightening (quick-fixes #10–#12, #16; reports 7-1/7-2/7-3, 8-1/5-9, 1-3/5-3/1-5/2-5/1-9/3-6/3-1/2-12/2-14, 7-4/4-7/4-7a/4-8, 1-11/6-8)** — incremental seam/error-path coverage; none guards a known live defect.
- **Re-tagged label/comment nits (#16a report 2-4, #16b report 3-3)** — cosmetic; touch only test assertions/comments.
- **Remaining Ideas #14, #15, #20–#41** — design questions, forward-compat scaffolding decisions, and latent-only observations; none is a current correctness defect and none clusters into a worthwhile pattern.
