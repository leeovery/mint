---
topic: ai-model-selection
cycle: 3
total_findings: 4
deduplicated_findings: 4
proposed_tasks: 0
---
# Analysis Report: AI Model Selection (Cycle 3)

## Summary
Standards returned CLEAN; duplication and architecture returned only low-severity findings (one duplication finding the agent labelled medium reduces, on the merits, to a coverage-loss tradeoff). All four findings were assessed individually and none cluster into an actionable pattern or survive the "discard low-severity unless they cluster" filter — each is a non-clustered nit, bikeshedding, already-covered, or net-negative to action (coverage loss). No high-severity findings exist. Zero tasks proposed; the implementation is clean.

## Discarded Findings
- Three near-identical per-site white-box transport-wiring test files (duplication) — Discarded as net-negative. The three per-site proofs (release_aitransport_internal_test.go, regenerate_fresh_aitransport_internal_test.go, run_aitransport_internal_test.go) verify something the isolated aitransport_test.go helper test does NOT: that each site wires the CORRECT verb through aitransport.New — release vs commit, and crucially that regenerate rides VerbRelease (the spec's explicit "easy miss"). Standards confirmed regenerate-routes-through-release has direct exact-assertion coverage. The recommendation to drop the per-site zero/positive-deadline scenarios or collapse to one minimal proof would trade away the per-verb delegation guarantee — particularly the regenerate-routing un-missable proof — for scaffolding reduction. Consolidating here is a coverage loss, not an improvement.
- Duplicated argv-matching FakeRunner helpers across commit and engine packages (duplication) — Discarded as an isolated low-value nit that does not cluster. The agent itself notes engine's stdinOf/invokedWith live in release_test.go, which is OUT of plan scope; only the commit-side aiInvocationStdin/invokedBinary copy is in-scope and net-new. The deadline-runner test-support pattern this might cluster with was already extracted onto the runner production surface in cycle 2, so no live pattern accrues here. A single in-scope copy of a trivial iterate-match-return loop, whose counterpart cannot be touched without scope creep, does not justify a new production test-surface API.
- DurationPtr co-located in the deadline-recording-runner file (architecture) — Discarded as bikeshedding. The agent explicitly calls it "a minor seam/cohesion nit, not a defect," states the placement is "correct in spirit" (shared cross-package production-file home, mirroring FakeRunner), and recommends "do not churn if it would ripple imports." Renaming the host file for a one-line helper is churn without behavioural or contract benefit.
- Timeout lacks the full TOML-to-transport integration proof ai_command has (architecture) — Discarded as already-covered. The agent's own description confirms the seam is "covered link-by-link" (config.Load→TimeoutFor in config_test.go; TimeoutFor→runner-deadline white-box via DeadlineRecordingRunner at all three sites), "the composition holds and this is not a correctness risk," and marks the fix "Optional — the per-link coverage already proves the chain." A dedicated TOML-to-transport timeout test would re-prove links already individually proven; it is symmetry-for-its-own-sake against ai_command's shape, not a genuine coverage gap.
