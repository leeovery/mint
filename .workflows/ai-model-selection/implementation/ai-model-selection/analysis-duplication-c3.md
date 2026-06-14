AGENT: duplication
FINDINGS:
- FINDING: Three near-identical per-site white-box transport-wiring test files
  SEVERITY: medium
  FILES: internal/engine/release_aitransport_internal_test.go:24-98, internal/engine/regenerate_fresh_aitransport_internal_test.go:24-100, internal/commit/run_aitransport_internal_test.go:25-101
  DESCRIPTION: These three files were each authored to prove their own wiring site
    (aiTransport / resolveFreshTransport / commitTransport) sources command + deadline
    from the per-verb accessors. The three bodies are structurally the same three
    scenarios — "command from accessor" (~25 lines), "explicit zero → no deadline"
    (~20 lines), "positive → deadline" (~18 lines) — diverging ONLY in the function
    under test, the verb, and a label string. Each repeats the same setup
    (build config.Config, spy := &runner.DeadlineRecordingRunner{}, call site,
    transport.Generate, assert Name/HadDeadline) and the same inline argv-equality
    comparison `len(spy.Args()) != len(wantArgs) || spy.Args()[0] != wantArgs[0] ||
    spy.Args()[1] != wantArgs[1]`. That argv check is itself copy-pasted verbatim three
    times (release_aitransport_internal_test.go:45, regenerate_fresh_aitransport_internal_test.go:47,
    run_aitransport_internal_test.go:46) and is brittle (hardcoded indices 0/1). The
    consolidated aitransport_test.go already proves the SHARED construction expression
    end-to-end and table-drives both verbs with a clean len-then-element-loop argv
    compare (aitransport_test.go:104-111), so the three internal files now overlap
    heavily with it — each adds only "this site delegates to the shared helper", a
    one-scenario fact, wrapped in three full scenarios of duplicated assertion scaffolding.
  RECOMMENDATION: Keep one minimal per-site delegation proof in each internal file
    (the command-from-accessor case is sufficient to prove the site wires the right verb
    through aitransport.New) and drop the duplicated zero/positive-deadline scenarios from
    the three internal files — that deadline-threading behaviour is already owned once by
    aitransport_test.go (the helper) and runner's DeadlineRecordingRunner test. If all
    three scenarios are retained per site, extract the repeated argv-equality assertion
    into one shared test helper (e.g. an exported assert in the runner/presentertest test
    surface, or reuse DeadlineRecordingRunner) so the brittle hardcoded-index compare
    lives in exactly one place rather than three.

- FINDING: Duplicated argv-matching test helpers across commit and engine packages
  SEVERITY: medium
  FILES: internal/commit/run_aitransport_test.go:49-89, internal/engine/release_test.go:400-416 (stdinOf/invokedWith referenced by the in-scope release_configconsolidation_test.go)
  DESCRIPTION: run_aitransport_test.go independently re-authored two FakeRunner
    argv-matching helpers — aiInvocationStdin (return the stdin of the first invocation
    whose name+args match) and invokedBinary (report whether any invocation matched
    name+args) — that are functionally identical to the engine package's stdinOf /
    invokedWith, which the in-scope release_configconsolidation_test.go consumes for the
    exact same purpose (proving the configured ai_command's binary+args were invoked with
    the prompt on stdin, and the default was not). Both pairs implement the same
    "iterate r.Invocations(), match name then element-wise args, return stdin / bool"
    loop. Because they live in sibling test packages that cannot import each other's
    _test.go helpers, the logic was copied rather than shared — exactly the copy-paste
    drift across task boundaries this consolidation guards against; a fix or refinement to
    the matching logic in one package will not reach the other.
  RECOMMENDATION: Promote the shared argv-match-and-stdin lookup onto the production
    test surface both packages already import — the runner package (alongside FakeRunner /
    DeadlineRecordingRunner, the precedent set for cross-package test helpers in this very
    work unit) — e.g. a FakeRunner method `StdinOf(name string, args ...string) string`
    and `Invoked(name string, args ...string) bool`. Then delete commit's aiInvocationStdin
    / invokedBinary and engine's stdinOf / invokedWith in favour of the single shared
    implementation. (Note: engine's stdinOf/invokedWith live in release_test.go, which is
    out of plan scope; the duplication introduced by THIS work is the commit-side copy —
    consolidating onto runner removes the in-scope copy and lets the pre-existing engine
    copy migrate too.)
SUMMARY: Production code is well-consolidated — the three wiring sites correctly delegate
  the construction expression to the single aitransport.New helper, and resolution
  (AICommandFor/TimeoutFor), the deadline spy, and DurationPtr each live in exactly one
  place. The remaining duplication is in the test layer: three near-identical per-site
  white-box test files that largely re-prove what the consolidated aitransport_test.go
  already owns, and a pair of argv-matching FakeRunner helpers copy-pasted between the
  commit and engine test packages.
