# Plan: Notes Failure Output Ugly and Uninformative

## Phases

### Phase 1: Carry claude's captured output through the AI transport carrier
status: draft

**Goal**: Upgrade `ai.ErrGenerationFailed` into a typed carrier error (e.g. `*ai.GenerationError`) that wraps the bare sentinel and carries claude's captured `Stdout`/`Stderr` (and optionally `ExitCode`) from the runner `Result`. `transport.attempt` stops discarding `res` on the bad-content path, and `Generate` packs the captured output into the carrier when a non-zero exit / empty body survives the single retry — all without changing `Generate`'s signature.

**Why this order**: This is the load-bearing fix (Fix 1). The captured output is discarded at the transport seam, which is shared by all three verbs; until the carrier exists, nothing downstream can populate `StageFailure.Output`. Fix 2 (concise message) and the `Output`-population wiring both depend on this carrier, so it must land first. It also has zero forward dependencies — it builds only on the runner's documented guarantee that a non-zero exit returns a fully-populated `Result`.

**Acceptance**:
- [ ] A non-zero-exit `Generate` (with `FakeRunner` seeded to return stdout on a non-zero exit, since prior fakes had no stdout to lose) returns an error carrying the runner's captured stdout and stderr as distinct fields, populated only after the single bad-content retry is exhausted.
- [ ] `errors.Is(err, ai.ErrGenerationFailed)` still matches the carrier — sentinel-based routing (release `on_notes_failure`, commit editor fallback) is unaffected.
- [ ] `context.Canceled` still propagates UNCHANGED through `classifyFatal` — a cancel is never wrapped into or swallowed by the carrier.
- [ ] `ai.ErrTimeout` and `ai.ErrCommandMissing` still short-circuit via `classifyFatal` with no carrier; the transport still never imports `config`; the single-retry ownership and the byte-identical success path are unchanged.
- [ ] All project gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).

### Phase 2: Render verbatim claude output below a concise notes-failure line
status: draft

**Goal**: At the two notes `StageFailed` surfacing helpers — `surfaceAndUnwind` (forward release) and `surface` (single-version/interactive regenerate) — extract the Phase 1 carrier's captured output via an `errors.As`-based helper and feed it into `StageFailure.Output` using the settled composition rule. Extend `failureMessage` to derive the concise cause phrase (Fix 2) so the top-line `Message` is one short phrase, separated from the still-intact `%w` matchable chain. Keep the `padStage` gap unchanged (Fix 3 — no presenter or layout change).

**Why this order**: Consumes the Phase 1 carrier through `errors.As(cause, &genErr)` (which traverses the `%w` chain, matching even when wrapped in `abortError`). No forward reference — it builds only on the now-populated error and the existing presenter, which already renders `Output` verbatim below the ✗ line via `writeNotesBody`. Centralising the concise phrase on `failureMessage` covers the forward chain, regenerate's shorter chain, and `resetAndAbort` in one place.

**Acceptance**:
- [ ] `StageFailure.Output` is populated at BOTH surfacing sites with the composed output: trim each stream for emptiness; include non-empty streams stdout-first then stderr, single-newline-joined; trim trailing whitespace of the composed result; internal content verbatim; empty `Output` (✗ line stands alone) when both streams are whitespace-only.
- [ ] The top-line `Message` is the concise cause phrase for all four known causes (`ai.ErrGenerationFailed`, `ai.ErrTimeout`, `ai.ErrCommandMissing`, `notes.ErrDiffTooLarge`): it contains no nested `%w` chain, does not begin with the stage label as a prefix (no `notes notes …`), and does not contain "failed" (an incidental "notes" inside the phrase is allowed).
- [ ] The concise-message derivation produces a clean phrase for both the forward-release wrap chain AND regenerate's shorter chain; `resetAndAbort` inherits the concise `Message` for free with no `Output` change (its cause is a git record/push failure, not the AI carrier).
- [ ] Only `Output`-carrying cause is generation-failed; timeout / command-missing / diff-too-large render the concise phrase with empty `Output`. The forward and regenerate paths behave identically.
- [ ] The `padStage` gap is unchanged for all aligned lines; only the message-text assertions in `pretty_test.go` change; `gate_forbidden_test.go` and `askline_test.go` stay untouched.
- [ ] All project gates pass: `go build ./...`, `gofmt -l .` (empty), `go vet ./...`, `go test -race ./...`, `golangci-lint run` (0 issues).
