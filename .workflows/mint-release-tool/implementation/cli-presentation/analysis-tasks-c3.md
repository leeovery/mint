---
topic: cli-presentation
cycle: 3
total_proposed: 2
---
# Analysis Tasks: CLI Presentation (Cycle 3)

## Task 1: Consolidate the prompt/gate test drivers onto the shared gate_helpers_test.go construction seam
status: approved
severity: medium
sources: duplication

**Problem**: The presenter test suite carries three independently-authored "drive one Prompt across both modes" driver tables and four near-identical single-Prompt construct-and-capture helpers, all doing the same job — build a Plain/Pretty presenter from an injected `strings.Reader` script, call `Prompt` once, and return the captured buffer(s). The three driver tables are `promptModeDriver`/`promptModeDrivers` (`prompt_test.go`), `promptDriver`/`promptDrivers` (`prompt_render_only_test.go`), and `gateDriver`/`gateDrivers` (`gate_helpers_test.go`). `promptModeDriver` and `promptDriver` share an identical struct shape, and the `prompt_test.go` doc comment itself states it "mirrors promptDriver in prompt_render_only_test.go". The four single-Prompt helpers are `drivePlainPrompt`/`drivePrettyPrompt` (`prompt_test.go`) and `drivePrettyPromptProfile` (`pretty_gate_test.go`), which re-implement the same buffer-allocation + presenter-construction idiom that `plainGate`/`prettyGate` (`gate_helpers_test.go`) already centralise. They have already drifted: `promptDrivers` pins the pretty arm to `TrueColor`, the others to `Ascii`; `drivePrettyPrompt` is a redundant Ascii-pinned specialisation of the more general profile-taking `drivePrettyPromptProfile`. Test-side copy-paste drift; production code is well-factored and not touched.

**Solution**: Make `plainGate`/`prettyGate` the single construction seam, build the prompt drivers on top of it, collapse the redundant single-Prompt helpers (replace `drivePrettyPrompt` with `drivePrettyPromptProfile(..., termenv.Ascii)`), and remove the redundant parallel mode-driver tables, parameterising the forced colour profile at the one remaining table.

**Outcome**: One shared construction seam and one shared mode-driver table underpin all prompt/gate tests; the colour profile is an explicit parameter; no test loses coverage; every previously-asserted property still runs against both real presenters.

**Do** (find exact symbols/sites by grepping — line numbers are approximate):
1. Confirm/adjust `plainGate`/`prettyGate` in `gate_helpers_test.go` expose what the prompt drivers need (build each presenter from an injected reader, return captured out/err, pretty profile as a parameter).
2. Rewrite `drivePrettyPrompt` as a thin call to `drivePrettyPromptProfile(input, gate, termenv.Ascii)` (or delete it and update call sites) so there is exactly ONE pretty-prompt driver.
3. Reimplement the remaining prompt drivers so their construction half delegates to the `plainGate`/`prettyGate` seam rather than re-allocating buffers and re-calling constructors inline.
4. Pick ONE canonical mode-driver table (reuse the richest — `gateDriver`/`gateDrivers`, or a shared table parameterising the pretty profile). Remove `promptModeDriver`/`promptModeDrivers` and `promptDriver`/`promptDrivers` once call sites consume the shared table.
5. At the render-only call site that deliberately needs a colour-CAPABLE profile (the screen-control guard in `prompt_render_only_test.go`, which must pass while lipgloss emits SGR colour escapes), pass `termenv.TrueColor` through the parameterised table so the intent is explicit.
6. Update all affected call sites; delete now-unreferenced helpers/types.

**Acceptance Criteria**:
- Only one "drive one Prompt across both modes" mode-driver table remains; the two redundant ones are removed.
- Exactly one pretty-prompt single-Prompt driver; `drivePrettyPrompt` no longer exists as an independent Ascii-pinned reimplementation.
- The remaining prompt drivers build via the `plainGate`/`prettyGate` seam, not inline construction.
- The forced colour profile is an explicit parameter; the render-only screen-control guard still runs under `TrueColor`.
- No production (`.go` non-test) file in `internal/presenter` is modified.

**Tests**:
- `go test ./internal/presenter/...` passes with every previously-asserted prompt/gate property still exercised against both real presenters.
- The render-only screen-control guard still asserts no clear/alt-screen/home sequence while SGR colour escapes ARE present (TrueColor path survived).
- `go vet ./internal/presenter/...` reports no unused functions/types left behind.

## Task 2: Converge the startup wiring so one entry point consumes StartupSignals and threads all axes
status: approved
severity: medium
sources: architecture

**Problem**: The presenter package grew three overlapping startup constructs that do not converge. `DetectStartupSignals` (`gating.go`) returns `StartupSignals{Mode, StdinInteractive}` resolving both orthogonal axes once, but nothing in production consumes it (only its own test). `NewForStartup` (`wiring.go`), documented as "the default startup convenience" / "the one construction site", calls `DetectMode` directly and threads only `Mode` + terminal width; it never threads `-y` (`WithYes`) or stdin-interactive (`WithInteractiveStdin`), which are reachable only via builder setters deferred to "a later main/cmd task". Net effect: two drifted startup paths — `StartupSignals.StdinInteractive` wired to nothing, while `NewForStartup` cannot produce a presenter that reaches the forbidden-combination fail-loud path (a load-bearing spec requirement: non-TTY stdin without `-y` must fail loud) because it leaves both gating fields at interactive defaults. The package's outermost surface is ambiguous.

**Solution**: Make `NewForStartup` (or one entry point alongside it) the single seam that consumes the resolved `StartupSignals` and threads all four signals — `Mode`, terminal width, `-y`, and `StdinInteractive` — onto the returned presenter. `-y` becomes a parameter the caller passes; stdin-interactive is detected from the stdin descriptor via the existing `DetectStdinTTY`/`StdinIsInteractive` primitives. STRICTLY scoped to the presenter package — do NOT build a main/cmd package.

**Outcome**: One converged startup path; `StartupSignals` is consumed (no longer dead); the documented seam threads all axes and a presenter built through it with non-interactive stdin and `-y` unset reaches the fail-loud forbidden-combination path.

**Do**:
1. Decide the single converged entry point: extend `NewForStartup`'s signature to take the `-y` decision and the stdin handle, OR add one new sibling constructor that supersedes it as the documented startup seam. Keep `New(mode, out, err)` as the lower-level mode+stream wiring point unchanged.
2. Inside it, resolve both axes via `DetectStartupSignals(plainFlag, stdout, stdin)` (or its `DetectMode` + `DetectStdinTTY`/`StdinIsInteractive` constituents) so `StartupSignals` is actually consumed.
3. Thread all four axes onto the returned presenter: select implementation by `signals.Mode`; for pretty apply `WithTermWidth(detectTermWidth(stdout))` as today; on the returned implementation apply `WithYes(yes)` from the caller-supplied `-y` parameter and `WithInteractiveStdin(signals.StdinInteractive)`.
4. Take the stdin handle as a parameter (`*os.File`, mirroring stdout/stderr) so the function stays unit-testable with `/dev/null`; never reach for `os` globals internally.
5. Update the doc comments on the converged entry point and trim the now-obsolete "deferred to a later main/cmd task" deferrals on `WithYes`/`WithInteractiveStdin` and `StartupSignals`/`DetectStartupSignals`. Do NOT add a main/cmd package — `-y` stays a parameter the (future) caller supplies.
6. Update any existing `NewForStartup` callers/tests to the converged form (or keep a thin shim defaulting `-y`=false and interactive stdin to the detected value), ensuring the production-default startup path goes through the converged seam.

**Acceptance Criteria**:
- One production startup entry point consumes `StartupSignals` (or its detectors) and threads `Mode`, width, `-y`, and `StdinInteractive`; `StartupSignals` is no longer referenced only by its own test.
- `-y` is a parameter into the startup seam; stdin-interactive is detected from the stdin descriptor, not derived from `Mode`.
- A presenter built through the converged seam with a non-interactive stdin handle and `-y` unset reaches the forbidden-combination fail-loud path.
- All wiring stays inside `internal/presenter`; no main/cmd package introduced.
- stdin/stdout/stderr handles are parameters; no `os` globals reached internally; unit-testable with non-TTY handles.

**Tests**:
- A test builds a presenter via the converged seam with a non-TTY stdin handle (`/dev/null`) and `-y` unset, then asserts the forbidden-combination fail-loud behaviour triggers on a gate prompt.
- A test passes `-y` true and asserts the gate auto-confirms without reading stdin (WithYes axis threaded).
- A test covers all four TTY combinations of stdout/stdin against the seam and asserts the presenter's render mode + stdin-interactive gating match the independently-resolved StartupSignals (render axis from stdout, gating axis from stdin; neither derived from the other).
- `go test ./internal/presenter/...` and `go vet ./internal/presenter/...` pass with no unused `StartupSignals`/`DetectStartupSignals` symbols remaining.
