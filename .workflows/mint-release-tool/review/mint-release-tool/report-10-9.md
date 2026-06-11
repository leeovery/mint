TASK: Stop Blocking-Stage Spinner Leaks Across Warn (Promoted Bug, Extended) — mint-release-tool-10-9 (type: bug, severity: high)

ACCEPTANCE CRITERIA:
- The --all notes-failure skip on the last/only version emits ⚠ skipped and the end summary with no live spinner over them.
- The real-run cache-reuse / miss / unreadable notices appear without a spinner animating over them.
- A normal blocking stage (no Warn/skip) still shows and stops its spinner as before.

STATUS: Complete

SPEC CONTEXT:
Spinner/stage rendering is owned by the CLI Presentation spec (pretty mode only — a blocking StageStarted starts a single spinner, replaced in place by the ✓/✗ completion line). The engine drives the stage events. Plain mode has no spinner (StageStarted emits a terse static start line; SuspendSpinner/ResumeSpinner are no-ops), so the leak is a pretty-mode-only concern. The "one spinner at a time" invariant and the requirement that no frame animates over a status line are presentation-spec rules.

IMPLEMENTATION:
- Status: Implemented (single general fix, as planned)
- Location:
  - Fix: internal/presenter/pretty.go:533 — Warn() calls p.stopSpinner() as its first statement, before rendering the ⚠ line. This is the single general fix covering every Warn site (current and future) rather than patching each engine call site, exactly as the plan's preferred option specifies. A thorough doc comment (pretty.go:520-532) explains why: it names both the batch-skip path and the real-run cache reuse/miss/unreadable path, notes stopSpinner is idempotent (standalone Warn unaffected), and notes it clears any pending suspend so a later ResumeSpinner cannot resurrect a stopped spinner.
  - stopSpinner (pretty.go:349-356): idempotent — clears spinnerSuspended, no-ops when activeSpinner is nil, else Stop()s and nils the handle.
- Both target sites confirmed driven through this seam:
  - (a) Batch-skip: internal/engine/regenerate_batch.go:269 emits emitBlockingStageStarted(p, "notes") (StageStarted{Blocking:true} → spinner), then on a ProduceBody failure reportSkip (regenerate_batch.go:272, 257) → p.Warn (regenerate_batch.go:328) fires with NO StageSucceeded. The general Warn fix stops the live spinner before the ⚠ line and the end summary.
  - (b) Real-run cache notices: internal/engine/release.go:816, 824, 834 (reportNotesReused / reportNotesRegenerating / reportNotesCacheUnreadable) all call p.Warn, fired from the reuse callback (release.go:793-802) inside the live blocking notes stage (started at release.go:433). The same fix stops the spinner before each notice; the stage then continues and its eventual StageSucceeded/StageFailed prints the ✓/✗ line in the cleared place.
- RunFinished (pretty.go:918): unchanged and correct — it does NOT stop a spinner, and does not need to. In the batch-skip path the spinner is already stopped by the preceding Warn, so the end summary RunFinished renders is leak-free. Verified there is no path where RunFinished is reached with a live spinner that this task should have closed.
- Notes: No drift. The plan offered two options (suspend/stop on Warn, OR close the stage before the notice); the implementation took the preferred presenter-side option, which is the more robust general fix. Plain presenter Warn (plain.go:240) is untouched and correctly unaffected (no spinner there).

TESTS:
- Status: Adequate
- Coverage:
  - Batch-skip case: TestPrettyPresenterWarnStopsActiveSpinnerBatchSkipPath (pretty_spinner_test.go:250) — StageStarted{Blocking} → Warn (no StageSucceeded) → RunFinished{VerbRegenerate}. Asserts exactly one spinner created, it is stopped, and tr.active==0 across the skip Warn and end summary (no resurrection by RunFinished). Directly mirrors the engine's reportSkip→Warn sequence.
  - Cache-reuse case: TestPrettyPresenterWarnStopsActiveSpinnerCacheReusePath (pretty_spinner_test.go:276) — StageStarted{Blocking} → Warn (mid-stage) → captures stoppedAtWarn at the moment the Warn returned → StageSucceeded. Asserts the spinner was stopped BY the Warn itself (not merely later by StageSucceeded — the load-bearing distinction), tr.active==0, and ordering: the ⚠ notice precedes the ✓ completion line in the buffer.
  - No-op safety: TestPrettyPresenterWarnWithNoActiveSpinnerCreatesNone (pretty_spinner_test.go:311) — a standalone Warn (post_release/push case) creates/touches no spinner, proving the common case is unaffected.
  - Normal blocking stage unchanged: TestPrettyPresenterSpinnerReplacedByCheckOnSuccess (:81) and …CrossOnFailure (:106), plus the one-spinner-at-a-time and defensive-stop tests, all still assert the start→stop lifecycle.
- Notes: Tests are behaviour-focused via the spySpinner/spyTracker seam (no real timed goroutine, no frame assertions). The cache-reuse test's stoppedAtWarn capture is precisely the right assertion — without the fix, the spinner would only stop at StageSucceeded, which this test would catch. Tests would fail if the fix were removed. Not over-tested: each test exercises a distinct path (skip / mid-stage continue / no-op / normal lifecycle) with no redundant happy-path duplication.

CODE QUALITY:
- Project conventions: Followed. Uses the established newSpinner injection seam (golang-concurrency: the timed goroutine never reaches lifecycle tests); spySpinner/spyTracker testify-free table of behavioural assertions consistent with the package's existing spinner tests; idempotent stopSpinner consistent with the existing defensive-stop idiom.
- SOLID principles: Good. Single general fix in the presenter (the layer that owns the spinner) rather than scattering stop calls across engine call sites — keeps the responsibility where the invariant lives (SRP) and means future Warn sites are covered without change (OCP).
- Complexity: Low. One added statement (p.stopSpinner()) at the head of Warn; no new branching.
- Modern idioms: Yes. No concerns.
- Readability: Good. The Warn doc comment (pretty.go:520-532) is thorough and explains the why (both leak paths, idempotency, suspend-flag clearing) without over-explaining.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
