---
phase: 1
phase_name: Presenter Seam & Render-Mode Skeleton
total: 6
---

## cli-presentation-1-1 | approved

### Task cli-presentation-1-1: Define the Presenter interface with the minimal event set

**Problem**: The whole presentation layer is built on a single dependency-inversion seam — an event/step-oriented `Presenter` interface the engine calls at lifecycle points. Nothing else in this phase (or any later phase) can be built until that contract exists. The engine emits *semantic events*; the presenter decides *how they look*. Without the interface there is no shared vocabulary for the recording presenter, the plain/pretty implementations, or the stream split.

**Solution**: Define a Go `Presenter` interface in a new `internal/presenter` package carrying the **minimal event set** needed for the walking skeleton: start-of-run, `StageStarted`, `StageSucceeded`, `StageFailed`, and end-of-run. Define the supporting payload types each event consumes, applying the spec's event-payload principle: the engine supplies in each payload every datum the renderings consume — the presenter never re-derives engine knowledge.

**Outcome**: A compilable `Presenter` interface plus payload types that every Phase 1 task (recording fake, plain impl, pretty impl, stream-split wiring) implements or drives, with no rendering logic yet.

**Do**:
- Create the Go module if not present (`go mod init` for the `mint` module) and create package directory `internal/presenter/`.
- In `internal/presenter/presenter.go` declare `type Presenter interface { ... }` with the minimal methods (illustrative signatures — exact surface is settled here, at implementation):
  - `RunStarted(info RunInfo)` — start-of-run brand/header line. `RunInfo` carries `Project`, `Version`, and an **engine-supplied action word** (`Action string`, e.g. `releasing` for `release`, `regenerating` for `regenerate`) so the start-of-run line is verb-shaped from the payload rather than hardcoding `releasing`. Per the event-payload principle the engine supplies the action word; the presenter renders it and never re-derives the verb. (This mirrors the verb-shaped end-of-run line owned by cli-presentation-4-4 and the engine-supplied brand leaf adopted in cli-presentation-1-5.)
  - `StageStarted(s StageStart)` — carries the stage `Name` and a `Blocking bool` flag (engine knowledge: it knows when it is about to invoke a long/slow command). Plain uses the flag to decide whether to emit a start line; pretty always shows progress.
  - `StageSucceeded(s StageSuccess)` — carries `Name`, `Detail string`, `Elapsed time.Duration`, and `Blocking bool` (so pretty renders `({elapsed})` on long/blocking stages only). The presenter does not time stages.
  - `StageFailed(s StageFailure)` — carries `Name`, `Message string`, and `Output string` (the captured underlying-command output). Rendering of `Output` is exercised in Phase 2; the field exists now so the contract is stable.
  - `RunFinished(r RunResult)` — end-of-run success line (carries `Project`, `Version`, optional `URL`); success-shaped (suppression on failure is a Phase 2/4 concern but the type should allow it).
- Declare the payload structs (`RunInfo`, `StageStart`, `StageSuccess`, `StageFailure`, `RunResult`) in the same package.
- Add a doc comment on the interface stating the seam contract: the engine calls these methods only and never touches colour/spinners/TTY state; the engine supplies every datum each rendering consumes.
- Keep this file rendering-free and dependency-free (only stdlib `time`); no `lipgloss`, no `fmt` output.

**Acceptance Criteria**:
- [ ] `internal/presenter/presenter.go` declares a `Presenter` interface with the five minimal methods (start-of-run, `StageStarted`, `StageSucceeded`, `StageFailed`, end-of-run).
- [ ] `RunInfo` carries an engine-supplied `Action` word (e.g. `releasing`/`regenerating`) so the start-of-run line is verb-shaped from the payload; no presenter code hardcodes the literal `releasing`.
- [ ] `StageStarted` and `StageSucceeded` payloads carry the `Blocking`/long-stage flag; `StageSucceeded` carries an engine-supplied `Elapsed`; `StageFailure` carries both `Message` and captured `Output`.
- [ ] The package compiles (`go build ./internal/presenter/`) and imports no UI library and no I/O beyond what the type definitions require.
- [ ] No method derives engine knowledge (no stage-name lists, no timing) — all such data arrives via payloads.

**Tests**:
- `"it compiles the Presenter interface and payload types"` (build/vet passes for the package).
- `"a value can satisfy the Presenter interface"` — a trivial no-op struct in a test file is assignable to `Presenter`, proving the interface is implementable.
- `"payloads expose the long/blocking flag and engine-supplied elapsed"` — a test constructs `StageStart{Blocking:true}` and `StageSuccess{Elapsed: ...}` to lock the field shapes.

**Edge Cases**: none (per the task table). The interface is a pure contract; behavioural edge cases live in the implementing tasks.

**Context**:
> The presentation layer is structured as an event/step-oriented `Presenter` interface that the engine calls at lifecycle points. The engine emits semantic events ("stage X started", …); the presenter decides how they look.
> Event payload principle: the engine supplies, in each event's payload, every datum the renderings consume — the presenter never re-derives engine knowledge or hardcodes stage-specific logic. `StageStarted` carries whether the stage is long/blocking; `StageSucceeded` carries its detail string and the elapsed time (the presenter does not time stages); `StageFailed` carries the error message and the captured underlying-command output.
> The engine never touches colour, spinners, or TTY state. It calls Presenter methods only. This mirrors the engine's existing dependency-inversion seams (`CommandRunner`, `Publisher`).
> This phase defines the *minimal* event set (start-of-run, `StageStarted`, `StageSucceeded`, `StageFailed`, end-of-run). The fuller vocabulary (`Warn`, `Unwound`, `ShowPlan`, `ShowNotes`, `Prompt`) is added in Phases 2–3 — the interface will be extended there, so design the payload structs to allow extension without churn.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam (Architecture)".

## cli-presentation-1-2 | approved

### Task cli-presentation-1-2: Recording/fake presenter for event assertions

**Problem**: The core Go rationale for the seam is testability: assert which events fired and with what payload, independent of rendering. The engine's behaviour must be verifiable without parsing styled text. A recording presenter is the tool that makes engine-driven tests possible and is itself the simplest implementation of the interface — building it second validates the interface shape from a consumer's perspective.

**Solution**: Implement a `RecordingPresenter` in the `internal/presenter` package (or a `presentertest` subpackage) that satisfies `Presenter`, captures every call in order with its full payload, and exposes accessors for assertions.

**Outcome**: Tests can drive any code through a `RecordingPresenter`, then assert the exact sequence and payloads of events fired — with zero events and multi-stage sequences both handled cleanly.

**Do**:
- Create `internal/presenter/recording.go` (or `internal/presenter/presentertest/recording.go`).
- Define `type RecordingPresenter struct { Events []Event }` where `Event` is a recorded entry capturing the method called and its payload. Use either a tagged struct (`Event{ Kind EventKind; RunInfo *RunInfo; StageStart *StageStart; ... }`) or one slice per event type — choose the form that makes assertions readable; document the choice.
- Implement every `Presenter` method to append a record preserving call order (append to a single ordered slice so interleaving across event types is captured).
- Add ergonomic accessors for tests, e.g. `Kinds() []EventKind` (ordered list of event kinds) and a helper to fetch the nth event, so a test can assert order without reaching into raw fields.
- Ensure a freshly constructed `RecordingPresenter` records zero events (nil/empty slice, no panic on accessors).
- No rendering, no I/O.

**Acceptance Criteria**:
- [ ] `RecordingPresenter` satisfies the `Presenter` interface (assignable to `Presenter`).
- [ ] Every method call is recorded in call order with its complete payload retrievable.
- [ ] A new `RecordingPresenter` with no calls reports zero events and its accessors return empty results without panicking (zero-events edge case).
- [ ] A sequence of multiple stages (`StageStarted`→`StageSucceeded`, repeated, interleaved with `RunStarted`/`RunFinished`) is recorded in the exact order issued (multiple-stages edge case).

**Tests**:
- `"it records a single StageSucceeded with its full payload"` — assert name, detail, elapsed, blocking all captured.
- `"it records events in call order across event types"` — drive `RunStarted`, `StageStarted`, `StageSucceeded`, `RunFinished` and assert `Kinds()` matches the issued order (multiple stages in sequence).
- `"it reports zero events before any call"` — new recorder, accessors return empty, no panic (zero events recorded).
- `"it satisfies the Presenter interface"` — compile-time assignment `var _ Presenter = (*RecordingPresenter)(nil)`.

**Edge Cases**:
- **Zero events recorded** — a recorder that has been constructed but never called must not panic and must report an empty event list.
- **Multiple stages in sequence** — repeated stage cycles must be recorded in issue order with no collapsing or de-duplication.

**Context**:
> Testability (the core Go rationale): assert which events fired and with what payload, independent of rendering. A `plain` impl is trivially assertable; a fake/recording presenter verifies engine behaviour without parsing styled text.
> Built independently of the engine — a recording presenter and fixed event sequences are enough to build and assert rendering.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The `Presenter` Seam (Architecture)" (Testability decision) and "Dependencies → Notes".

## cli-presentation-1-3 | approved

### Task cli-presentation-1-3: Startup render-mode selection (TTY detection, no sniffing)

**Problem**: Mode must be selected **once at startup** and never re-checked downstream. The selection rule is precise and the spec is emphatic about what is *banned*: no environment sniffing. Getting this wrong (e.g. consulting `TERM`/`CI`) would violate a core design decision and silently change behaviour for agents and humans. This task isolates the selection logic so it can be unit-tested without a real terminal.

**Solution**: Implement a pure mode-selection function plus a thin startup wiring that returns the chosen mode given the `--plain` flag and the stdout file descriptor, using OS-reported stream type only. Selection precedence: `--plain` → `plain`; else `isatty(stdout)` → `pretty`; non-TTY → `plain`.

**Outcome**: A single, tested decision point produces `pretty` or `plain` from `(plainFlag, stdout)` and nothing downstream re-checks the flag or TTY.

**Do**:
- Add a `Mode` type to `internal/presenter` (e.g. `type Mode int; const ( ModePlain Mode = iota; ModePretty )`).
- Implement a **testable pure core**: `func SelectMode(plainFlag bool, isTTY bool) Mode` applying the precedence — `plainFlag` → `ModePlain`; else `isTTY` → `ModePretty`; else `ModePlain`. This function takes the already-resolved TTY boolean so it is trivially unit-testable without a device.
- Implement the OS edge: `func IsTerminal(f *os.File) bool` wrapping `golang.org/x/term`'s `term.IsTerminal(int(f.Fd()))` (the spec's stated equivalent of `os.Stdout.Stat().Mode() & os.ModeCharDevice != 0`). Add `golang.org/x/term` to `go.mod`.
- Provide the startup wiring: `func DetectMode(plainFlag bool, stdout *os.File) Mode { return SelectMode(plainFlag, IsTerminal(stdout)) }`.
- Do **not** read `LANG`, `LC_*`, `TERM`, `CI`, or `NO_COLOR` anywhere in this code path. Add a code comment stating the no-sniffing ban explicitly so a future contributor does not "helpfully" add it.
- Selection happens once; expose the chosen `Mode` (or the constructed presenter) to be passed down — nothing downstream re-detects.

**Acceptance Criteria**:
- [ ] `SelectMode(true, true)` → `ModePlain` (`--plain` on a TTY still selects plain).
- [ ] `SelectMode(false, true)` → `ModePretty`; `SelectMode(false, false)` → `ModePlain` (piped/redirected stdout selects plain).
- [ ] Setting `LANG`/`LC_ALL`/`TERM`/`CI`/`NO_COLOR` env vars does not change the selected mode for any `(plainFlag, isTTY)` combination — none of these are read in the selection path.
- [ ] `IsTerminal` is the sole TTY signal and uses `term.IsTerminal(int(f.Fd()))`; the selection path consults no environment variable.
- [ ] Mode is resolved at a single call site; there is no second TTY/flag check downstream.

**Tests**:
- `"--plain forces plain even on a TTY"` — `SelectMode(true, true) == ModePlain`.
- `"a TTY without --plain selects pretty"` — `SelectMode(false, true) == ModePretty`.
- `"piped/redirected (non-TTY) stdout selects plain"` — `SelectMode(false, false) == ModePlain`.
- `"environment is not sniffed for mode selection"` — set `t.Setenv` for `LANG`, `LC_ALL`, `TERM=xterm`, `CI=true`, `NO_COLOR=1`, then assert `SelectMode`/`DetectMode` output is governed only by `(plainFlag, isTTY)` and is unchanged by the env (LANG/LC_*/TERM/CI/NO_COLOR set but ignored).

**Edge Cases**:
- **`--plain` on a TTY** — explicit flag wins over the char-device signal; selects `plain`.
- **Piped/redirected stdout** — non-TTY selects `plain`; this *is* the intended force-plain path for a human capturing output.
- **Env vars set but ignored** — `LANG`/`LC_*`/`TERM`/`CI`/`NO_COLOR` present must not influence the decision (no sniffing).

**Context**:
> Precedence: `--plain` passed → `plain`; otherwise `isatty(stdout)` → `pretty`, non-TTY → `plain`. Detection uses the OS-reported stream type: `term.IsTerminal(int(os.Stdout.Fd()))` (equivalently `os.Stdout.Stat().Mode() & os.ModeCharDevice != 0`).
> No sniffing of `LANG`/`LC_*`/`TERM`/`CI`. A flag is an explicit instruction; the ban is on guessing from the environment. `NO_COLOR` env handling is out of scope (it's sniffing; `--plain` is the explicit equivalent).
> A human who pipes/redirects gets `plain` — exactly what's wanted when capturing output, and this is the force-plain path.
> Selected once at startup (`--plain` if passed, else `isatty(stdout)`). Nothing downstream re-checks the TTY or the flag.
> A real-but-colour-incapable TTY (`TERM=dumb`) is still selected as `pretty` by `isatty(stdout)` — colour incapability is handled later by lipgloss auto-downgrade (Task cli-presentation-1-5), not by the mode selector. The mode selector must therefore *not* special-case `TERM`.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Render-Mode Detection & Output Streams".

## cli-presentation-1-4 | approved

### Task cli-presentation-1-4: Plain presenter renders the minimal stage sequence

**Problem**: The `plain` presenter is the agent/token-efficient implementation — terse `key: value` text, no ANSI, no glyphs, no animation, and crucially **no UI library**. Building it on the minimal event set proves the seam end-to-end for the cheap mode and establishes the plain vocabulary the later phases extend.

**Solution**: Implement a `PlainPresenter` satisfying `Presenter` that renders the minimal stage sequence (start-of-run → a stage success → end-of-run) as terse lowercase `key: value` lines using only `fmt`, writing narration to an injected `io.Writer`.

**Outcome**: Given an event stream, `PlainPresenter` emits the spec's plain lines (`mint: {action} {project} v{X}` — e.g. `mint: releasing acme v1.4.0` — `{stage}: {detail}`/`{stage}: ok`, `done: {project} v{X} {url}`) with the start-of-run action word taken from the engine-supplied `RunInfo.Action` (not hardcoded), and with zero ANSI/glyph/animation bytes and no UI-library import.

**Do**:
- Create `internal/presenter/plain.go` with `type PlainPresenter struct { out io.Writer; err io.Writer }` and a constructor `NewPlainPresenter(out, err io.Writer) *PlainPresenter`. (The `err` writer is wired fully in Task cli-presentation-1-6; accept it now so the constructor is stable.)
- Implement the minimal methods using only `fmt.Fprintf(p.out, …)`:
  - `RunStarted` → `mint: {action} {project} v{X}`, where `{action}` is the **engine-supplied** action word from `RunInfo.Action` (`releasing` for `release`, `regenerating` for `regenerate`) — render the supplied action, do **not** hardcode `releasing` (the same start-of-run event is reused for `regenerate` per cli-presentation-4-2, which requires the plain line `mint: regenerating {project} v{X}`). This mirrors the pretty brand line in cli-presentation-1-5 and honours 1-1's "no presenter code hardcodes the literal `releasing`."
  - `StageStarted` → emit a terse start line **only when `Blocking` is true** (e.g. `{name}: …`); short stages emit nothing on start. (Full long/blocking start-line wording is hardened in Phase 2; for the skeleton, honour the flag.)
  - `StageSucceeded` → `{stage}: {detail}` (or `{stage}: ok` when detail is empty).
  - `StageFailed` → `{stage}: FAILED - {message}` (captured-output delimiter block is a Phase 2 concern; the one-line summary is enough for the skeleton — stderr duplication wired in Task cli-presentation-1-6).
  - `RunFinished` → `done: {project} v{X} {url}`.
- Import **only** `fmt`/`io` (stdlib). Do **not** import `lipgloss`, any spinner package, or anything that emits ANSI.
- Lines are lowercase and terse per the plain contract.

**Acceptance Criteria**:
- [ ] `PlainPresenter` satisfies `Presenter` and writes narration to the injected `out` writer.
- [ ] A minimal sequence (`RunStarted` → `StageSucceeded` → `RunFinished`) produces the expected terse lines in order.
- [ ] The plain start-of-run line renders the engine-supplied `RunInfo.Action` word (e.g. `mint: releasing {project} v{X}` for `release`); no plain presenter code hardcodes the literal `releasing` (so `regenerate` renders `mint: regenerating {project} v{X}` per cli-presentation-4-2).
- [ ] Output contains **no** ESC (`0x1b`) byte, no braille/emoji glyphs, and no carriage-return animation — verified by scanning the captured bytes (no ANSI/glyph/animation edge case).
- [ ] The `internal/presenter` plain code path imports no UI library (verified by inspecting imports / a build-constraint or dependency assertion) — `plain` mode pulls in no UI library.
- [ ] A short (non-blocking) `StageStarted` emits no start line; a blocking `StageStarted` does.

**Tests**:
- `"it renders the minimal stage sequence as terse key:value lines"` — capture into a `bytes.Buffer`, assert exact line sequence for start-of-run → stage success → end-of-run.
- `"the start-of-run line renders the engine-supplied action word"` — `RunStarted{Action:"regenerating", Project:"acme", Version:"1.4.0"}` → `mint: regenerating acme v1.4.0`; `Action:"releasing"` → `mint: releasing acme v1.4.0` (no hardcoded `releasing`).
- `"it emits no ANSI, glyph, or animation bytes"` — assert the buffer contains no `0x1b`, no `\r`, and no characters above the basic ASCII set used by the contract (no ANSI/glyph/animation).
- `"it omits a start line for short stages and emits one for blocking stages"` — drive both, assert presence/absence.
- `"the plain package imports no UI library"` — a guard test (e.g. parse the package's imports or assert no `lipgloss`/spinner dependency is reachable from plain).

**Edge Cases**:
- **No ANSI/glyph/animation bytes in output** — the captured output must be pure terse text.
- **No UI-library import** — the plain path must not pull in `lipgloss` or a spinner package.

**Context**:
> The `plain` presenter renders the same `Presenter` events as terse, token-efficient text — no animation, no glyphs, no colour. `key: value` lines, lowercase, one per stage on completion. Start line for long/blocking stages only.
> Per-event rendering (plain): start of run → `mint: releasing {project} v{X}` (the release-shaped instance — the `{action}` word is engine-supplied and verb-shaped, so `regenerate` renders `mint: regenerating {project} v{X}`); `StageSucceeded` → `{stage}: ok` / `{stage}: {detail}`; `StageFailed` → `{stage}: FAILED - {message}`; end of run → `done: {project} v{X} {url}`.
> Applies to every verb — `release`, `regenerate`, `init`, `version` all emit through the same `Presenter`; the start-of-run action word is therefore verb-shaped from `RunInfo.Action` (cli-presentation-1-1), not a release-only literal.
> `plain` mode pulls in no UI library — just `fmt` lines. That is the point of token-efficiency.
> Exact wording is refinable; the byte-purity (no ANSI/glyph/animation) and the no-UI-library constraints are the hard requirements for this task.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Plain Layer", "Library Selection".

## cli-presentation-1-5 | approved

### Task cli-presentation-1-5: Pretty presenter renders the minimal stage sequence via lipgloss

**Problem**: The `pretty` presenter is the human implementation — brand line, colour, status glyphs — styled exclusively via `lipgloss`. The skeleton must prove pretty renders the minimal sequence through the same event stream as plain, and must demonstrate that lipgloss's built-in colour auto-downgrade handles colour-incapable terminals **for free** (no third mode, no `mint`-side colour logic). Spinner animation is explicitly out of scope for this phase (hardened in Phase 4).

**Solution**: Implement a `PrettyPresenter` satisfying `Presenter` that renders the minimal stage sequence (start-of-run brand line → a `✓` stage success → end-of-run brand line) using `lipgloss` for colour and glyph styling, writing narration to an injected `io.Writer`, and relying on lipgloss colour auto-downgrade rather than any custom colour gating.

**Outcome**: Given an event stream, `PrettyPresenter` emits the brand line, an indented `✓ {stage} {detail}` line, and the closing brand line; when colour is unavailable (downgrade), it emits no colour codes while keeping layout and glyphs.

**Do**:
- Add `github.com/charmbracelet/lipgloss` to `go.mod`. Create `internal/presenter/pretty.go` with `type PrettyPresenter struct { out io.Writer; err io.Writer; ... }` and constructor `NewPrettyPresenter(out, err io.Writer) *PrettyPresenter`.
- Implement the minimal methods:
  - `RunStarted` → top brand line `🌿 mint · {project}  ›  {action} v{X}` (flush-left), where `{action}` is the **engine-supplied** action word from `RunInfo` (`releasing` for `release`, `regenerating` for `regenerate`) — render the supplied action, do not hardcode `releasing` (the same start-of-run event is reused for `regenerate` per cli-presentation-4-2). The brand leaf is likewise **engine-supplied** (carried on the start-of-run/end-of-run payload, defaulting to `🌿`) rather than hardcoded, honouring the spec's "leaf ties to `commit_prefix`" note and the event-payload principle; render the supplied leaf, do not re-derive it.
  - `StageSucceeded` → two-space indent, green `✓` glyph, stage name padded to a column, terse detail: `  ✓ {stage}  {detail}`. Render `({elapsed})` only when `Blocking` is true (short stages render detail without elapsed).
  - `StageStarted` → for the skeleton, render a dim stage line (no spinner animation — Phase 4 owns the spinner lifecycle); keep it a single printed line so the flow is linear.
  - `StageFailed` → red `✗ {stage}  {message}` line (captured-output rendering is Phase 2).
  - `RunFinished` → bottom brand line `🌿 released {project} v{X} · {url}`.
- Use `lipgloss` styles for colour (green/red/amber) and let lipgloss own colour-capability downgrade. Do **not** check `TERM`/`NO_COLOR` or implement custom colour gating — rely on lipgloss's auto-downgrade.
- To make downgrade testable, allow forcing lipgloss's colour profile in tests (e.g. set the renderer's colour profile to `Ascii`/no-colour for the downgrade test and to a colour profile for the colour test) via an injectable `*lipgloss.Renderer` so the presenter does not read global TTY state at render time.
- Print-style linear narration only — no alt-screen, no Bubble Tea.

**Acceptance Criteria**:
- [ ] `PrettyPresenter` satisfies `Presenter` and writes narration to the injected `out` writer.
- [ ] A minimal sequence renders the top brand line, an indented `✓` stage-success line with the stage detail, and the bottom brand line, in order.
- [ ] With colour forced **on**, output contains ANSI colour codes around the glyph/text; the layout (indent, glyph, padded name) is present.
- [ ] With colour **downgraded** (no-colour profile), output contains **no** ANSI colour escape codes, while layout and glyphs (`✓`, brand leaf) are preserved (colour auto-downgrade edge case).
- [ ] Styling goes exclusively through `lipgloss`; there is no custom `NO_COLOR`/`TERM` check in the pretty path.
- [ ] The brand leaf is rendered from the engine-supplied payload datum (defaulting to `🌿`), not re-derived/hardcoded in the presenter.
- [ ] The start-of-run brand line renders the engine-supplied `Action` word from `RunInfo` (not a hardcoded `releasing`).

**Tests**:
- `"it renders the minimal pretty stage sequence with brand lines and a check glyph"` — capture to a buffer, assert brand top line, `✓` stage line, brand bottom line in order.
- `"colour-on output contains ANSI colour codes"` — force a colour profile, assert ESC-based SGR codes present.
- `"colour auto-downgrade emits no colour codes but preserves layout and glyphs"` — force the no-colour profile, assert no SGR colour codes yet the `✓` glyph, indentation, and brand leaf remain (colour auto-downgrade on a non-colour-capable TTY; layout/glyphs preserved under downgrade).
- `"elapsed renders only on blocking stages"` — assert `(…s)` appears for a blocking stage success and not for a short one.

**Edge Cases**:
- **Colour auto-downgrade** — on a non-colour-capable TTY the renderer emits no colour codes; this is lipgloss behaving correctly underneath, not a third `mint` mode.
- **Layout/glyphs preserved under downgrade** — indentation, padded stage column, status glyph, and brand leaf must survive the colour downgrade.

**Context**:
> `lipgloss` for all `pretty`-mode styling — colour, the 🌿 brand line, status glyphs, the titled notes rule. It is pure string styling (no event loop), so it composes with the `Presenter` seam, and it auto-downgrades colour when piped (also relied on for colour-incapable TTYs).
> NOT Bubble Tea / no alt-screen / no full-screen TUI. Print-style linear narration only.
> Brand lines — Top: `🌿 mint · {project}  ›  releasing v{X}`; Bottom: `🌿 released {project} v{X} · {url}`. Status glyphs: `✓` success (green) · `✗` failure (red) · `⚠` warn (amber) · `↩` auto-unwind. Stage lines: two-space indent, glyph, stage name padded to a column, terse detail.
> Brand-leaf provenance: the spec notes "the leaf ties to the engine's `commit_prefix` brand." Per the event-payload principle the engine supplies every datum the rendering consumes, so the leaf glyph should arrive in the start-of-run / end-of-run payload (e.g. a `Leaf`/`Brand` field on `RunInfo`/`RunResult`, defaulting to `🌿`) rather than being hardcoded in the presenter — the presenter renders the engine-supplied leaf so a customised `commit_prefix` brand stays consistent. Every worked example uses the default `🌿`; if a fixed constant leaf is preferred, the field can be omitted and the literal `🌿` rendered.
> A real-but-colour-incapable TTY (`TERM=dumb`) is still selected as `pretty`; `mint` leans on lipgloss's built-in colour auto-downgrade, which emits no colour codes there while keeping layout and glyphs. This is not a third `mint` mode.
> Spinners are out of scope for this phase — the spinner lifecycle (started on `StageStarted`, replaced in place on completion) is hardened in Phase 4. For the skeleton, `StageStarted` renders a static dim line.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Pretty Layer", "Library Selection", "Render-Mode Detection & Output Streams" (colour-incapable terminal).

## cli-presentation-1-6 | approved

### Task cli-presentation-1-6: Wire and verify the stdout/stderr stream split

**Problem**: The stream contract is fixed and independent of render mode: run narration → stdout; errors and warnings → stderr (and also in the narration). This guarantees that under redirection (`mint release > run.log`) a failure still reaches the terminal via stderr and cannot silently vanish, while a successful run leaves nothing on stderr. The split must be wired through both presenters and verified end-to-end on the minimal event set.

**Solution**: Wire both `PlainPresenter` and `PrettyPresenter` to write narration to their `out` writer and to additionally write the one-line failure summary to their `err` writer, then verify the split with tests that capture stdout and stderr separately for both a success and a failure run.

**Outcome**: On success, all output is on stdout and stderr is empty; on failure, the failure appears in the stdout narration **and** the one-line summary appears on stderr (the multi-line captured body, when present in later phases, is not duplicated to stderr).

**Do**:
- Confirm both presenters take separate `out` and `err` `io.Writer`s (added in Tasks cli-presentation-1-4 and cli-presentation-1-5). Provide a default wiring (`out = os.Stdout`, `err = os.Stderr`) at the startup construction site alongside mode selection (Task cli-presentation-1-3).
- In both presenters, route **all narration** (brand lines, stage lines, end-of-run) to `out`.
- For `StageFailed`, write the rendered failure to `out` (narration) **and** write the **one-line** `FAILED`/error summary to `err`. Do not duplicate any multi-line captured body to `err` (the captured-output body is a Phase 2 addition; keep the rule explicit now so Phase 2 inherits it).
- Ensure `RunStarted`, `StageStarted`, `StageSucceeded`, `RunFinished` write **only** to `out` — never to `err` — so a successful run leaves stderr empty.
- Add a small construction helper (e.g. `func New(mode Mode, out, err io.Writer) Presenter`) returning the correct implementation for the selected mode, so the split and the mode selection meet at one wiring point.

**Acceptance Criteria**:
- [ ] Both presenters accept distinct `out`/`err` writers; default startup wiring uses `os.Stdout`/`os.Stderr`.
- [ ] A success run (`RunStarted` → `StageSucceeded` → `RunFinished`) writes narration to stdout and leaves stderr **empty** (narration absent from stderr on success).
- [ ] A failure run writes the failure into the stdout narration **and** writes the one-line `FAILED`/error summary to stderr (failure summary on stderr and in stdout narration).
- [ ] The multi-line captured body is not duplicated to stderr (asserted as a contract now; exercised fully once captured output lands in Phase 2).
- [ ] Both `PlainPresenter` and `PrettyPresenter` obey the identical split (the split is fixed regardless of render mode).

**Tests**:
- `"a success run writes narration to stdout and nothing to stderr"` — separate buffers; assert stderr buffer is empty for both modes.
- `"a failure summary appears on stderr and in the stdout narration"` — drive `StageFailed`; assert the one-line summary is present in both the stdout buffer and the stderr buffer, for both modes (failure summary on stderr and in stdout narration).
- `"non-failure events never write to stderr"` — drive `RunStarted`/`StageStarted`/`StageSucceeded`/`RunFinished` only; assert stderr empty (narration absent from stderr on success).
- `"the constructor selects the implementation matching the mode and wires both writers"` — `New(ModePlain, …)` returns plain, `New(ModePretty, …)` returns pretty, each writing to the provided writers.

**Edge Cases**:
- **Failure summary on stderr and in stdout narration** — the one-line summary is duplicated to stderr while remaining part of the stdout narration.
- **Narration absent from stderr on success** — a clean run must leave stderr empty so redirected logs don't mix streams and a real failure stays visible.

**Context**:
> Output streams — narration is the product, so it's stdout: run narration → stdout (stages, the plan, the notes preview, the final summary, and `mint version`'s value). Errors + warnings → stderr — for visibility under redirection. `mint release > run.log` sends stdout to the file, but a failure on stderr still reaches the terminal and cannot silently vanish. Errors/warnings appear in both the narration and on stderr.
> An agent capturing combined output (`2>&1`) by default sees narration and errors regardless of the split.
> `StageFailed` … the captured output is narration → stdout; per the stream contract the one-line `FAILED`/error summary additionally goes to stderr for redirect-visibility (the multi-line captured body is not duplicated to stderr).
> The stream split is fixed regardless of mode (one of the three orthogonal axes).
> Exit-code ownership stays with the engine/`main`, not the `Presenter` — the presenter is render-only and has no say in process status. Do not set exit codes here.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Render-Mode Detection & Output Streams" (Output streams), "Scope & Output Modes" (three orthogonal axes).
