---
topic: cli-presentation
cycle: 1
total_proposed: 4
---
# Analysis Tasks: cli-presentation (Cycle 1)

## Task 1: Extract shared byte-purity ASCII-scan test helper
status: pending
severity: medium
sources: duplication

**Problem**: The same ~12-line byte-purity ASCII-scan loop — `for i, b := range ...Bytes() { switch { case b == 0x1b ...; case b == 0x0d ...; case b == '\n' ...; case b < 0x20 || b > 0x7e ... } }`, asserting the plain byte-purity contract (no ESC, no CR, only printable ASCII + newline) — is copy-pasted verbatim across nine test functions in five files. The only variation is the trailing descriptive string and, in two copies, an outer loop over both out and err buffers. The byte-purity invariant now lives in nine places that must be kept in sync, and it is the package's single largest cross-file duplication.

**Solution**: Extract one shared test helper in the `presenter_test` package — `assertBytePureASCII(t *testing.T, buf *bytes.Buffer, context string)` running the byte scan — plus a thin `assertBytePureASCIIStreams(t *testing.T, context string, bufs ...*bytes.Buffer)` wrapper for the two multi-stream call sites. Replace all nine inline blocks with a single call each, passing the per-site context string.

**Outcome**: The byte-purity contract is defined once. All nine assertion sites call the shared helper; behaviour is unchanged and the full test suite still passes. A future change to the byte-purity rule edits one location.

**Do**:
1. Add a new test helper (e.g. `internal/presenter/bytepurity_test.go` in package `presenter_test`) defining `assertBytePureASCII(t *testing.T, buf *bytes.Buffer, context string)`. It marks itself `t.Helper()` and runs the existing scan logic, reporting failures with `context` plus the byte offset and offending byte (preserving the current per-arm failure messaging).
2. Add `assertBytePureASCIIStreams(t *testing.T, context string, bufs ...*bytes.Buffer)` that calls `t.Helper()` then loops over `bufs` invoking `assertBytePureASCII` with a stream-distinguishing context suffix, matching the current two-buffer (out/err) call sites.
3. Replace the inline loop in each of the nine sites with a single helper call, passing the existing per-site context string:
   - `internal/presenter/plain_test.go:479` (multi-stream → use `assertBytePureASCIIStreams`)
   - `internal/presenter/plain_test.go:534`
   - `internal/presenter/plain_test.go:743`
   - `internal/presenter/plain_test.go:827`
   - `internal/presenter/plain_test.go:950`
   - `internal/presenter/gate_skip_test.go:146`
   - `internal/presenter/init_test.go:108`
   - `internal/presenter/gate_sourcetarget_test.go:196`
   - `internal/presenter/gate_forbidden_test.go:74` (multi-stream → use `assertBytePureASCIIStreams`)
4. Run `go test ./internal/presenter/...` and `go vet ./...`; confirm green.

**Acceptance Criteria**:
- Exactly one definition of the byte-purity ASCII-scan exists (the helper); no inline copy remains in any of the nine sites.
- The two multi-stream sites (plain_test.go:479, gate_forbidden_test.go:74) assert both out and err via `assertBytePureASCIIStreams`.
- Each call retains its original per-site context string in failure output.
- `go test ./internal/presenter/...` and `go vet ./...` pass with no behavioural change.

**Tests**:
- The existing nine byte-purity assertions continue to pass unchanged after refactor (this is a test-only consolidation; the production behaviour they guard is untouched).
- Sanity-verify the helper still fails when fed a buffer containing an ESC byte (temporary local check, not committed) to confirm the extraction preserved the negative path.

## Task 2: Neutralise the hardcoded plain blocking-stage start verb
status: pending
severity: medium
sources: standards

**Problem**: `StageStarted` in plain mode renders a fixed `"{name}: generating...\n"` for ALL blocking stages (`internal/presenter/plain.go:147-152`). "generating" is stage-specific narration: it fits the AI notes stage (`notes: generating…`, the spec's only worked example) but reads wrong for the other named blocking stage — the `pre_tag` build hook (`prep`), which the spec explicitly calls long/blocking. That stage renders `prep: generating...`, which is semantically incorrect (a build hook is not "generating"). The StageStart payload carries only `Name` + `Blocking`, so the presenter has no engine-supplied start verb and invents one — exactly the presenter-side stage assumption the spec's event-payload principle (specification.md lines 64-66) is designed to prevent.

**Solution**: Replace the synthesised stage-specific word with a stage-agnostic verb that is correct for both notes generation and a build hook — `"{name}: running...\n"`. Keep the ASCII ellipsis (the byte-purity guard is fixed) and keep the line presenter-synthesised. This is the minimal, spec-permitted ("wording refinable") change that removes the wrong-sounding output without introducing a new payload field.

**Outcome**: Plain blocking-stage start lines read correctly for every named blocking stage (`notes: running...`, `prep: running...`). No stage-specific verb is hardcoded; the line remains byte-pure ASCII and the pretty side is unaffected.

**Do**:
1. In `internal/presenter/plain.go`, change the format string in `StageStarted` from `"%s: generating...\n"` to `"%s: running...\n"` (preserve the ASCII `...`, not U+2026).
2. Update the surrounding doc comment (lines 144-146) so it no longer cites "generating..." as the example word — describe the verb as a stage-agnostic synthesised start word kept byte-pure ASCII.
3. Update any plain `StageStarted` test expectations that assert the literal `generating...` string to expect `running...` (search `internal/presenter` tests for `generating`).
4. Run `go test ./internal/presenter/...` and `go vet ./...`; confirm green.

**Acceptance Criteria**:
- Plain `StageStarted` emits `"{name}: running...\n"` for blocking stages and remains silent for non-blocking stages.
- No occurrence of the stage-specific verb "generating" remains in plain start narration or its doc comment.
- The start line is still byte-pure ASCII (uses `...`, not the U+2026 ellipsis) and passes the byte-purity guard.
- `go test ./internal/presenter/...` and `go vet ./...` pass.

**Tests**:
- Existing plain `StageStarted` blocking-stage test updated to assert `"{name}: running...\n"`; the non-blocking-stage silence test continues to pass.
- The byte-purity assertion covering the start line continues to pass (ASCII ellipsis preserved).

## Task 3: Collapse the four pretty constructors into one with functional options
status: pending
severity: medium
sources: architecture

**Problem**: `PrettyPresenter` exposes four exported constructors (`NewPrettyPresenter`, `NewPrettyPresenterWithProfile`, `NewPrettyPresenterWithErr`, `NewPrettyPresenterWithInput`) plus a `WithInput` setter that exists only to patch a gap one constructor leaves (`NewPrettyPresenterWithErr` defaults input to `os.Stdin`, so `WithInput` re-injects a scripted reader). Each constructor hard-codes a different subset of three orthogonal knobs (profile, err writer, input reader), so the combination "force colour AND capture err AND script input" is only reachable by chaining a setter onto a constructor that omits an axis. This is the constructor explosion the field docs themselves anticipated, and the asymmetric outlier versus the clean two-constructor plain side. (`internal/presenter/pretty.go:183,194,203,214,272`.)

**Solution**: Collapse the three profile/err/input variants into one constructor taking functional options: `NewPrettyPresenter(out io.Writer, opts ...Option) *PrettyPresenter`, with `WithProfile(termenv.Profile)`, `WithErr(io.Writer)`, and `WithInput(io.Reader)` options. Keep a thin production entry. Remove the bespoke `NewPrettyPresenterWith...` constructors and the `WithInput`-patches-a-gap setter wart. Rendered behaviour and the `Presenter` seam are unaffected — this is a pure test-ergonomics refactor.

**Outcome**: Every pretty-presenter capability combination (any subset of forced profile, err writer, scripted input) is expressible through one constructor plus composable options. No setter exists solely to backfill a constructor gap. Adding a future axis adds an option, not a constructor.

**Do**:
1. Define an `Option` type (`func(*PrettyPresenter)` or a small config struct applied in the constructor core) in `internal/presenter/pretty.go`.
2. Implement options: `WithProfile(profile termenv.Profile)` (sets the renderer colour profile), `WithErr(err io.Writer)` (wires the err writer), `WithInput(in io.Reader)` (sets the gate input reader). Ensure profile-forcing still calls `renderer.SetColorProfile` and that the default renderer/profile/input (`os.Stdin`) match today's `newPrettyPresenter` defaults when no option is given.
3. Rewrite `NewPrettyPresenter(out io.Writer, opts ...Option)` to build via `newPrettyPresenter` defaults then apply options in order. Preserve the existing default field initialisation (stdinInteractive=true, styles, `newSpinner=newBriandownsSpinner`).
4. Remove `NewPrettyPresenterWithProfile`, `NewPrettyPresenterWithErr`, `NewPrettyPresenterWithInput`, and the `WithInput` method (folded into the option). Keep the chainable builder setters that are NOT constructor-gap patches (`WithYes`, `WithInteractiveStdin`, `WithSpinnerFactory`) as-is.
5. Update all call sites — production wiring (`internal/presenter/wiring.go`) and every test using the removed constructors/`WithInput` setter — to the new `NewPrettyPresenter(out, opts...)` form. Translate each old call to the equivalent option set (e.g. `NewPrettyPresenterWithErr(out, errBuf, termenv.TrueColor)` → `NewPrettyPresenter(out, WithErr(errBuf), WithProfile(termenv.TrueColor))`).
6. Run `go build ./...`, `go vet ./...`, and `go test ./internal/presenter/...`; confirm green.

**Acceptance Criteria**:
- `PrettyPresenter` has a single exported constructor `NewPrettyPresenter(out io.Writer, opts ...Option)`; the three `NewPrettyPresenterWith...` variants and the `WithInput` setter are gone.
- The "force colour AND capture err AND script input" combination is reachable in one constructor call with three options, no setter chaining required to fill a gap.
- Production wiring and all tests compile and pass against the new API; rendered output and the `Presenter` seam are byte-for-byte unchanged.
- `go build ./...`, `go vet ./...`, and `go test ./internal/presenter/...` pass.

**Tests**:
- Existing pretty-presenter tests retargeted to the option-based constructor continue to pass, covering: colour-forced out-only (`WithProfile`), stderr-split capture (`WithErr` + `WithProfile`), and scripted gate input (`WithInput` + `WithProfile`).
- A test exercising all three options together asserts the previously-awkward combination now constructs in one call and behaves identically.

## Task 4: State the ASCII/case-fold precondition on SourceGate/TargetGate
status: pending
severity: low
sources: architecture

**Problem**: `SourceGate`/`TargetGate` reuse the `Choice`/`GateChoice` vocabulary built for the engine's fixed semantic answers (accept/abort/edit/regenerate — y/n/e/r) to carry open-ended enumerated values (github/gitlab, stable/beta), setting `AcceptEcho = string(def)` (`internal/presenter/gate.go:184,200`). The shared parse path (`parseChoice`) lower-cases/case-folds these source/target values the same way it folds y/n, and the `AcceptEcho` byte-purity constraint silently depends on every engine-supplied source/target value being ASCII — a contract the type does not enforce and that would render wrong (or trip the plain byte-purity guard) for a non-ASCII target name. The seam holds for the current enumerations but rests on caller discipline rather than being self-contained.

**Solution**: No structural change. Convert the silent dependency into a stated contract: add a precondition doc comment on `SourceGate` and `TargetGate` declaring that the supplied options and default must be ASCII enumerated values only (because the shared parse path case-folds them and `AcceptEcho` must stay byte-pure ASCII for the plain echo).

**Outcome**: The latent ASCII/case-fold assumption is documented at the two gate constructors, so a future engine change that broadens source/target values beyond ASCII has a stated precondition to consult rather than discovering the case-fold/byte-purity breakage at runtime.

**Do**:
1. In `internal/presenter/gate.go`, extend the `SourceGate` doc comment to state the precondition: options and `def` must be ASCII enumerated values, because `parseChoice` case-folds them and `AcceptEcho = string(def)` must stay byte-pure ASCII for the plain echo; non-ASCII values would render wrong or trip the plain byte-purity guard.
2. Add the equivalent precondition note to the `TargetGate` doc comment.
3. Run `go vet ./...`; confirm green (no behaviour change, so existing tests remain valid).

**Acceptance Criteria**:
- `SourceGate` and `TargetGate` each carry a doc comment stating the "ASCII enumerated values only" precondition and the reason (case-folding parse + byte-pure `AcceptEcho`).
- No functional/code change beyond comments; rendered behaviour and the seam are unchanged.
- `go vet ./...` and `go test ./internal/presenter/...` pass.

**Tests**:
- No new behavioural test required (documentation-only change). The existing source/target gate tests continue to pass, confirming no regression.
