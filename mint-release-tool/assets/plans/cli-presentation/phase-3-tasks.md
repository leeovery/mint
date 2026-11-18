---
phase: 3
phase_name: Interactive Gating — Prompt, Input Model & `-y` Orthogonality
total: 8
---

## cli-presentation-3-1 | approved

### Task cli-presentation-3-1: Gate model with declared choice set rendered/returned by Prompt

**Problem**: A *gate* is described by the choices it offers, and one `Prompt` method must render every gate variant in `mint` — the four-choice (`y`/`n`/`e`/`r`) notes-review gate and the two-choice (`y`/`n`) reuse confirm — returning exactly one of the declared choices. Before any rendering (3-4), input parsing (3-3), `-y` skip (3-5), or fail-loud (3-6) task can be built, the `Gate` payload that carries the choice set, the per-choice action labels, and the declared default must exist and be locked into the `Presenter` interface. Building the model first prevents the rest of the phase being authored against a moving contract — and crucially the model must be data-driven (the presenter must not hardcode the y/n/e/r set or assume yes-default), so both the four-choice and two-choice gates flow through the same method.

**Solution**: Add a `Gate` payload type to `internal/presenter` carrying an ordered slice of declared choices (each a key + action label) plus the declared default key, add `Prompt(gate Gate) (Choice, error)` to the `Presenter` interface, record it in `RecordingPresenter`, and provide the two canonical gate constructors (notes-review four-choice, reuse-confirm two-choice). This task fixes the data model and the interface signature; the actual line-read parsing (3-3) and pretty menu (3-4) are layered on top in later tasks. The returned `Choice` is always a member of the gate's declared set.

**Outcome**: A `Gate` value carries its ordered choice set, per-choice action labels, and a declared default key; `Prompt(gate)` is on the interface and returns one of the gate's declared choices; the notes-review gate declares `y`/`n`/`e`/`r` (default `y`) and the reuse confirm declares `y`/`n` (default `y`); a default outside `y` is representable, and a choice outside the declared set is never returned.

**Do**:
- In `internal/presenter/presenter.go`, define the gate model:
  - `type Choice string` (e.g. constants `ChoiceYes = "y"`, `ChoiceNo = "n"`, `ChoiceEdit = "e"`, `ChoiceRegen = "r"`) — the key a user types.
  - `type GateChoice struct { Key Choice; Action string }` — one selectable option, where `Action` is the human label (e.g. `accept & proceed`, `abort`, `edit in $EDITOR`, `regenerate`). Keep order significant (the slice order is the render order).
  - `type Gate struct { Prompt string; Choices []GateChoice; Default Choice }` — `Prompt` is the question text (e.g. `Continue?`), `Choices` is the ordered declared set, `Default` is the key that fires on empty-Enter. `Default` must be one of the `Choices` keys.
- Add `Prompt(gate Gate) (Choice, error)` to the `Presenter` interface. (The `error` return carries the forbidden-combination / EOF failure surfaced in 3-3/3-6; declare it now so the signature is stable.) Add a doc comment: `Prompt` is **render-only** — it renders the choice set, reads one line, and returns a declared choice; it never invokes `$EDITOR` or `claude` (the engine owns `e`/`r` re-entry — see 3-8).
- Provide gate constructors in the package so the engine and tests build gates consistently, not by hand:
  - `func NotesReviewGate() Gate` → `Prompt:"Continue?"`, choices in order `y` (`accept & proceed`), `n` (`abort`), `e` (`edit in $EDITOR`), `r` (`regenerate`), `Default: ChoiceYes`.
  - `func ReuseConfirmGate() Gate` → `Prompt:"Continue?"`, choices in order `y` (`accept & proceed`), `n` (`abort`), `Default: ChoiceYes`. No `e`/`r` (there are no freshly-generated notes to edit or regenerate).
- Add the record implementation to `RecordingPresenter` (capture the `Gate` argument; for the recorder, return a configurable canned `Choice` so engine-driven tests can script the answer — document the canned-answer mechanism, e.g. a `NextChoice`/queue field).
- Add a validation helper used by the parsing/rendering tasks: `func (g Gate) Has(c Choice) bool` (membership test against the declared set) and `func (g Gate) Keys() []Choice`. The presenter must rely on these — it must **not** hardcode the y/n/e/r set anywhere.
- Keep this file rendering-free (no `fmt` output, no `lipgloss`) — it is pure model plus interface, consistent with the Phase 1/2 `presenter.go` discipline.

**Acceptance Criteria**:
- [ ] `Gate` carries an ordered `[]GateChoice` (key + action label) and a `Default` key that is a member of the choice set.
- [ ] `Prompt(gate Gate) (Choice, error)` is on the `Presenter` interface and recorded by `RecordingPresenter`.
- [ ] `NotesReviewGate()` declares exactly `y`/`n`/`e`/`r` in that order with default `y` and the spec's action labels.
- [ ] `ReuseConfirmGate()` declares exactly `y`/`n` in that order with default `y` and no `e`/`r`.
- [ ] A gate can declare a non-`y` default (the model does not assume yes-default); `Gate.Has`/`Gate.Keys` operate on the declared set, not a hardcoded list.
- [ ] No code in the presenter package hardcodes the `y`/`n`/`e`/`r` choice set — choices are read from the gate value.

**Tests**:
- `"the notes-review gate declares y/n/e/r in order with default y"` — `NotesReviewGate()` keys equal `[y n e r]`, `Default == y`, action labels match the spec.
- `"the reuse confirm declares only y/n with default y"` — `ReuseConfirmGate()` keys equal `[y n]`, `Default == y`, no `e`/`r`.
- `"a gate can declare a non-y default"` — construct a `Gate{Choices: y,n, Default: n}`; `Default` is `n` and `Has(n)` is true (model does not force yes-default).
- `"Has rejects a choice outside the declared set"` — `ReuseConfirmGate().Has("e") == false`; `NotesReviewGate().Has("x") == false`.
- `"Prompt is on the interface and the recorder captures the gate"` — drive `Prompt(NotesReviewGate())` through `RecordingPresenter`; assert the gate argument is captured and the canned choice is returned.

**Edge Cases**:
- **Four-choice notes-review gate** — `y`/`n`/`e`/`r`, default `y`, rendered/returned through the one `Prompt` method.
- **Two-choice reuse confirm** — `y`/`n`, default `y`, no `e`/`r`, same method.
- **Gate declaring a non-`y` default** — the model represents any declared default; the presenter must not assume `y`.
- **Choice outside the declared set** — `Has` returns false; the parsing task (3-3) uses this to reject and re-prompt; `Prompt` never returns a non-member.

**Context**:
> `Prompt(gate)` carries its choice set. A gate is described by the choices it offers; `Prompt` renders whatever choice set the gate declares and returns one of them. This lets one method render every gate variant.
> Notes-review gate (release; regenerate-fresh) — the four-choice `y`/`n`/`e`/`r` `Continue?` menu; default-yes; the engine owns the `e`/`r` re-entry loop.
> Reuse confirm (regenerate reusing existing notes) — a reduced two-choice `y`/`n` confirm rendered in the same `Continue?` vocabulary (no `e`/`r`, since there are no freshly-generated notes to edit or regenerate); default-yes.
> Review gate vertical menu (pretty): `y accept & proceed [default]` / `n abort` / `e edit in $EDITOR` / `r regenerate` then `Continue? ›`.
> Engine owns the four semantic choices; presentation owns how they look. `Prompt(gate)` returns a single `choice` (`y`/`n`/`e`/`r`) and is render-only.
> Phase 1 established the `Presenter` interface and the design note that the fuller vocabulary (including `Prompt`) is added in later phases without churn.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Gating & `-y` Orthogonality" (`Prompt(gate)` carries its choice set; gate inventory), "The Pretty Layer" (Review gate menu).

## cli-presentation-3-2 | approved

### Task cli-presentation-3-2: Stdin-TTY detection independent of stdout render-mode detection

**Problem**: Gating is about *input* (is stdin a TTY?); render mode is about *output* (is stdout a TTY?). The spec makes their independence the backbone of the design: a human with `-y` at a terminal still gets full styling; `--plain` drops styling without touching gating. Phase 1 built stdout-TTY detection for render-mode selection (`SelectMode`/`DetectMode`/`IsTerminal`). Phase 3's gating layer needs a *separate* stdin-TTY signal — used to decide whether an interactive prompt can read a line or whether the forbidden-combination rule fires. Conflating the two (reusing the stdout signal for stdin) would silently break orthogonality: piping stdout would wrongly suppress prompts, and capturing stdin would wrongly affect styling. This task isolates and tests the stdin signal so 3-3/3-5/3-6 can depend on it cleanly.

**Solution**: Add a stdin-TTY detection path that reuses the same OS-reported `IsTerminal` primitive against the **stdin** file descriptor, expose it as a distinct, separately-injectable signal from the stdout render-mode signal, and prove via tests that the two axes are computed independently (all four TTY combinations) with no environment sniffing.

**Outcome**: A tested stdin-TTY signal exists alongside the Phase 1 stdout signal; the four combinations (stdin TTY × stdout TTY, both on, both off, mixed) are each computable independently; and no env var influences either signal.

**Do**:
- Reuse the Phase 1 primitive `IsTerminal(f *os.File) bool` (it already wraps `term.IsTerminal(int(f.Fd()))`) — do **not** add a second TTY mechanism. Apply it to `os.Stdin` for the gating axis.
- Introduce an explicit gating-input signal distinct from render mode. Prefer a small testable pure core mirroring `SelectMode`'s shape, e.g. `func StdinIsInteractive(isStdinTTY bool) bool { return isStdinTTY }` plus startup wiring `func DetectStdinTTY(stdin *os.File) bool { return IsTerminal(stdin) }`. The pure core takes the resolved boolean so it is unit-testable without a device. (If a thin wrapper feels redundant, at minimum document that the gating axis consumes `IsTerminal(os.Stdin)` and is computed separately from `DetectMode(...)` which consumes `IsTerminal(os.Stdout)`.)
- Ensure the gating decision (used by 3-3/3-5/3-6) takes the **stdin** signal, never the stdout/render-mode value. The render-mode value (`Mode`) and the stdin-interactive boolean are two separate inputs threaded through startup; nothing derives one from the other.
- Do **not** read `LANG`/`LC_*`/`TERM`/`CI`/`NO_COLOR` in this path (same no-sniffing ban as Phase 1). Add a code comment stating the stdin axis is orthogonal to the stdout/render-mode axis and that neither sniffs the environment.
- At the startup wiring point, resolve both signals once: `mode := DetectMode(plainFlag, os.Stdout)` and `stdinInteractive := DetectStdinTTY(os.Stdin)`. Document that these are independent and neither re-checks the other's stream.

**Acceptance Criteria**:
- [ ] Stdin-TTY detection uses the same `IsTerminal` primitive applied to the stdin descriptor — no second/duplicate TTY mechanism.
- [ ] The stdin-interactive signal is a distinct value from the stdout-derived `Mode`; the gating path consumes the stdin signal and never the stdout signal.
- [ ] All four combinations are independently representable: stdin non-TTY while stdout TTY; stdout non-TTY while stdin TTY; both non-TTY; both TTY.
- [ ] No environment variable (`LANG`/`LC_*`/`TERM`/`CI`/`NO_COLOR`) is read in the stdin-detection or gating-decision path.
- [ ] Both signals are resolved once at startup; neither is derived from the other.

**Tests**:
- `"stdin non-TTY while stdout TTY keeps render pretty and gating non-interactive"` — `DetectMode(false, ttyStdout) == ModePretty` and `StdinIsInteractive(false) == false`; the two axes disagree, proving independence.
- `"stdout non-TTY while stdin TTY selects plain rendering but interactive gating"` — `SelectMode(false, false) == ModePlain` and `StdinIsInteractive(true) == true`.
- `"both non-TTY"` — `SelectMode(false,false) == ModePlain` and `StdinIsInteractive(false) == false`.
- `"both TTY"` — `SelectMode(false,true) == ModePretty` and `StdinIsInteractive(true) == true`.
- `"environment is not sniffed for the stdin axis"` — `t.Setenv` `LANG`/`LC_ALL`/`TERM=xterm`/`CI=true`/`NO_COLOR=1`, then assert `StdinIsInteractive`/`DetectStdinTTY` are governed only by the stdin TTY boolean and unchanged by env.

**Edge Cases**:
- **stdin non-TTY while stdout TTY** — render mode stays `pretty` (stdout); gating sees a non-interactive stdin (triggers the `-y`-required rule in 3-6) — the two axes diverge.
- **stdout non-TTY while stdin TTY** — render mode is `plain` (stdout); gating is interactive (stdin) — divergence the other way.
- **both non-TTY** — `plain` rendering and non-interactive gating.
- **no environment sniffing** — neither axis consults env vars.

**Context**:
> `-y`/`--yes` is orthogonal to styling and to the output stream. Gating is about input (is stdin a TTY?); render mode is about output (is stdout a TTY?) — checked independently.
> Their independence is the backbone of the design: a human with `-y` at a terminal still gets full styling; `--plain` drops styling without touching gating; the stream split is fixed regardless of mode.
> Detection uses the OS-reported stream type: `term.IsTerminal(int(os.Stdout.Fd()))`. No sniffing of `LANG`/`LC_*`/`TERM`/`CI`. (The same primitive and ban apply to the stdin axis.)
> Phase 1 (cli-presentation-1-3) established `SelectMode`/`DetectMode`/`IsTerminal` for the stdout render-mode axis; this task adds the orthogonal stdin axis using the same primitive.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Scope & Output Modes" (three orthogonal axes), "Gating & `-y` Orthogonality", "Render-Mode Detection & Output Streams" (TTY detection, no sniffing).

## cli-presentation-3-3 | approved

### Task cli-presentation-3-3: Line-read input model — case-insensitive parse, empty-Enter default, re-prompt loop

**Problem**: The gate must read input the safe, simple way: **line-read** (type the letter, press Enter) — not raw single-keypress (no termios raw-mode complexity). The parsing rules guard a destructive-adjacent default: an empty line (just Enter) selects the declared default and accepts; input is case-insensitive (`N` = `n`); and **unrecognised input re-prompts and never silently accepts** — garbage must never proceed. Old muscle-memory keys (`a`, `q`) and any other unrecognised key (`x`) re-prompt rather than mapping to anything. This is the shared input engine both the notes gate and the regenerate source/target prompts (3-7) reuse, so it must be built and tested independently of how the menu looks (3-4) and of `-y` (3-5).

**Solution**: Implement the line-read parse-and-loop core that both presenters use inside `Prompt`: read a line from the injected input reader, trim it, lower-case it, and resolve against the gate's declared choice set — empty → the gate's `Default`; an exact (case-folded) match of a declared key → that choice; anything else → re-prompt (re-render the gate and read again) without ever returning a non-declared choice. Handle EOF as a terminal condition (return an error, do not loop forever). Keep this logic mode-agnostic so plain and pretty share it; only the rendering of the prompt/menu differs.

**Outcome**: Given a gate and an input stream, the input core returns the declared default on empty Enter, the matching choice for any case variant of a declared key, loops (re-prompting) on whitespace-only or unrecognised input until a valid choice or EOF, and returns an error (never a silent accept) on EOF.

**Do**:
- Add an injectable input source to both presenters so stdin is testable: extend the presenter constructors (or add a field) with an `in io.Reader` (default `os.Stdin` at the startup wiring point in `New(...)`). Wrap it in a `bufio.Reader`/`bufio.Scanner` for line reads.
- Implement a shared parse function in the package, e.g. `func parseChoice(line string, g Gate) (Choice, bool)`: trim leading/trailing whitespace; if the result is empty → return `g.Default, true`; otherwise lower-case and compare against each `g.Keys()` (the declared set) — on match return that key, `true`; on no match return `"", false`. It must use `g.Has`/`g.Keys` from 3-1 and must **not** hardcode `y`/`n`/`e`/`r`.
- Implement the read-and-loop core used by `Prompt`, e.g. `func (p *PlainPresenter) readChoice(g Gate) (Choice, error)` (and the pretty equivalent): render the prompt (delegated to the mode's renderer — plain prompt line in this task's plain path; pretty menu lands in 3-4), read one line; on a successful `parseChoice` return the choice; on failure re-render the prompt and read again (the re-prompt loop); on EOF (reader returns `io.EOF` with no line) return a non-nil error and **do not** default-accept.
- Empty Enter selects the default **only** on a deliberate empty line — a whitespace-only line trims to empty and is therefore treated as empty-Enter (selects default). Document this explicitly (the spec says "the default fires only on a deliberate empty Enter"; a line of only spaces is treated as empty). If the implementer instead chooses to re-prompt on whitespace-only, document and test that choice — but the spec's "deliberate empty Enter = default" most naturally maps whitespace-trimmed-empty to the default; pick one, document it, and lock it with a test. (Recommended: trim-to-empty ⇒ default, matching ordinary CLI line-read behaviour.)
- Unrecognised input (`x`, `a`, `q`, or any non-declared key) must re-prompt — never return and never map to a declared choice. Repeated unrecognised lines keep re-prompting until a valid line or EOF.
- For this task, the plain `Prompt` may render a minimal prompt line (e.g. `{Prompt} [y/n/e/r] ` or similar terse form) to drive the loop; the full pretty vertical menu is 3-4. Keep the parse/loop core identical across modes.
- EOF handling: returning an error (rather than blocking or silently accepting) is what the forbidden-combination task (3-6) and unattended-without-`-y` scenarios rely on — an interactive prompt whose stdin closes must not silently accept the destructive-adjacent default.

**Acceptance Criteria**:
- [ ] An empty Enter (blank line) returns the gate's declared `Default` (e.g. `y` for the notes gate).
- [ ] Input is case-insensitive: `N`/`n` → `n`, `Y`/`y` → `y`, `E` → `e`, `R` → `r` (only for keys the gate declares).
- [ ] Unrecognised input (`x`, `a`, `q`, any non-declared key) re-prompts and never returns a non-declared choice and never silently accepts the default.
- [ ] A whitespace-only line resolves per the documented rule (recommended: treated as empty-Enter ⇒ default) — behaviour is documented and tested.
- [ ] Repeated unrecognised lines keep re-prompting; a subsequent valid line is then accepted and returned.
- [ ] EOF on stdin returns a non-nil error (no infinite loop, no silent default-accept).
- [ ] The parse/loop core is mode-agnostic and uses the gate's declared set (`Has`/`Keys`) — no hardcoded `y/n/e/r`.

**Tests**:
- `"empty Enter selects the declared default"` — input `"\n"` against `NotesReviewGate()` returns `y`.
- `"uppercase N maps to n"` — input `"N\n"` returns `n`; `"E\n"` returns `e`.
- `"unrecognised x re-prompts then accepts a valid choice"` — input `"x\nn\n"` returns `n` after one re-prompt (assert the prompt was rendered twice).
- `"old muscle-memory a and q re-prompt and never accept"` — input `"a\nq\ny\n"` returns `y`; `a`/`q` never returned.
- `"a whitespace-only line is treated as empty-Enter (default)"` — input `"   \n"` returns the default `y` (per the documented rule).
- `"repeated unrecognised then valid"` — input `"x\nz\n?\nr\n"` returns `r` after three re-prompts.
- `"EOF on stdin returns an error and does not default-accept"` — input that reaches EOF with no valid line (e.g. `""` or `"x"` then EOF) returns a non-nil error and not the default.

**Edge Cases**:
- **Empty Enter selects default** — a deliberate blank line accepts the destructive-adjacent default.
- **Uppercase `N` maps to `n`** — case-insensitive matching across all declared keys.
- **Unrecognised `x`/`a`/`q` re-prompts (never accepts)** — garbage never proceeds; old keys do not map.
- **Whitespace-only line** — documented handling (recommended: treated as empty ⇒ default).
- **Repeated unrecognised then valid choice** — the loop persists through multiple bad lines and then accepts the first valid one.
- **EOF on stdin** — returns an error rather than blocking or silently accepting; this underpins the fail-loud behaviour in 3-6.

**Context**:
> Line-read (type the letter, press Enter) — not raw single-keypress; no termios raw-mode complexity.
> Empty line (just Enter) = default = accept. The default fires only on a deliberate empty Enter.
> Case-insensitive (`N` = `n`).
> Unrecognised key (`x`, or old muscle-memory `a`/`q`) → re-prompt, never silently accept. Garbage never proceeds — keeps the destructive-adjacent default safe.
> Regenerate source/target prompts use the same line-read input model as `Continue?` (type the value, press Enter; case-insensitive; unrecognised input re-prompts, never silently proceeds) — so this core is shared (consumed by 3-7).
> The reconciliation note: the default-yes `Continue?` gate (`y`/`n`/`e`/`r`, Enter ⇒ accept) supersedes the engine's stale `[a]`/`[q]` keys — `a` and `q` are therefore unrecognised and must re-prompt.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Gating & `-y` Orthogonality" (Gate input handling), "Dependencies → Notes" (review-gate reconciliation dropping `[a]`/`[q]`).

## cli-presentation-3-4 | approved

### Task cli-presentation-3-4: Pretty Prompt vertical-menu rendering — options above question, [default] beside action, prompt last

**Problem**: In `pretty` mode the review gate is a styled vertical menu with a fixed shape: the options listed **above** the question, the `[default]` marker beside the default action, and the `Continue? ›` prompt **last**. This shape is the human affordance for the gate — and it must render directly from the gate's declared choice set so the four-choice notes gate and the two-choice reuse confirm both render correctly from the same code (the two-choice confirm shows only `y`/`n`, no `e`/`r`). After unrecognised input, the menu must be redrawn (linear re-render) before reading again, so the user sees the options again rather than a bare reprompt.

**Solution**: Implement the pretty `Prompt` rendering: for the given `Gate`, render each declared choice as an indented `  {key}  {action}` line in declared order, append ` [default]` to the line whose key equals `gate.Default`, then a blank line, then the `  {Prompt} › ` prompt line; wire this renderer into the 3-3 read-and-loop core so it is redrawn on each pass (including after bad input). Styling goes through the injected `lipgloss` renderer with colour auto-downgrade preserved from Phase 1.

**Outcome**: Given a gate, `PrettyPresenter.Prompt` renders the options above the question with `[default]` beside the default action and the `Continue? ›` prompt last; the two-choice confirm renders only `y`/`n`; and after unrecognised input the full menu is redrawn before the next read.

**Do**:
- In `internal/presenter/pretty.go`, implement the prompt rendering used by the read-and-loop core (from 3-3): a method/helper, e.g. `renderGate(g Gate)`, that writes to `out`:
  - One line per `GateChoice` in declared order: two-space indent, the key, padded gap, the action label — mirroring the worked example `    y  accept & proceed`. Append ` [default]` to the choice whose `Key == g.Default` (worked example: `y  accept & proceed [default]`).
  - A blank line after the option list.
  - The prompt line last: `  {g.Prompt} › ` (worked example `  Continue? › `). Do not print a newline after the `› ` (the cursor sits after the prompt for the line-read).
- Drive this from the loop: render the menu, read a line (3-3 core), and on unrecognised input **re-render** the full menu (linear — it scrolls; no screen-clearing, no alt-screen) and read again. On empty Enter the default fires; on a valid key return it.
- The menu must be built **from the gate's declared choices** — the two-choice reuse confirm renders exactly two lines (`y`/`n`) with no `e`/`r`; the four-choice notes gate renders four. No hardcoded option list.
- Style via the injected `*lipgloss.Renderer` (colour for keys/markers as desired); rely on lipgloss colour auto-downgrade (no `NO_COLOR`/`TERM` check). Under downgrade, the menu structure (lines, `[default]` marker, `›` prompt) must survive without colour codes.
- This task is `pretty`-only. The plain prompt rendering remains the terse form established in 3-3 (a `key: value`-style prompt line); do not draw the vertical menu in plain.
- Scope: do **not** implement `-y` skip (3-5), the forbidden-combination failure (3-6), or any spinner stop/resume around `$EDITOR` (Phase 4). This task only renders the interactive pretty menu and its redraw-on-bad-input.

**Acceptance Criteria**:
- [ ] Pretty renders the options **above** the question and the `{Prompt} › ` line **last**, with a blank line between.
- [ ] The `[default]` marker appears beside the action whose key equals `gate.Default` (and only that one).
- [ ] A four-choice notes gate renders four option lines (`y`/`n`/`e`/`r`); a two-choice reuse confirm renders only `y`/`n` (no `e`/`r`).
- [ ] After unrecognised input the full menu is redrawn (options + prompt) before the next read — verified by counting renders.
- [ ] The menu is built from the gate's declared choices (no hardcoded option list); reordering/changing the gate changes the rendered menu.
- [ ] Under colour downgrade, the menu structure and `[default]`/`›` markers are present with no ANSI colour codes.

**Tests**:
- `"the four-choice gate renders options above the prompt with the prompt last"` — `Prompt(NotesReviewGate())` (scripted input `y`) renders `y/n/e/r` lines then a blank line then `Continue? › `, in that order.
- `"the [default] marker sits beside the default action only"` — assert ` [default]` on the `y` line and on no other line for the default-yes gate.
- `"the two-choice confirm renders only y/n"` — `Prompt(ReuseConfirmGate())` renders exactly two option lines and no `e`/`r`.
- `"the menu is redrawn after bad input"` — scripted input `"x\ny\n"`; assert the option block + prompt are rendered twice (once before the bad line, once after).
- `"a non-y default marks the declared default line"` — gate with `Default:n` renders ` [default]` on the `n` line, not the `y` line.
- `"colour downgrade preserves the menu structure"` — force no-colour profile; assert option lines, `[default]`, and `›` present with no SGR colour codes.

**Edge Cases**:
- **Two-choice confirm renders only `y`/`n`** — no `e`/`r` lines for the reuse gate; driven by the declared set.
- **`[default]` marker on declared default action** — only the `gate.Default` line carries the marker (and a non-`y` default marks its own line).
- **Prompt redrawn after bad input** — the full menu re-renders before each subsequent read (linear scroll, no clear).

**Context**:
> Review gate — vertical menu, options above the question, `[default]` next to its action, prompt last:
> ```
>     y  accept & proceed [default]
>     n  abort
>     e  edit in $EDITOR
>     r  regenerate
>
>   Continue? › 
> ```
> Enter ⇒ `y` (accept & proceed — the 99% path). `n` ⇒ abort; `e` ⇒ `$EDITOR`; `r` ⇒ regenerate-with-context.
> Reuse confirm — a reduced two-choice `y`/`n` confirm rendered in the same `Continue?` vocabulary (no `e`/`r`); default-yes.
> Rendering is linear — each pass re-prints the notes block + gate below (it scrolls; no screen-clearing or alt-screen). (After bad input the menu redraws in place in the scroll, not via clearing.)
> `lipgloss` for all `pretty`-mode styling; it auto-downgrades colour when piped or on colour-incapable TTYs (Phase 1 established the injectable renderer).

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "The Pretty Layer" (Review gate — vertical menu), "Gating & `-y` Orthogonality" (reuse confirm; linear re-render), "Library Selection".

## cli-presentation-3-5 | approved

### Task cli-presentation-3-5: `-y` skip with rendered auto-accept echo in both modes

**Problem**: Under `-y`/`--yes` the gate is **skipped** — not drawn-and-auto-pressed. The outcome is identical to pressing Enter (accept), but the interactive menu is **not** shown and stdin is **not** read; the auto-accept is communicated via a *rendered event* so it appears in a captured log (preserving "narration → presenter," not engine-printed text). Plain renders `notes: accepted (-y)`; pretty renders a concise accept line in the run vocabulary (e.g. `✓ notes  accepted (-y)`). This applies to both the four-choice notes gate and the two-choice reuse confirm. Getting this wrong (drawing the menu then auto-answering, or reading stdin) would contradict the spec and break the orthogonality story ("full styling under `-y`" means the rest of the run is styled, not that the menu is shown then auto-answered).

**Solution**: Thread a gating decision into `Prompt` so that when `-y` was passed the method **returns the gate's declared default without rendering the menu or reading stdin**, and instead emits a rendered auto-accept echo — plain `notes: accepted (-y)`, pretty a concise `✓ {prompt-subject}  accepted (-y)` line. The `-y` decision is the gating axis (orthogonal to render mode and stream split); pass it in at construction or as part of the prompt call, not re-derived.

**Outcome**: With `-y` active, `Prompt(gate)` returns the gate's default (`y`) without drawing the menu and without reading from the input reader; plain emits `notes: accepted (-y)` and pretty emits a concise accept line; this holds for both the notes gate and the reuse confirm.

**Do**:
- Make the `-y`/yes decision available to `Prompt`. Recommended: thread it through construction (e.g. a `yes bool` field set in `New(mode, out, err, in, opts...)` from the `-y` flag) so the presenter knows the gate is to be skipped; alternatively pass it on the call. Document the chosen plumbing. This is the **gating axis** — it is independent of `Mode` (render) and of the stream split.
- In both presenters' `Prompt`, **first** check the yes/skip decision. When skipping:
  - Do **not** render the menu/prompt and do **not** read from the input reader at all.
  - Emit the rendered auto-accept echo:
    - Plain (`plain.go`): `{subject}: accepted (-y)` to `out` — for the notes gate `notes: accepted (-y)` (worked example). The `{subject}` derives from the gate (e.g. a gate label/subject field, or `notes` for the notes gate / an analogous subject for the reuse confirm) — render the spec's `notes: accepted (-y)` for the notes gate and an analogous echo for the reuse confirm.
    - Pretty (`pretty.go`): a concise accept line in the same vocabulary, e.g. `  ✓ {subject}  accepted (-y)` (worked spec wording `✓ notes  accepted (-y)`), styled via the lipgloss renderer.
  - Return the gate's declared `Default` (`y`) as the choice.
- The auto-accept echo is narration → `out` only (it is not an error/warning; no stderr copy).
- To carry the echo subject without the presenter re-deriving engine knowledge, add a subject/label to the `Gate` model if not already present (e.g. `Gate.Subject string`), set by each constructor: `notes` for `NotesReviewGate()` and `notes` for `ReuseConfirmGate()` (the reuse confirm is also a notes-acceptance gate rendered in the same `Continue?` vocabulary, so its echo is `notes: accepted (-y)` — the source/target gates in cli-presentation-3-7 set `source`/`target`). The presenter renders `{subject}: accepted (-y)` from the payload rather than hardcoding `notes`. Document this addition (it extends 3-1's model). (The exact subject word is refinable, but each constructor must set a concrete value so the `-y` echo line is deterministic and testable.)
- Scope: this task covers the gate-skip + echo for the notes gate and reuse confirm. The regenerate source/target `-y` echoes are 3-7 (which reuses this same skip mechanism). The forbidden-combination (non-TTY stdin **without** `-y`) is 3-6 — here `-y` is present, so no failure.

**Acceptance Criteria**:
- [ ] Under `-y`, `Prompt` does **not** render the interactive menu/prompt and does **not** read from the input reader.
- [ ] Under `-y`, `Prompt` returns the gate's declared default (`y`).
- [ ] Plain emits `notes: accepted (-y)` for the notes gate (and an analogous `{subject}: accepted (-y)` for the reuse confirm) to stdout.
- [ ] Pretty emits a concise accept line in the run vocabulary (e.g. `✓ notes  accepted (-y)`) to stdout.
- [ ] The reuse confirm is auto-accepted under `-y` exactly like the notes gate (with its analogous echo).
- [ ] The auto-accept echo is on stdout only (not stderr).

**Tests**:
- `"the menu is not drawn under -y"` — pretty `Prompt(NotesReviewGate())` with `-y` active: assert no `Continue? ›` prompt and no option lines are written.
- `"-y returns the default without reading stdin"` — provide an input reader that records reads (or a reader that would error if read); assert it is never read and `Prompt` returns `y`.
- `"plain renders notes: accepted (-y)"` — plain `Prompt(NotesReviewGate())` under `-y` writes exactly `notes: accepted (-y)`.
- `"pretty renders a concise accept line"` — pretty under `-y` writes `✓ notes  accepted (-y)` (accept line in the run vocabulary).
- `"the reuse confirm is auto-accepted under -y"` — `Prompt(ReuseConfirmGate())` under `-y` returns `y`, draws no menu, and emits the analogous echo.
- `"the auto-accept echo is stdout only"` — assert stderr is empty after a `-y` `Prompt`.

**Edge Cases**:
- **Menu not drawn under `-y`** — the interactive options/prompt are never rendered; the gate is skipped, not auto-pressed.
- **Plain `notes: accepted (-y)`** — the exact plain echo line.
- **Pretty concise accept line** — `✓ notes  accepted (-y)` in the run vocabulary.
- **Reuse-confirm auto-accepted** — the two-choice confirm is skipped and echoed under `-y` just like the notes gate.
- **Returns default without reading stdin** — stdin is never touched under `-y` (decouples accept from any input availability).

**Context**:
> Pretty under `-y`: `-y` skips the gate rather than auto-pressing it — identical outcome to pressing Enter, but the menu is not drawn (consistent with "the gate is skipped"). "Full styling under `-y`" means the rest of the run is styled; it does not mean the interactive menu is shown then auto-answered. The auto-accept is rendered via the gate auto-accept event (pretty: a concise accept line in the same vocabulary, e.g. `✓ notes  accepted (-y)`; plain: `notes: accepted (-y)`).
> Gate auto-accept under `-y` is a rendered event, not engine-printed text. When `-y` skips the gate, the engine emits an event the presenter renders (plain: `notes: accepted (-y)`; pretty: see the Gating section), preserving "narration → presenter."
> Plain `-y` echo — when the gate is skipped under `-y`, emit `notes: accepted (-y)` so the auto-accept is visible in the captured log.
> Reuse confirm: Plain skips it under `-y` exactly like the notes gate, with an analogous auto-accept echo.
> Worked plain run (`-y`) shows `notes: accepted (-y)` between the notes block and the record stage.
> `-y` is the gating axis, orthogonal to styling and the stream split.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Gating & `-y` Orthogonality" (Pretty under `-y`; reuse confirm), "The `Presenter` Seam" (Gate auto-accept under `-y` is a rendered event), "The Plain Layer" (`-y` echo; worked plain run).

## cli-presentation-3-6 | approved

### Task cli-presentation-3-6: Forbidden-combination fail-loud (non-TTY stdin without `-y`) surfaced through Presenter and to stderr

**Problem**: If **stdin is not a TTY** and **`-y` was not passed**, `mint` must **fail loud** ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin reading a line that will never come. This is the safety rule for any interactive gate. The failure must **surface through the `Presenter`** as a rendered failure (styled in `pretty`, terse in `plain`) — because render mode is selected on stdout *independently* of the stdin problem, the run can still be styled while it reports the stdin failure — and, per the stream contract, the one-line failure also goes to **stderr**. This depends on the orthogonal stdin signal (3-2): the render mode is chosen from stdout while the gate-failure is driven by stdin.

**Solution**: In `Prompt`, before rendering the menu or reading input, check the gating preconditions: if `-y` is **not** set and stdin is **not** interactive, do not draw the menu or read stdin — instead render a failure through the presenter (pretty styled, plain terse) to `out`, write the one-line failure summary to `err`, and return the forbidden-combination error from `Prompt` (the engine owns the non-zero exit). Render mode is whatever was selected from stdout; this failure does not change it.

**Outcome**: When `-y` is absent and stdin is non-TTY, `Prompt` does not block on stdin; it renders a "not a TTY — pass `-y` to run unattended" failure (styled in pretty, terse in plain) to stdout, writes the one-line summary to stderr, returns an error, and the render mode chosen on stdout is unaffected.

**Do**:
- Give `Prompt` the gating preconditions it needs: the `-y`/yes decision (from 3-5's plumbing) and the stdin-interactive boolean (from 3-2). Thread the stdin-interactive flag into the presenter the same way as `yes` (construction field set at startup wiring), so `Prompt` can check it without re-detecting.
- In both presenters' `Prompt`, evaluate the precedence:
  1. If `yes` is set → skip + auto-accept (3-5).
  2. Else if stdin is **not** interactive → **forbidden combination**: render the failure (do not draw the menu, do not read stdin) and return the error.
  3. Else → interactive line-read loop (3-3/3-4).
- Render the failure through the presenter:
  - Plain (`plain.go`): a terse failure line, e.g. `gate: FAILED - not a TTY; pass -y to run unattended` (follow the established plain `FAILED` vocabulary from Phase 2; keep the message the spec's "not a TTY — pass `-y` to run unattended"). Write it to `out`.
  - Pretty (`pretty.go`): a styled `✗`-glyph failure line in the run vocabulary, e.g. `✗ gate  not a TTY — pass -y to run unattended`, via the lipgloss renderer. Write it to `out`.
  - In **both** modes also write the **one-line** failure summary to `err` (per the Phase 1/2 stream contract: the one-line summary is duplicated to stderr).
- Return a non-nil error from `Prompt` (e.g. a sentinel `ErrNotInteractive`) so the engine can map it to a non-zero exit. The presenter **does not** set the exit code (exit-code ownership is the engine's).
- Do **not** read from the input reader at all on this path — the whole point is to avoid blocking on stdin that will never deliver a line. (This is distinct from the EOF case in 3-3, which is the fallback if a read somehow occurs; here we never read.)
- Confirm render mode is untouched: the `Mode` was selected from stdout (Phase 1) and is whatever it is — pretty if stdout is a TTY even though stdin is not. The failure is rendered in that mode. Add a test asserting a pretty-styled failure when stdout is a TTY but stdin is not.

**Acceptance Criteria**:
- [ ] When `-y` is absent and stdin is non-interactive, `Prompt` does **not** read stdin and does **not** draw the menu — it fails immediately (no blocking).
- [ ] Pretty renders a styled `✗` failure line ("not a TTY — pass `-y` to run unattended"); plain renders a terse failure line — both to stdout.
- [ ] The one-line failure summary is also written to stderr in both modes.
- [ ] `Prompt` returns a non-nil error on this path; the presenter sets no exit code.
- [ ] The render mode is still chosen on stdout independently — a non-TTY stdin with a TTY stdout still renders the failure in `pretty`.
- [ ] The precedence is correct: `-y` present → auto-accept (no failure); `-y` absent + non-TTY stdin → fail; `-y` absent + TTY stdin → interactive.

**Tests**:
- `"non-TTY stdin without -y fails without blocking on stdin"` — `yes=false`, `stdinInteractive=false`; provide a reader that would block/error if read; assert `Prompt` returns the error without reading.
- `"pretty renders a styled failure"` — pretty mode, `yes=false`, non-TTY stdin: assert a `✗` failure line with the "pass -y" message on stdout.
- `"plain renders a terse failure"` — plain mode, same preconditions: assert a terse `FAILED`-style line with the message on stdout.
- `"the failure summary also goes to stderr"` — assert the one-line summary present in the stderr buffer in both modes.
- `"render mode is still chosen on stdout"` — stdout TTY (`ModePretty`) but stdin non-TTY: assert the failure renders in pretty (styled), proving the axes are independent.
- `"-y present does not trigger the forbidden combination"` — `yes=true`, non-TTY stdin: assert auto-accept (3-5), no failure.

**Edge Cases**:
- **Non-TTY stdin without `-y` fails (no stdin block)** — fails immediately without attempting a blocking read.
- **Styled failure in pretty** — the failure carries the `✗` glyph and styling when render mode is pretty.
- **Terse failure in plain** — the failure is a terse `FAILED`-style line in plain.
- **Also to stderr** — the one-line summary is duplicated to stderr per the stream contract.
- **Render mode still chosen on stdout** — a TTY stdout yields a pretty-styled failure even though stdin is the non-TTY that caused it (orthogonality).

**Context**:
> Forbidden-combination rule (applies to any interactive gate): if stdin is not a TTY and `-y` was not passed, `mint` fails loud ("not a TTY — pass `-y` to run unattended") rather than blocking on stdin. `-y` answers every gate. This failure surfaces through the `Presenter` (rendered as a failure — styled in `pretty`, terse in `plain` — since render mode is selected on stdout independently of the stdin problem) and, per the stream contract, also goes to stderr.
> Per-event rendering: review gate (plain) → (not shown — non-TTY ⇒ `-y` required ⇒ gate skipped; emits `notes: accepted (-y)`). (When `-y` is absent, the non-TTY case is the fail-loud path of this task.)
> `StageFailed` one-line summary additionally goes to stderr (the captured body does not) — the same one-line-to-stderr rule applies to this failure.
> Exit-code ownership stays with the engine/`main`, not the `Presenter`.
> Depends on the orthogonal stdin signal (3-2) and the `-y` plumbing (3-5).

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Gating & `-y` Orthogonality" (Forbidden-combination rule), "Render-Mode Detection & Output Streams" (Output streams; exit-code ownership), "The Plain Layer" (Per-event rendering, review gate row).

## cli-presentation-3-7 | approved

### Task cli-presentation-3-7: Regenerate source/target prompts reuse the line-read model, skip under `-y`, obey forbidden-combination

**Problem**: Before the notes/confirm gate, `regenerate` has interactive **source** and **target** selection prompts. The spec requires they render through the `Presenter` using the **same line-read input model** as `Continue?` (type the value, press Enter; case-insensitive; unrecognised input re-prompts, never silently proceeds), with plain rendering them as terse `key: value` prompt lines; under `-y` they are **skipped using the provided flags/defaults** with an auto-accept echo in the same vocabulary; and the forbidden-combination rule (non-TTY stdin without `-y` fails loud) applies to them as to any gate. This proves the gate machinery (3-1…3-6) generalises beyond the binary/choice gate to value-selection prompts — reusing the same input core, skip mechanism, and fail-loud, not a parallel implementation.

**Solution**: Express the source and target selection prompts as gates (reusing the 3-1 `Gate` model with their respective declared option sets and a declared default) so they flow through the same `Prompt` method — the same line-read loop (3-3), the same `-y` skip + echo (3-5), and the same forbidden-combination fail-loud (3-6). Plain renders them as terse `key: value` prompt lines; the `-y` echo shows the chosen source/target so they appear in a captured log.

**Outcome**: The `regenerate` source and target prompts render via `Prompt` using the shared line-read model (case-insensitive, unrecognised re-prompts), plain renders terse `key: value` lines, `-y` skips them using the provided flag/default and echoes the chosen value, and non-TTY stdin without `-y` fails loud — all reusing the gate machinery rather than a separate code path.

**Do**:
- Model the source and target prompts as gates using the 3-1 `Gate` type, treating source/target as an **enumerated declared choice set** (the engine supplies the available sources/targets as `GateChoice` keys, exactly like the y/n/e/r gate), so the shared 3-3 line-read/exact-match-against-declared-keys core applies unchanged — there is no free-form value entry on this path. Provide constructors, e.g. `func SourceGate(options []GateChoice, def Choice) Gate` and `func TargetGate(options []GateChoice, def Choice) Gate`, with the gate `Prompt` text being the source/target question (e.g. `Source?` / `Target?`) and a `Subject` (3-5) of `source` / `target` for the `-y` echo. The declared options and default come from the engine (the available sources/targets and the flag/default) — the presenter does not invent them. (If the engine ever needs free-form value entry rather than a fixed option set, that is a separate parse variant and out of scope for this task.)
- Reuse `Prompt(gate)` end-to-end — do **not** write a second input loop. The same precedence applies: `-y` → skip + echo; non-TTY stdin without `-y` → fail loud; else interactive line-read loop with re-prompt on unrecognised input.
- Plain rendering of the interactive prompt is a terse `key: value` prompt line in the same vocabulary (e.g. `source: ` / `target: ` as the prompt, listing the options tersely as needed). Keep it consistent with the plain prompt form from 3-3; do not draw the pretty vertical menu in plain.
- `-y` skip echo (reusing 3-5): emit `source: {chosen} (-y)` / `target: {chosen} (-y)` (plain) and the concise pretty accept line (`✓ source  {chosen} (-y)` / `✓ target  {chosen} (-y)`) so the chosen source/target is visible in a captured log. The chosen value under `-y` is the gate's declared default (which the engine sets from the provided flag/default).
- Forbidden-combination (reusing 3-6): non-TTY stdin without `-y` fails loud through the presenter (styled pretty / terse plain) and to stderr, returning the error — exactly as for the notes/confirm gate.
- Interactive parsing (reusing 3-3): unrecognised input re-prompts and never silently proceeds; case-insensitive matching against the declared option set; empty Enter selects the declared default.
- Do not duplicate the input/skip/fail logic — this task is primarily wiring the source/target prompts as gates and asserting the shared behaviour holds for value-selection, plus the plain `key: value` rendering and the `-y` value echo.

**Acceptance Criteria**:
- [ ] Source and target prompts render through the shared `Prompt(gate)` method (no second input loop).
- [ ] Interactive input uses the shared line-read model: case-insensitive, empty Enter → declared default, unrecognised input re-prompts and never silently proceeds.
- [ ] Plain renders the prompts as terse `key: value` prompt lines (consistent with the plain prompt vocabulary).
- [ ] Under `-y`, the prompts are skipped using the provided flag/default and the chosen source/target is echoed (`source: {chosen} (-y)` / `target: {chosen} (-y)` plain; concise pretty accept line) so it is visible in a captured log.
- [ ] Non-TTY stdin without `-y` fails loud for the source/target prompts (through the presenter and to stderr), reusing the forbidden-combination path.
- [ ] When skipped under `-y`, the flag/default value is used (the engine-provided default), not an interactive read.

**Tests**:
- `"unrecognised source input re-prompts then accepts a valid value"` — scripted input `"bogus\n<valid>\n"` returns the valid source after one re-prompt.
- `"plain renders terse key: value prompt lines for source and target"` — assert the plain prompt lines are terse `source:`/`target:`-style lines (no pretty menu in plain).
- `"-y echoes the chosen source and target"` — `yes=true`: assert `source: {default} (-y)` and `target: {default} (-y)` (plain) / concise pretty accept lines, returning the declared defaults without reading stdin.
- `"non-TTY stdin without -y fails loud for source/target"` — `yes=false`, stdin non-TTY: assert the fail-loud failure (stdout styled/terse + stderr summary + error) for the source gate.
- `"the flag/default is used when skipped under -y"` — set the gate default from a provided flag value; assert `Prompt` returns that value under `-y`.
- `"case-insensitive and empty-Enter default apply to source/target"` — uppercase valid input maps to its lowercase choice; empty Enter returns the declared default.

**Edge Cases**:
- **Unrecognised input re-prompts** — value-selection re-prompts on unrecognised input, never silently proceeds (shared with 3-3).
- **Plain terse `key: value` lines** — source/target prompts render terse in plain, same vocabulary.
- **`-y` echoes chosen source/target** — the chosen values appear in the captured log via the auto-accept echo.
- **Non-TTY without `-y` fails loud** — the forbidden-combination rule applies to source/target prompts (shared with 3-6).
- **Flag/default used when skipped** — under `-y` the provided flag/default is the chosen value, not an interactive read.

**Context**:
> Regenerate source/target prompts. Before the notes/confirm gate, `regenerate` has interactive source and target selection prompts. They render through the `Presenter` using the same line-read input model as `Continue?` (type the value, press Enter; case-insensitive; unrecognised input re-prompts, never silently proceeds), with plain rendering them as terse prompt lines in the same `key: value` vocabulary. Under `-y` they are skipped using the provided flags/defaults, with an auto-accept echo in the same vocabulary as the notes gate (so the chosen source/target are visible in a captured log). The forbidden-combination rule applies to them as to any interactive gate: non-TTY stdin without `-y` fails loud.
> Gate inventory: `regenerate` — interactive source + target prompts, then the notes-review gate (fresh) / a simple confirm (reuse). Under `-y`: uses flags/defaults, auto-accepts.
> This task reuses 3-1 (`Gate` model), 3-3 (line-read loop), 3-5 (`-y` skip + echo), and 3-6 (forbidden-combination) — it is wiring + the plain `key: value`/value-echo rendering, not a parallel implementation.

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Gating & `-y` Orthogonality" (Regenerate source/target prompts; gate inventory), "Cross-Verb Rendering" (`regenerate`).

## cli-presentation-3-8 | approved

### Task cli-presentation-3-8: Render-only Prompt contract — engine owns the `e`/`r` re-entry loop, linear re-render

**Problem**: `Prompt(gate)` returns a single `choice` and is **render-only**: on `e`/`r` the **engine** does the work — invoking `$EDITOR` (edit) or re-running generation via `claude` (regenerate-with-context) — then re-calls `ShowNotes` with the refreshed body and `Prompt` again, looping until `y`/`n`. The presenter **never** calls `$EDITOR` or `claude`; it only re-renders on each pass. Rendering is **linear** — each pass re-prints the notes block + gate below (it scrolls; no screen-clearing, no alt-screen). This contract is what keeps the presentation seam clean (the presenter has no engine side effects) and must be locked with tests: `Prompt` returning `e`/`r` produces *no* presenter-side invocation of editor/regeneration, and repeated `ShowNotes`+`Prompt` passes scroll without clearing.

**Solution**: Lock and verify the render-only contract: `Prompt` returns `e`/`r` as plain choices with **no** presenter side effect (no `$EDITOR`, no `claude`, no engine call), and a driver that simulates the engine's loop (ShowNotes → Prompt → on `e`/`r` re-call ShowNotes+Prompt → until `y`/`n`) renders linearly — each pass appends to the output (scrolls) with no screen-clear/alt-screen control sequences. This is primarily a contract-and-test task plus any guard needed to ensure the presenter exposes no editor/regeneration hook.

**Outcome**: `Prompt` returns `e` or `r` as a choice with no editor/regeneration side effect in the presenter; an engine-style loop re-calling `ShowNotes`+`Prompt` across passes produces linear, scrolling output (no clear-screen or alt-screen sequences); and the loop terminates when the choice is `y`/`n`.

**Do**:
- Audit/confirm the `Prompt` implementations from 3-3…3-6: when the user types `e` or `r` (declared choices of the notes gate), `Prompt` simply **returns** that `Choice` and does nothing else — it must not import `os/exec`, must not reference `$EDITOR`, and must not call any generation/claude code. Add a guard test that the presenter package does not invoke an editor or regeneration (e.g. assert no `os/exec` usage in the prompt path, or that `Prompt` has no dependency that runs subprocesses).
- Add a doc comment on `Prompt` (extending 3-1's) stating the render-only contract explicitly: the engine owns `e`/`r` re-entry; the presenter only renders and returns a choice.
- Provide a test driver that simulates the engine's re-entry loop against a presenter (both modes), to verify the linear-render contract:
  - Loop: call `ShowNotes(refreshedBody)`; call `Prompt(NotesReviewGate())`; if the choice is `e` or `r`, simulate the engine refreshing the body and loop again; stop on `y`/`n`.
  - Script the choices (via the input reader for the real presenters) as `e`, then `r`, then `y` to exercise multiple passes ending on `y`.
- Assert **linear re-render**: the output across passes is cumulative/append-only — each pass re-prints the notes block + gate below the previous one. Assert there are **no** screen-clearing or alt-screen control sequences in the output (no `ESC[2J` clear-screen, no `ESC[H` cursor-home reset used to overwrite, no `ESC[?1049h`/`l` alt-screen enter/leave). The output scrolls.
- Assert the loop **ends on `y`/`n`**: when the scripted choice is `y` (or `n`), the driver exits the loop and `Prompt` returns that choice; `e`/`r` continue the loop.
- Scope: do **not** implement the pretty spinner stop/resume around `$EDITOR` — that is explicitly deferred to Phase 4. This task covers only the render-only contract and the linear re-render. (The engine actually invoking `$EDITOR`/`claude` is engine-spec scope; here the test driver *simulates* the engine to exercise the presenter contract.)

**Acceptance Criteria**:
- [ ] `Prompt` returning `e` produces no presenter side effect — no `$EDITOR` invocation, no subprocess, no generation call.
- [ ] `Prompt` returning `r` produces no presenter side effect — no `claude`/regeneration invocation.
- [ ] A simulated engine loop (ShowNotes → Prompt, re-entering on `e`/`r`) renders linearly: output is append-only/cumulative across passes.
- [ ] The output contains no screen-clearing or alt-screen control sequences (no `ESC[2J`, no alt-screen enter/leave, no cursor-home overwrite).
- [ ] The loop ends when `Prompt` returns `y` or `n`; `e`/`r` continue it.
- [ ] The render-only contract is documented on `Prompt` and guarded by a test (no editor/regeneration dependency in the prompt path).
- [ ] The pretty spinner stop/resume around `$EDITOR` is **not** implemented here (deferred to Phase 4).

**Tests**:
- `"Prompt returning e has no presenter side effect"` — scripted `e`; assert `Prompt` returns `e` and that no editor/subprocess was invoked (guard/spy).
- `"Prompt returning r has no presenter side effect"` — scripted `r`; assert `Prompt` returns `r` and no regeneration/claude call occurred.
- `"repeated ShowNotes+Prompt passes scroll without clearing"` — drive the simulated loop with `e`,`r`,`y`; assert each pass's notes block + gate appears in order in the cumulative output and there are no clear-screen/alt-screen sequences.
- `"the loop ends on y"` — scripted `e`,`y`: the driver loops once on `e`, then exits on `y`; `Prompt`'s final return is `y`.
- `"the loop ends on n"` — scripted `n`: the driver exits immediately with `n`.
- `"the prompt path has no editor/regeneration dependency"` — guard test asserting the presenter does not reach `os/exec`/`$EDITOR`/claude in the prompt path.

**Edge Cases**:
- **`e` returns no presenter side effect** — `Prompt` returns `e` and nothing else; the engine (simulated) owns the editor invocation.
- **`r` returns no presenter side effect** — `Prompt` returns `r` and nothing else; the engine owns regeneration.
- **Repeated ShowNotes+Prompt passes scroll** — linear, append-only render across passes; no screen-clear/alt-screen.
- **Loop ends on `y`/`n`** — terminal choices exit the loop; `e`/`r` continue it.

**Context**:
> Regenerate / edit re-entry — the engine owns the loop: `Prompt(gate)` returns a single `choice` (`y`/`n`/`e`/`r`) and is render-only. On `e`/`r` the engine does the work — invoking `$EDITOR` (edit) or re-running generation via `claude` (regenerate-with-context) — then re-calls `ShowNotes` with the refreshed body and `Prompt` again, looping until `y`/`n`. The presenter never calls `$EDITOR` or `claude`; it only re-renders on each pass. Rendering is linear — each pass re-prints the notes block + gate below (it scrolls; no screen-clearing or alt-screen).
> Because the engine drives the handoff, it is also the engine that stops the pretty spinner before `$EDITOR` takes over the terminal and resumes after. (The spinner stop/resume is deferred to Phase 4 — out of scope here.)
> NOT Bubble Tea / no alt-screen / no full-screen TUI. Print-style linear narration only.
> `$EDITOR` (note edit) takes over the terminal — the spinner is stopped before handing off, resumed after. (Phase 4.)

**Spec Reference**: `.workflows/mint-release-tool/specification/cli-presentation/specification.md` — "Gating & `-y` Orthogonality" (Regenerate / edit re-entry — the engine owns the loop), "Library Selection" (no alt-screen / linear narration), "Spinner Lifecycle" (`$EDITOR` hand-off — deferred to Phase 4).
