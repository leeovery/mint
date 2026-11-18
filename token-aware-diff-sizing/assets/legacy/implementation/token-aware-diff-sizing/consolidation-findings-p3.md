# Consolidation Findings: Token Aware Diff Sizing (Phase 3)

## Findings

### F1: `regenerate <version> --notes-file P` resolves the publisher, and can warn, before it reads P
- **Class**: behaviour
- **Failure**: Take an operator with a non-github origin or no remote who runs `mint release regenerate 1.4.0 --notes-file P` with P missing, empty or `""`. Before the refusal they see `publish skipped: provider could not be resolved (...); downgrading to tag + push only`, a downgrade notice for a run that never starts. A non-downgrade provider-resolution error goes further: it surfaces and aborts in place of the refusal, so the operator fixes the provider first and only on the next run learns that P is unusable. §13.2 and §13.4 say mint reads the file at startup, before any check, and refuses it before any step runs. 3-5's criterion says every refusal lands before any step. Release keeps that promise, since it reads P before `ResolveRoot` (`release.go:320`); regenerate does not.
- **Evidence**: `cmd/mint/main.go:146` (ResolveRoot), `:151` (config.Load), `:170` (ResolveReleaseBranch), `:205` (ResolveRegenerateTarget), `:224` (`resolveRegeneratePublisher` → `internal/engine/release.go:1141-1146` `ResolvePublisher` → `warnPublishDowngraded`), and only then `:241` `engine.RegenerateRun`, which reads the file at `internal/engine/regenerate_interactive.go:164`. Release's order, for contrast: `internal/engine/release.go:319-323`. `internal/engine/notesfile_refusal_test.go` drives `RegenerateRun` directly, so the cmd-layer order goes untested.
- **Proposed shape**: Read P at the head of `runRegenerateSingle`, ahead of version-tag and publisher resolution, through an engine entry point that wraps `readNotesFile` and surfaces the refusal on `deps.Presenter` at stage `notes`. Carry the read body into `RegenerateRunRequest` (a `Supplied *string` beside `NotesFile`, which `notesExclusion` still needs), so `RegenerateRun` stops reading. The minimum fix is to move `resolveRegeneratePublisher` after the read. Add a cmd-level test: with an unresolvable origin and an empty P, the output holds the refusal and no `publish skipped` warning.
- **Bank**: 3-5 reviewer: "the --notes-file refusal is not the first thing reported: the cmd layer's publisher resolution runs, and can warn, before RegenerateRun reads the file".

### F2: The tag annotation drops a supplied body's `#` lines and trailing whitespace
- **Class**: behaviour
- **Failure**: `git tag -a <tag> -F -` runs without `--cleanup`, and git's default for `git tag` is `strip`. `man git-tag` (git 2.56.0): "The strip mode is default … strip removes both whitespace and commentary". Commentary means every line that starts with `core.commentChar` (`#`). Hand-written notes are usually Markdown, and `--notes-file` is the route this phase opens for them (`NEXT_RELEASE.md`, `notes.md`). Their `## Added` headings, `#123` lines, two-space line breaks and repeated blank lines therefore vanish from the tag annotation without any message. The changelog section and the provider release carry them verbatim. The operator approved the full body at the review gate, so the first sign is a tag body that differs from the GitHub release. A later `regenerate --source tag` then copies the stripped body onto other surfaces. README.md:107 promises the contents ship "byte-identical as the notes"; §13.3 says the body reaches "the tag, changelog and provider release byte-identical"; CLAUDE.md's "Byte-identical bodies" invariant says the same. The phase's own fixture is exactly this shape: `internal/engine/release_notesfile_test.go:22-25` uses `"## ✨ Added\n"` and a trailing `"  \n"`, and its doc says "trailing spaces and blank lines survive". It passes only because FakeRunner records the stdin and never runs git's cleanup. The defect predates this phase but rarely fired: the default prompt emits emoji section headers, not `#`. A supplied body makes it the common case.
- **Evidence**: `internal/release/release.go:99` (`"git", "tag", "-a", tag, "-F", "-"`), `:124-127` (`composeTagMessage` = subject + blank line + body); `README.md:107`; `internal/engine/release_notesfile_test.go:22-25`. Pinned tag argv that a fix updates: `internal/release/release_test.go:40`, `internal/engine/release_test.go:387`, `:1540`, `internal/engine/release_commitgraph_test.go:33`, `internal/engine/release_downgrade_test.go:62`, `internal/engine/release_setversion_test.go:177,181,224`, `internal/engine/release_priortag_test.go:390`.
- **Proposed shape**: Add `--cleanup=verbatim` to the annotated-tag argv in `createAnnotatedTag`, so the annotation is `composeTagMessage`'s bytes exactly, on every notes route. Update the pinned argv in the tests listed above. Check that the tag-body reader behind `regenerate --source tag` (`RequireTagBody`) still splits subject from body when trailing whitespace is no longer normalised.

## Comment Corrections

- internal/engine/release.go:1487-1491 — regenerate now passes `NotesReviewGateNoRegen()` as well, and the caller list is a cardinality claim
  OLD: // The gate is supplied by the caller: release passes gateForKind(kind)
// (ReleaseReviewGate's y/n/e/r for normal-AI, FirstReleaseReviewGate's y/n/e
// otherwise), regenerate passes presenter.NotesReviewGate() (its own wording). The
// loop is gate-agnostic — it acts on the choice keys the supplied gate declares, and
// the SAME gate is re-rendered on every iteration.
  NEW: // The gate is supplied by the caller. The loop is gate-agnostic — it acts on the
// choice keys the supplied gate declares, and the SAME gate is re-rendered on every
// iteration.
- internal/engine/regenerate_write.go:56-59 — describes two gatings, but the supplied source now runs a third (the review gate without `r`)
  OLD: // RegenerateSource names where a regenerate run sourced its notes body — the axis
// that drives the gating: fresh notes are reviewable (the full y/n/e/r gate), the two
// reuse-style sources are deterministic (a simple confirm). It is the engine-level
// concrete enum so the orchestration never threads the cmd-layer flag types.
  NEW: // RegenerateSource names where a regenerate run sourced its notes body — the axis
// that drives the gating (gateRegenerate). It is the engine-level concrete enum so the
// orchestration never threads the cmd-layer flag types.
- internal/engine/regenerate_write.go:113 — the fresh/reuse mapping leaves out the supplied source's gate
  OLD: 	// Source selects the gating: fresh → notes-review gate, reuse → simple confirm.
  NEW: 	// Source selects the gate gateRegenerate runs.
- internal/engine/regenerate_write.go:152-153 — the two-way mapping leaves out the supplied source and repeats gateRegenerate's doc
  OLD: 	// Gate/confirm by source BEFORE any mutation. Fresh runs the notes-review gate
	// (the reviewed body may be edited/regenerated); reuse is a simple confirm.
  NEW: 	// Gate/confirm by source BEFORE any mutation.
- internal/engine/regenerate_interactive.go:173-175 — false: no source forces a target (ResolveRegenerateAxes asks the target for every source); the comment sits on the call this phase changed
  OLD: 	// Resolve both axes — source then target — via the shared resolver: a supplied flag
	// skips its question, an unset axis is asked, and a reuse source FORCES release (no
	// target question). Both questions are asked back-to-back before any slow work.
  NEW: 	// Resolve both axes — source then target — via the shared resolver: a supplied flag
	// skips its question and an unset axis is asked. Both questions are asked
	// back-to-back before any slow work.
- internal/engine/regenerate_interactive.go:241-244 — the fresh/reuse mapping leaves out the supplied source; the comment also carries a task id and repeats RegenerateWrite's and the Regenerator field's docs
  OLD: 	// Delegate to the 5-9 write path: it shows the notes, runs the source-appropriate
	// confirm/review gate (fresh → notes-review, reuse → simple confirm), and writes.
	// The per-run regenerator (bound to this version's fresh AI range) backs the fresh
	// gate's `r` choice.
  NEW:
- internal/notes/resolve.go:53-59 — the list of branches that never reach the resolver leaves out the supplied body, which now sits ahead of them all; "(a later task)" is workflow vocabulary
  OLD: // CONTRACT — NORMAL PATH ONLY: ResolveFailure governs ONLY the normal AI path. It is
// invoked solely when the generator surfaces a typed failure. The notes-path precedence
// (a later task) sits IN FRONT and ensures first-release, degenerate/empty-diff, and
// --no-ai NEVER reach here — those paths never call the AI, so they can never produce a
// failure for this resolver to route. This is a pure function of (failure, lastTag,
// rel): it is meaningful ONLY given a real failure, and does not itself implement that
// precedence.
  NEW: // CONTRACT — NORMAL PATH ONLY: ResolveFailure is invoked solely when the generator
// surfaces a typed failure. Every non-AI branch of the notes precedence returns before
// the AI is called, so none reaches here; this function does not itself implement that
// precedence.
- internal/notes/resolve.go:67 — workflow vocabulary ("Phase-6-rejected"); it restates the code, and the doc this phase rewrote already says Load rejects unknown values
  OLD: 	// "" / "abort" and any unknown (Phase-6-rejected) value abort, tagging nothing.
  NEW:
- internal/notes/noai.go:25-28 — workflow vocabulary ("a later task", "separate tasks"); the precedence exists, and the "after the first-release and degenerate checks" ordering now leaves out the supplied body
  OLD: //
// The notes-path precedence (a later task) selects this provider when --no-ai is set
// (after the first-release and degenerate checks); the flag parsing and wiring are
// separate tasks. This provides only the body behaviour the flag selects.
  NEW:
