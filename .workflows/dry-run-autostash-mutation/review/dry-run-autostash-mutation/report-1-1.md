TASK: dry-run-autostash-mutation-1-1 — Gate Autostash On Dry-Run; Bypass Clean-Tree Gate For Dry-Run+Autostash

ACCEPTANCE CRITERIA:
- Project gates green: go build ./... ; gofmt -l . (empty) ; go vet ./... ; go test -race ./... ; golangci-lint run (0 issues).
- Engine test: --dry-run --autostash against a DIRTY tree issues ZERO `git stash push`/`git stash pop` mutations (exact argv via FakeRunner) AND still completes the full preview (no clean-tree abort, dry-run summary rendered).
- Preflight test: RunLocalGates(..., skipCleanTree=true) does NOT run the clean-tree probe (`git status --porcelain` absent) while branch + tag-free gates still run; skipCleanTree=false unchanged.
- Regression guards: real --autostash (non-dry) still stashes/pops; non-autostash dry run on a dirty tree still aborts at the clean-tree gate.
- Edge case: bypass conditioned on DryRun && AutoStash, never DryRun alone.

STATUS: Complete

SPEC CONTEXT:
The spec (specification.md) settles Option A: skip the real stash entirely in dry-run so a dry run is provably mutation-free (never reaches git.Mutator). Because --autostash's only job is to clean the tree for the clean-tree preflight gate, skipping the stash forces a conditional bypass of that gate — but ONLY for the DryRun && AutoStash combo, mirroring the existing --any-branch gate-skip pattern. Observable dry-run preview must stay byte-identical (the notes diff is lastTag..HEAD, never WIP-dependent). Exclusions: no change to real-run autostash, non-autostash dry-run, other gates, and no new flags/config/events. CLAUDE.md adds the load-bearing invariant that a dry run NEVER reaches the Mutator and leaves the repo byte-for-byte unchanged.

IMPLEMENTATION:
- Status: Implemented (all 5 planned steps landed; clean scope)
- Location:
  - Step 1 — internal/engine/release.go:392 — autostash block gated `if opts.AutoStash && !opts.DryRun {` so neither autostashPush nor the deferred autostashPop runs under dry-run.
  - Step 2 — internal/preflight/preflight.go:81 — RunLocalGates signature gains `skipCleanTree bool` (mirrors anyBranch); :82-86 wraps CheckCleanTree in `if !skipCleanTree`, skipping the `git status --porcelain` probe entirely. Doc comment updated at :74-80.
  - Step 3 — internal/engine/release.go:1106 — runPreflight gains `skipCleanTree bool` and forwards it to RunLocalGates (:1110). Doc comment updated at :1101-1105.
  - Step 4 — internal/engine/release.go:413 — call site passes `opts.DryRun && opts.AutoStash` as skipCleanTree.
  - Step 5 — WHY-comments updated: ReleaseOptions.AutoStash field (release.go:246-250), ReleaseOptions.DryRun field (:262-272, now states the autostash combo guarantee holds), Stage-2 autostash block comment (:385-391), Release function doc (:296-301), runPreflight comment, RunLocalGates comment, and autostash.go header (:25-28).
- Notes:
  - The condition is correctly `DryRun && AutoStash`, never DryRun alone — verified at the single call site (release.go:413). No second path can trigger the bypass.
  - The `--any-branch` skip pattern is mirrored faithfully: skipCleanTree is a separate boolean threaded the same way as anyBranch, evaluated independently (an && of the two skip guards never collides).
  - Scope is clean: implementation commit b1396e9 touches exactly internal/engine/release.go, internal/preflight/preflight.go, internal/engine/autostash.go and their tests (+ manifest/tick bookkeeping). No new flags, config keys, presenter events, or README changes — consistent with spec exclusions. No scope creep, no orphaned code.

TESTS:
- Status: Adequate
- Coverage:
  - Engine (release_autostash_test.go:301 TestRelease_DryRunAutostash_DirtyTree_NoStashMutation_CompletesPreview): seeds a DIRTY-tree dry-run+autostash timeline with the clean-tree probe slot ABSENT and NO mutation tail. Asserts ZERO `git stash push --include-untracked` (:318), ZERO `git stash pop` (:321), ZERO `git status --porcelain` (:325, gate bypassed), no gh (:330-334), assertNoMutation (:329), terminal RunFinished (:336-339), and NO StageFailed (:341-343). This is precisely the acceptance criterion — exact-argv mutation-free preview that still completes.
  - Preflight (preflight_test.go:262 TestRunLocalGates_SkipCleanTree_SkipsCleanTreeGate): with skipCleanTree=true and NO porcelain response seeded, asserts the `status --porcelain` call is never recorded (:279-283) and that exactly 2 local-gate calls ran — on-branch + tag-free (:285). skipCleanTree=false remains exercised by TestRunLocalGates_AllPass (:214), TestRunLocalGates_AnyBranch_* (:230/:255), and TestRunLocalGates_CheapFirstAbort (:300).
  - Regression — real (non-dry) autostash: existing 4-4 suite intact (StashesBeforeGate_DirtyTreePasses, PopsAfterSuccessfulRelease, AbortUnwindsBeforePop, PopConflict variants, NoWIP_IsNoOp) all still pass — proving the real-run path is unchanged.
  - Regression — non-autostash dirty dry run (release_dryrun_test.go:318 TestRelease_DryRun_NonAutostash_DirtyTreeStillAborts): seeds a dirty porcelain, asserts AbortError, StageFailed surfaced, the `git status --porcelain` probe WAS issued (:346, gate NOT bypassed for DryRun alone), and assertNoMutation. This directly pins the edge case that the bypass requires AutoStash too.
  - The shared seedDryRunFirstRelease helper was correctly parameterised with skipCleanTree (release_dryrun_test.go:38) so the porcelain slot is conditionally omitted, keeping ONE definition of the read-gate order across callers (every other caller passes false).
- Notes:
  - Not under-tested: all five acceptance bullets and both edge cases have a dedicated, behaviour-level assertion. Tests would fail if the feature broke — e.g. if the autostash block were not gated on !DryRun the unseeded `git stash push` would error the run; if the gate bypass were dropped the unseeded porcelain probe (or a dirty read) would abort the preview.
  - Not over-tested: the new tests are focused and non-redundant; the preflight test asserts exact call count (2) rather than over-mocking.
  - Tests assert exact argv and exact presenter event kinds, per the project test idiom.

CODE QUALITY:
- Project conventions: Followed. Every mutation still flows through git.Mutator (autostash.go:50/74); reads stay on the runner; output stays on the presenter. The non-negotiable seam "DryRun never reaches the Mutator" is now genuinely upheld for the --autostash combo (the only path that previously violated it). External test packages, t.Parallel(), t.TempDir(), FakeRunner + RecordingPresenter all used.
- SOLID principles: Good. skipCleanTree is a single-responsibility boolean threaded through the existing gate driver exactly like anyBranch — no new abstraction, open/closed honoured (RunLocalGates extended via a parameter, callers updated).
- Complexity: Low. Two `if !flag {` guards and one `&&` at the call site; no added branching depth.
- Modern idioms: Yes. Idiomatic Go; no concerns.
- Readability: Good. The WHY-comments are thorough and true-to-as-built: no comment claims the stash runs in dry-run; the DryRun field doc, Release doc, Stage-2 block, autostash.go header, runPreflight and RunLocalGates docs all consistently state the stash is skipped and the clean-tree gate bypassed under DryRun && AutoStash. Comment/code agreement verified line-by-line.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] internal/engine/release_test.go:1304 — assertNoMutation checks only `git tag -a` / `git push` / `gh release create` prefixes; it does NOT flag `git stash push`/`git stash pop` even though both are Mutator mutations. This task's dry-run-autostash test compensates with explicit invokedWith checks (release_autostash_test.go:318-323), so coverage is not weakened here. Decide whether to broaden assertNoMutation to also reject stash mutations so future dry-run tests inherit the guard automatically rather than each restating it — a judgment call on the helper's intended contract, hence idea not quickfix.
