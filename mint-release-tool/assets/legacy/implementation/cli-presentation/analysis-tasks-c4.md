---
topic: cli-presentation
cycle: 4
total_proposed: 1
---
# Analysis Tasks: CLI Presentation (Cycle 4)

## Task 1: Restore the one-place mode-branch/stream-split invariant in presenter wiring
status: approved
severity: low
sources: architecture

**Problem**: In `internal/presenter/wiring.go`, `New(mode, out, err)` documents itself as "the single wiring point where mode selection and the stdout/stderr stream split meet" and asserts "the split is established in exactly one place". That claim is now stale. Cycle-3 task 7-2 introduced `NewForStartup`, the fuller, production-reachable seam that re-implements the same mode-branch and out/err stream-split inline (`if signals.Mode == ModePretty { NewPrettyPresenter(stdout, WithErr(stderr))... } else { NewPlainPresenter(stdout, stderr)... }`) before layering term-width, `-y`, and stdin-interactive on top. `New` is now referenced only by tests (`wiring_test.go` `newSplit`). So the mode-branch + stream-split physically lives in two places, and `New`'s "exactly one place" / "single wiring point" doc comment no longer holds. This is the direct doc-accuracy consequence of task 7-2.

**Solution**: Make the documented one-place invariant true again by converging the two seams. PREFERRED: have `NewForStartup` route its mode-branch and out/err stream-split through the same shared core `New` flows through, so the split genuinely lives in one place and `New` stays the shared seam both paths pass through; `NewForStartup` then layers only its extra axes (term-width, `-y`, stdin-interactive) on top. Because the concrete `.WithTermWidth/.WithYes/.WithInteractiveStdin` setters are not on the `Presenter` interface that `New` returns, convergence is achieved by extracting the mode-branch + stream-split into a small concrete-returning helper (e.g. an unexported `buildForMode(mode Mode, out, err io.Writer)` that returns the concrete pretty/plain presenter), and having BOTH `New` (which returns it as the `Presenter` interface) and `NewForStartup` (which keeps the concrete return so it can chain its extra setters) call that one helper. After this, exactly one site performs the mode-branch and out/err wiring. FALLBACK (only if clean delegation can't be done without behaviour change): leave the structure as-is and instead down-scope `New`'s doc comment so it describes itself as the raw/lower-level (test) wiring seam, and move the "single production construction site" / single-place language exclusively onto `NewForStartup`'s doc comment.

**Outcome**: The mode-branch and stdout/stderr stream-split exist in exactly one place in the package, and every doc comment that asserts a single/one-place construction invariant is accurate with respect to where that branch actually lives. No runtime behaviour changes: production wiring (`NewForStartup`) and the test seam (`New`) produce byte-for-byte identical presenters to today, including the same writer wiring, mode selection, and the additional axes (width, `-y`, stdin-interactive) that `NewForStartup` threads.

**Do**:
1. Read `internal/presenter/wiring.go` (`New` ~line 19, `NewForStartup` ~line 62) and `internal/presenter/wiring_test.go` (the `newSplit` helper that calls `New`).
2. Confirm the duplicated logic: both functions perform `if mode/signals.Mode == ModePretty -> NewPrettyPresenter(out, WithErr(err))` else `NewPlainPresenter(out, err)`.
3. PREFERRED path: extract that mode-branch + stream-split into one unexported helper that returns the concrete presenter (return type must let `NewForStartup` chain `.WithTermWidth/.WithYes/.WithInteractiveStdin`). Then:
   - `New` calls the helper and returns its result as the `Presenter` interface — unchanged externally observable behaviour.
   - `NewForStartup` calls the same helper for the out/err mode-branch, then applies `WithTermWidth(detectTermWidth(stdout))` (pretty only, exactly as today — width must NOT be probed/applied for the plain branch), `WithYes(yes)`, and `WithInteractiveStdin(signals.StdinInteractive)`. Preserve the existing detail that plain does not probe width and pretty does.
   - NOTE on the helper's return type: the two branches build different concrete types (`*PrettyPresenter` vs `*PlainPresenter`), so a single concrete return type is impossible. Resolve cleanly — e.g. the helper returns the `Presenter` interface for `New`'s use, and `NewForStartup` keeps its own per-branch construction but routes the *writer/mode decision* through a shared decision so the branch logic (which mode → which constructor + WithErr) is expressed once; OR the helper returns a small struct / both presenters share a tiny `applyStreamSplit`-style seam. The bar is: the "which mode → which constructor, wired with out/err" decision is expressed in exactly one place, and `NewForStartup` can still chain its concrete-only setters. If a fully-shared single branch is not achievable without behaviour change or contortion, take the FALLBACK.
4. Update the doc comments so the one-place claim names the actual shared location, and `NewForStartup`'s comment reflects that it delegates the mode-branch/stream-split rather than re-deriving it. Keep `NewForStartup` documented as the single PRODUCTION construction site.
5. If the preferred convergence cannot be done without changing any observable behaviour or without awkward contortion, apply the FALLBACK: re-word `New`'s doc comment to "raw/lower-level (test) wiring seam" (drop its "single wiring point" / "exactly one place" claim) and move the single-production-site language exclusively to `NewForStartup`.
6. Scope strictly to the `internal/presenter` package. Do not touch engine/main wiring, do not change any public signatures of `New` or `NewForStartup`, and do not add new EXPORTED symbols.

**Acceptance Criteria**:
- The mode-branch + stdout/stderr stream-split decision appears in exactly one place in `internal/presenter`; the other seam delegates to it (preferred) OR the doc comments are corrected so no comment falsely asserts a single-place invariant the code does not satisfy (fallback).
- `New`'s and `NewForStartup`'s public signatures are unchanged.
- `NewForStartup` still threads all four signals exactly as before: mode (via `DetectStartupSignals`), term-width (pretty only, via `detectTermWidth(stdout)`), `-y` (via `WithYes`), and stdin-interactive (via `WithInteractiveStdin(signals.StdinInteractive)`); plain still does not probe width.
- No behaviour change: existing presenter tests (including `wiring_test.go` `newSplit`, the width tests, and the forbidden-combination / gating tests) pass without modification to their assertions.
- No new exported symbols; change confined to the `internal/presenter` package; engine/main wiring untouched.

**Tests**:
- Run the full `internal/presenter` test suite and confirm all pass UNCHANGED — this is the regression guard proving no behaviour change.
- Confirm the existing assertions that `NewForStartup` arms the gating axes (`-y` and stdin-interactive) and width still hold, so convergence did not drop any axis from the production seam.
