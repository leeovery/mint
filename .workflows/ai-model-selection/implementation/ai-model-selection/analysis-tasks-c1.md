---
topic: ai-model-selection
cycle: 1
total_proposed: 2
---
# Analysis Tasks: AI Model Selection (Cycle 1)

## Task 1: Consolidate the three duplicated transport-construction wiring sites into one shared helper
status: pending
severity: medium
sources: duplication, architecture

**Problem**: The three production transport-wiring sites — `internal/engine/release.go:936-944` (aiTransport), `internal/engine/regenerate_fresh.go:135-143` (resolveFreshTransport), and `internal/commit/run.go:780-788` (commitTransport) — were each implemented in isolation and converged on a byte-identical production construction expression: `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)})`. The only token that varies is the Verb constant (VerbRelease at two sites, VerbCommit at the third). The spec's per-key sourcing contract — source BOTH command and timeout from the accessor, assign Timeout directly so "no deadline" is never reachable by zero-by-omission — now lives in three independent places and can drift: a future change adding a third resolved field or altering the no-deadline mapping must be made identically in all three or one site silently diverges. The spec explicitly flags resolveFreshTransport as the "easy miss" site precisely because it is a structural duplicate, and the correctness of regenerate-rides-`[release]` currently rests on each site independently passing the right verb constant rather than on construction.

**Solution**: Extract the shared production-construction expression into a single helper that maps a `(runner, config, verb)` triple to the constructed transport, so the accessor-sourcing and the `ai.Config` assembly live in exactly one place. Each wiring site then differs only in the verb constant it passes, making the regenerate-rides-`[release]` decision a one-token choice at an obvious call. Keep each site's local nil-injected-transport test-seam guard in place — that part legitimately differs by the deps wrapper type and is NOT the duplication being centralized; only the construction expression moves.

**Outcome**: The "thread command + timeout from the accessor, never zero-by-omission" contract is expressed once. The two engine-package sites (aiTransport, resolveFreshTransport) and the commit-package site (commitTransport) all route their construction through the shared helper. A future change to the resolved-field set or the no-deadline mapping is a single edit. All existing white-box transport tests and the full suite continue to pass with no behavioural change.

**Do**:
1. Define a small shared constructor that takes the `runner.CommandRunner`, the resolved `config.Config`, and the `config.Verb`, and returns the constructed `*ai.Transport` via `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)})`. Place it where it can be shared by both the engine and commit packages WITHOUT forcing `config` to import `ai` (the spec mandates the `config`↔`ai` decoupling survives — `config` never imports `ai`). Options that preserve the import direction: a thin engine/commit-side helper that both packages import, or have the helper live alongside `ai`/the transport accepting the resolved values; pick whichever keeps the import direction clean. It depends only on `ai`, `config`, and `runner`, all already imported at the sites.
2. Rewrite `aiTransport` (`internal/engine/release.go:936-944`) to keep its local injected-transport short-circuit and call the shared helper with `config.VerbRelease` for the production path.
3. Rewrite `resolveFreshTransport` (`internal/engine/regenerate_fresh.go:135-143`) to keep its local injected-transport short-circuit and call the shared helper with `config.VerbRelease` (regenerate rides `[release]`).
4. Rewrite `commitTransport` (`internal/commit/run.go:780-788`) to keep its local injected-transport short-circuit and call the shared helper with `config.VerbCommit`.
5. Run all gates: `go build ./...`, `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Acceptance Criteria**:
- The production construction expression `ai.NewTransport(r, ai.Config{AICommand: cfg.AICommandFor(verb), Timeout: cfg.TimeoutFor(verb)})` appears in exactly one place; the three wiring sites delegate to it.
- Each wiring site retains its own local nil-injected-transport test-seam guard (unchanged behaviour for the injected path).
- `internal/config` still does not import `internal/ai` — the decoupling is preserved.
- Each site passes its correct verb constant: VerbRelease for aiTransport and resolveFreshTransport, VerbCommit for commitTransport; regenerate continues to resolve through `[release]`.
- All existing white-box transport tests pass unchanged; no argv or rendered-line drift.
- All gates pass: build, `gofmt -l .` prints nothing, `go vet`, `go test -race`, `golangci-lint run` reports 0 issues.

**Tests**:
- Existing per-site white-box transport tests (release, regenerate, commit) continue to pin each site's resolved verb and argv — assert they pass unchanged after the refactor, proving the regenerate-rides-`[release]` routing and the per-verb command/timeout threading survive consolidation.
- Verify (via the existing regenerate transport test) that resolveFreshTransport still produces release-resolved command/timeout values, not bare shared/default values.
- If the shared helper is independently unit-testable in its package, add a focused test that constructs through it for both VerbRelease and VerbCommit and asserts the resulting `ai.Config` carries the accessor-resolved AICommand and Timeout (never zero-by-omission).

## Task 2: Rewrite forward-looking phase/task comment narration in config/verb to match as-built code
status: pending
severity: medium
sources: standards

**Problem**: CLAUDE.md's Comments contract is explicit: "Keep them TRUE TO AS-BUILT" and "Never leave scope/phase claims that contradict the shipped code." Several WHY-comments in the shipped config/verb files describe work that is actually completed in THIS change as still-future or deferred to a later task/phase, directly contradicting the as-built code. Every behavioural decision the comments describe is correctly implemented — the only defect is the forward-looking tense. Affected sites:
- `internal/config/verb.go:5` — "the layered accessors (AICommandFor / TimeoutFor, arriving in later tasks) accept" — both accessors are shipped in this same change (config.go), so they are not "arriving in later tasks."
- `internal/config/config.go:82` (DefaultAICommand) — "the transport's duplicate self-default and initgen's scaffold literal are removed/sourced from this constant in later phases" — both are already done in this change (transport self-default deleted; initgen pinned by a build-failing drift test).
- `internal/config/config.go:91` (DefaultTimeout) — "the transport's defaultTimeout literal (which Phase 2 deletes in favour of this)" — that literal is already deleted in this change.
- `internal/config/config.go:584,589` (TimeoutFor doc) — "Phase 2's ai.Config.Timeout is also *time.Duration ..." and "that conditional lives in Phase 2, NOT here" — the boundary type and the transport's conditional WithTimeout skip are both shipped in this change.
- `internal/config/config.go:137,206,210,259,649` — multiple comments defer the negative-drop / floor application to "Task 1-7's TimeoutFor accessor" / "1-7's job" as if 1-7 were a separate future task; TimeoutFor is implemented in the same file in this change.
- `internal/config/config.go:199,248,695` and `internal/config/config.go:104` — "the resolver (1-4) needs ..." / "engine-level keys and other verb tables arrive in later phases" — these phrase already-shipped resolution and the `[commit]` table as forthcoming.

**Solution**: Rewrite each affected comment in the present-tense / as-built voice: state the contract the code now upholds rather than which task or phase will deliver it. Preserve the WHY/contract content in full — only drop the task/phase tense. Leave untouched the genuinely-historical "Phase 1" notes and the out-of-scope "Phase 6 provider validation" carve-outs, which remain accurate.

**Outcome**: Every WHY-comment in `internal/config/verb.go` and `internal/config/config.go` describes the shipped behaviour as a present fact. No comment claims a behaviour that is in fact already implemented is "arriving in later tasks," "removed in later phases," or "1-7's job." The CLAUDE.md Comments contract holds. No code behaviour changes; the build remains green.

**Do**:
1. `internal/config/verb.go:5` — rephrase to the present tense, e.g. "the layered accessors AICommandFor / TimeoutFor accept ..." (drop "arriving in later tasks").
2. `internal/config/config.go:82` (DefaultAICommand) — rephrase to state this constant is the single source the transport and initgen derive from now, e.g. "the transport carries no default and initgen sources its scaffold literal from this constant" (drop "in later phases").
3. `internal/config/config.go:91` (DefaultTimeout) — rephrase to state the transport's defaultTimeout literal is gone and this constant is the source (drop "which Phase 2 deletes").
4. `internal/config/config.go:584,589` (TimeoutFor doc) — rephrase to describe the shipped `*time.Duration` boundary and the transport's conditional WithTimeout skip as as-built (drop "Phase 2's ..." / "lives in Phase 2, NOT here"); retain the WHY about why the conditional lives in the transport, not the accessor.
5. `internal/config/config.go:137,206,210,259,649` — rephrase the negative-drop / floor narration to present tense, e.g. "TimeoutFor drops a negative to the floor" (drop "1-7's job" / "Task 1-7's TimeoutFor accessor").
6. `internal/config/config.go:199,248,695` and `internal/config/config.go:104` — rephrase to describe the shipped resolver and the `[commit]` table as present, e.g. "the resolver resolves ..." / "the `[commit]` table ..." (drop "the resolver (1-4) needs" / "arrive in later phases").
7. Confirm the historical "Phase 1" notes and the "Phase 6 provider validation" carve-outs are left unchanged.
8. Run all gates: `go build ./...`, `gofmt -l .` (must print nothing), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

**Acceptance Criteria**:
- None of the enumerated comments in `internal/config/verb.go` or `internal/config/config.go` describe an already-shipped behaviour as future, deferred, or owned by a later task/phase.
- The WHY/contract content of each comment is preserved (the reasoning the code can't show survives); only the task/phase tense is removed.
- The genuinely-historical "Phase 1" notes and the out-of-scope "Phase 6 provider validation" carve-outs remain unchanged and accurate.
- No code behaviour changes — comment-only edits.
- All gates pass: build, `gofmt -l .` prints nothing, `go vet`, `go test -race`, `golangci-lint run` reports 0 issues.

**Tests**:
- No new tests required — this is a comment-only change. The full existing suite (`go test -race ./...`) must continue to pass unchanged, proving no behaviour was touched.
- Manual/review verification that each enumerated line now reads in the as-built present tense and that the historical/out-of-scope carve-outs were not altered.
