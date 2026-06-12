AGENT: architecture
FINDINGS:
- FINDING: Test-only per-mode probe builders duplicate probeArgs's argv derivation
  SEVERITY: low
  FILES: internal/commit/preflight.go:131-148, internal/commit/preflight.go:113-118
  DESCRIPTION: stagedProbeArgs / trackedProbeArgs / untrackedProbeArgs live in production
    code (preflight.go) but are called ONLY from tests (source_test.go) — production's
    emptiness path goes through probeArgs(spec, diffExclude) inside wouldStageNothing,
    never these three. They also use a DIFFERENT parameter convention from production:
    they take already-mapped exclude pathspecs and append them directly, whereas probeArgs
    takes the raw diffExclude globs and re-derives the tail via excludePathspecs. The
    result is a second, parallel place the probe argv is assembled — the exact "two
    hand-aligned copies" the source.go single-sourcing was designed to eliminate, only now
    reintroduced as a test-facing surface. Their own doc comment concedes the divergence
    ("They take the pre-mapped excludes for symmetry with their historical callers"),
    confirming they are vestigial. If a future change to probeArgs/nameOnly (e.g. a base
    that omits the `-- .` selector, or a different exclusion tail) is made, these builders
    would silently drift from production and the tests asserting against them would pass
    while exercising a stale argv shape — eroding the single-source invariant they appear
    to validate. The genuinely structural test (TestEmptinessVerdictAgreesWithL1Source and
    TestProbeArgv_IsL1SourceArgvPlusNameOnly via sourceArgs) already proves the invariant
    against the shared base builders; the per-mode wrappers add maintenance surface without
    adding coverage that the shared-base assertions don't.
  RECOMMENDATION: Either delete the three per-mode wrappers and have source_test.go assert
    against probeArgs(spec, diffExclude) directly (the production entry point, with the raw
    diffExclude convention), or — if a named per-mode checkable is wanted — make them
    one-line forwarders to probeArgs over the matching sourceSpec so there is exactly one
    derivation. Do not keep a parallel builder with a divergent parameter convention.
SUMMARY: The commit implementation composes cleanly across all task seams — the L1/L2/L3
  layering, the single Mutator/Transport/Deps seams, the source.go single-sourced per-mode
  descriptor shared by preflight and L1, the one shared commitAccept tail both accept paths
  reach, the editorUnavailable single predicate, and the routing sentinels are all
  well-scoped, well-typed, and validated by an unusually thorough end-to-end Run-level
  integration suite. The only structural smell is a set of test-only probe-argv builders in
  preflight.go that duplicate production's probeArgs with a divergent parameter convention,
  a low-severity drift risk against the single-source invariant they nominally guard.
