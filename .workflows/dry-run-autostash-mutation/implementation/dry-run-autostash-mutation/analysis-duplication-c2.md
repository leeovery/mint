AGENT: duplication
FINDINGS:
- FINDING: Inlined dry-run+autostash git timeline near-duplicates seedDryRunFirstRelease
  SEVERITY: low
  FILES: internal/engine/release_autostash_test.go:309-320, internal/engine/release_dryrun_test.go:33-47
  DESCRIPTION: The new TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview
    inlines an 11-line `f.SeedSequence("git", ...)` read-side timeline (rev-parse
    --show-toplevel → symbolic-ref → tag --list → fetch --tags → rev-parse --abbrev-ref
    HEAD → rev-parse -q --verify → rev-list → ls-remote → rev-parse HEAD → remote get-url).
    This is byte-for-byte the existing `seedDryRunFirstRelease` helper (release_dryrun_test.go)
    with EXACTLY ONE line removed — the `status --porcelain` clean-tree probe, which is the
    very gate the bypass omits. The two timelines are the same first-release dry-run read
    sequence and will drift together whenever the read-side gate order changes (a sibling
    test, TestRelease_DryRun_NonAutostash_DirtyTreeStillAborts, already maintains a third
    partial copy of the same prefix). Because the only difference is the presence/absence of
    one gate line, the divergence is the meaningful signal and is easy to lose in a hand-copied
    block. Same test package (engine_test), so the helper is already in scope.
  RECOMMENDATION: Reuse the existing helper rather than re-inlining. Either (a) add a sibling
    helper next to seedDryRunFirstRelease — e.g. seedDryRunAutostashReadGates(f, root,
    releaseBranch, tag) — that scripts the identical sequence minus the clean-tree probe and
    have the new test call it, or (b) parameterise seedDryRunFirstRelease with a
    skipCleanTree bool that conditionally seeds the `status --porcelain` line, so the one
    intentional difference is expressed once and the shared prefix can never silently diverge.
SUMMARY: One low-severity near-duplicate: the new dry-run+autostash test hand-copies the
  seedDryRunFirstRelease read-gate timeline minus a single line instead of reusing the helper,
  leaving two copies of the same sequence to drift. The production change itself (boolean
  threaded runPreflight → RunLocalGates, autostash block gated on !opts.DryRun) introduces no
  duplication — it cleanly mirrors the established anyBranch parameter pattern.
