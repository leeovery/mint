TASK: commit-command-7-1 — Structurally single-source the per-mode git source selection shared by the preflight probes and the L1 diff sources (tick-59ad40; Phase 7: Analysis Cycle 2)

ACCEPTANCE CRITERIA:
- Each per-mode git argv prefix and the `-- .` selector is spelled exactly once and consumed by both the preflight probe and the L1 source.
- The preflight probe argv is the shared prefix plus `--name-only` (diff cases) / the shared `ls-files` prefix (untracked case), with no independently re-spelled verb/refspec/selector.
- The StagingMode→sources mapping, including the AddAll tracked-then-untracked short-circuit, is defined in one place consumed by both the emptiness path and the diff path.
- The empty-staging preflight cluster (3 sentinels, checkSomethingToCommit, wouldStageNothing, *ProbeArgs, emptyStagingError, gitOutputEmpty) plus the shared selection helper live in internal/commit/preflight.go; run.go no longer contains them.
- The AddAll probe still uses `--name-only` (does not render each untracked file body).
- go vet ./..., golangci-lint run, and the commit tests pass; the verbatim empty-staging messages and per-mode emptiness verdicts are unchanged.

STATUS: Complete

SPEC CONTEXT:
The commit spec makes "the preflight and the AI's L1 diff read ONE exclusion-filtered source and cannot drift" a load-bearing invariant (Preflight & Safety; Staging Model → Empty-staging handling). The empty-staging path must (a) never invoke the AI on an empty diff, (b) fire the correct verbatim git-style message keyed on the ACTUAL post-mode tree state rather than the flag passed (clean-tree vs no-changes-staged vs no-tracked-changes), and (c) measure the post-exclusion set so an all-excluded staged set fails loud before generate. The AddAll source is "tracked then untracked," read-only, computed without mutating the index. This task is a pure refactor: single-source the per-mode source selection so the invariant is structural, not two hand-aligned copies, and colocate the preflight cluster in its own file.

IMPLEMENTATION:
- Status: Implemented (clean refactor, behaviour-preserving)
- Location:
  - internal/commit/source.go (new, 109 lines) — the single source-of-truth: sourceKind/sourceSpec, stagedBaseArgs/trackedBaseArgs/untrackedBaseArgs (the `[verb, refspec/flags, "--", "."]` prefixes spelled once), sourcesForMode (the single StagingMode→sources descriptor), sourceArgs (base + shared excludePathspecs tail).
  - internal/commit/preflight.go (new, 198 lines) — the moved cluster: errNothingToCommit/errNoChangesStaged/errNoTrackedChanges sentinels, checkSomethingToCommit, wouldStageNothing (an all-specs-empty fold with first-non-empty short-circuit, driven by sourcesForMode), probeArgs/nameOnly, stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs, emptyStagingError, gitOutputEmpty.
  - internal/commit/generate.go:156-250 — sourceDiff/renderSource/diffSourceText/untrackedAdditions now build argv from the shared prefixes via sourceArgs; the per-mode switch is gone, replaced by iterating sourcesForMode(g.mode). The AddAll composition lives only in sourcesForMode.
  - internal/commit/run.go:294 — now only CALLS checkSomethingToCommit (orchestration); the 147-line cluster was removed (confirmed: no probe/sentinel/source definitions remain in run.go).
- Notes:
  - All six acceptance criteria are met. Verified the move via git show 4bf0695 --stat: run.go -147, preflight.go +192, source.go +104, generate.go rewritten.
  - The "one source, cannot drift" invariant is now structural: a diff probe is provably sourceArgs(nameOnly(base), exclude) and the L1 diff is sourceArgs(base, exclude) — differing only by the spliced `--name-only`; the untracked probe reuses the ls-files prefix verbatim. The AddAll `--name-only` lightness (probe never renders each untracked body) is preserved — the untracked probe is the ls-files enumeration, not a per-file --no-index render.
  - nameOnly slices base[:len-2]/base[len-2:]; safe because every *BaseArgs builder returns a >=4-element slice ending in `-- .`. The append(append([]string{},...)...) copy-first idiom avoids aliasing the literals the builders return.
  - One genuine observation (non-blocking): the production preflight path (wouldStageNothing → probeArgs) does NOT consume the three named stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs builders — those are now referenced ONLY by source_test.go. They remain as the spec-mandated "single checkable builders" for the white-box property test. No drift risk in practice (the test seeds excludes := excludePathspecs(exclude), so the named builders and probeArgs agree by construction), and the end-to-end tests exercise probeArgs through Run. But the structural guarantee the named builders assert is one step removed from the argv production actually runs.

TESTS:
- Status: Adequate
- Coverage:
  - internal/commit/source_test.go (new, white-box package commit):
    - TestProbeArgv_IsL1SourceArgvPlusNameOnly — staged/tracked probe == L1 sourceArgs + `--name-only`; untracked probe == shared ls-files prefix verbatim == untracked L1 sourceArgs. Directly satisfies the spec's first required test.
    - TestProbeArgv_DerivesFromSharedBase — asserts each diff probe is the shared base with exactly one extra element (`--name-only`) spliced after the refspec and the untracked probe is the base verbatim, against the shared *BaseArgs (not a re-spelled literal) — making the single-sourcing structural.
    - TestEmptinessVerdictAgreesWithL1Source_PerMode — 7 sub-cases across StagedOnly/All/AddAll proving wouldStageNothing and sourceDiff agree (both empty / both non-empty) through sourcesForMode, including "addall tracked short-circuits" (untracked probe never read) and "addall untracked only". Satisfies the spec's second required test.
  - Behaviour-parity tests (unchanged, still exercising the new shared path end-to-end via Run): staging_empty_test.go (clean-tree / no-changes-staged / no-tracked-changes verbatim messages keyed on tree state; no-AI / no-add / no-commit on empty), staging_excluded_test.go (post-exclusion probe measures the all-excluded set → fail loud before generate), generate_test.go (L1 source rendering).
- Notes:
  - Not under-tested: both spec-required tests present; the load-bearing verbatim messages and tree-state-keyed selection are covered by the pre-existing end-to-end suite, confirming parity after the refactor.
  - Not over-tested: the two argv tests overlap slightly (both assert the diff-probe = base + --name-only shape), but they assert it from different angles (literal-equality vs derived-from-shared-base) and the redundancy is cheap and intentional for the "structural" claim. Acceptable.
  - The emptiness/L1-agreement test seeds the FakeRunner with the expected number of scripted calls per mode but does not assert the exact argv each probe/source issued — it relies on the FakeRunner's seeded sequence. The argv equality is covered separately in the two ProbeArgv tests, so the split coverage is sound.

CODE QUALITY:
- Project conventions: Followed. Unexported helpers, consumer-side seam (CommandRunner/Transport) injected, read-only RunInDir(root) anchoring consistent with generate. Error wrapping with %w preserves errors.Is matching; sentinels returned unwrapped so verbatim text survives to the presenter (matches the surface helper contract).
- SOLID principles: Good. source.go holds selection (one reason to change: the per-mode argv shape); preflight.go holds emptiness orchestration; generate.go holds rendering. The shared descriptor (sourcesForMode) is the single dependency both consumers invert onto.
- Complexity: Low. wouldStageNothing is a linear all-specs-empty fold; renderSource is a 2-way kind switch; nameOnly/probeArgs/sourceArgs are small pure helpers.
- Modern idioms: Yes. Slice copy-first appends avoid aliasing; NUL-split on the -z enumeration is correct; iota-based sourceKind.
- Readability: Good — arguably above average. Doc comments are thorough (the file headers explain WHY the single-sourcing is structural). They lean verbose, but for a load-bearing invariant that is defensible.
- Issues: None blocking. See the non-blocking note about the named *ProbeArgs builders being test-only consumers.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/commit/preflight.go:143-153 — stagedProbeArgs/trackedProbeArgs/untrackedProbeArgs are now consumed ONLY by source_test.go (production uses probeArgs(spec, diffExclude)). They are kept deliberately as the spec's "single checkable builders," but the property they assert is one indirection removed from the argv production runs. Consider either (a) having wouldStageNothing call the named per-mode builders so the test checks the exact production argv, or (b) adding one assertion that probeArgs(spec, exclude) equals the matching named builder, closing the small gap between what is tested and what runs. Requires a judgement call on which direction (and whether the gap is worth closing given the end-to-end coverage), hence idea not quickfix.
