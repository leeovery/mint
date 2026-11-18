---
topic: cli-presentation
cycle: 5
total_proposed: 2
---
# Analysis Tasks: CLI Presentation (Cycle 5)

## Task 1: Extract shared AST import-scanning guard helper across the two dependency-guard tests
status: approved
severity: low
sources: duplication

**Problem**: Two test files independently re-author the same go/parser ImportsOnly scan loop. `TestPlainPresenterImportsNoUILibrary` (internal/presenter/plain_test.go:1013-1029) and `TestPromptPathImportsNoSubprocessDependency` (internal/presenter/prompt_render_only_test.go:62-85) both run the identical core: `token.NewFileSet()` → range source paths → `parser.ParseFile(fset, path, nil, parser.ImportsOnly)` (with `t.Fatalf` on parse error) → range `file.Imports` → `strings.Trim(imp.Path.Value, "\"")` → range a marker slice → match → `t.Errorf`. The prompt guard's own doc comment states it "MIRRORS the UI-library guard ... and its go/parser ImportsOnly approach". The two guards diverge only in (a) which sources they scan (`plainPresenterSources` vs `presenterNonTestSources`, each already its own named helper), (b) the marker slice (`uiLibraryMarkers` vs `subprocessMarkers`), and (c) the match predicate (`strings.Contains` vs `==`). A future change to the parse/scan mechanics must currently be made in two places.

**Solution**: Extract a single shared test helper — e.g. `assertImportsExclude(t *testing.T, sources []string, markers []string, exact bool)` — that runs the parse-and-scan loop once, parameterised on the source paths, the marker slice, and a match mode (exact-equality vs substring). Have BOTH guards call it with their own sources/markers/match-mode. Keep each guard's existing source-glob helper and marker slice as-is; only the ~12-line parse-and-scan loop collapses into the helper. Move the prompt guard's `scanned == 0` defence INTO the shared helper so BOTH guards gain the regression protection against a glob/parse failure silently scanning nothing.

**Outcome**: One shared helper owns the parse-and-scan mechanics; each guard is reduced to a single call passing its own sources, markers, and match mode. Both guards inherit the `scanned == 0` defence. A future change to the scan mechanics is made in exactly one place. No production code changes; both guards still FAIL if their banned import appears.

**Do**:
1. In the presenter test package, add `assertImportsExclude(t *testing.T, sources []string, markers []string, exact bool)`. Inside: `t.Helper()`; `fset := token.NewFileSet()`; `scanned := 0`; range `sources`, `parser.ParseFile(fset, path, nil, parser.ImportsOnly)` with `t.Fatalf` on error; `scanned++`; range `file.Imports`, `p := strings.Trim(imp.Path.Value, "\"")`, range `markers`, and match using `==` when `exact` is true else `strings.Contains(p, marker)`, emitting `t.Errorf` on a match.
2. Preserve each guard's existing error-message wording so the failure messages stay specific (UI-library marker message vs subprocess marker message). If the two messages cannot both be expressed through one helper cleanly, pass the message context (or keep the per-guard `t.Errorf` text via a small formatting closure/param) rather than genericising it into a single vague message.
3. After the scan loop, add the `scanned == 0` `t.Fatal` defence inside the helper so it guards every caller.
4. Rewrite `TestPlainPresenterImportsNoUILibrary` to call `assertImportsExclude(t, plainPresenterSources(t), uiLibraryMarkers, false)` (substring match).
5. Rewrite `TestPromptPathImportsNoSubprocessDependency` to call `assertImportsExclude(t, presenterNonTestSources(t), subprocessMarkers, true)` (exact match), removing its now-duplicated inline loop and local `scanned` bookkeeping.
6. Run `go test ./internal/presenter/...` and confirm both guards still pass.

**Acceptance Criteria**:
- A single shared helper runs the parse-and-scan loop; neither test file contains its own copy of the `parser.ParseFile` ImportsOnly + `file.Imports` range loop.
- Both `TestPlainPresenterImportsNoUILibrary` and `TestPromptPathImportsNoSubprocessDependency` call the shared helper with their own sources, marker slice, and match mode.
- The `scanned == 0` defence lives in the shared helper and therefore protects both guards.
- The plain guard still uses substring matching; the prompt guard still uses exact-equality matching.
- No production (non-test) source is modified.

**Tests**:
- `go test ./internal/presenter/...` passes (both guards green under normal sources).
- Sanity-verify the guards still fail loudly: temporarily inject a banned import marker into the relevant scanned source (e.g. a lipgloss import into plain.go for the UI guard, an os/exec import for the subprocess guard) and confirm each respective test FAILS; revert.
- Sanity-verify the `scanned == 0` defence: confirm both guards fail with the "scanned no sources" fatal if their source glob returns empty.

## Task 2: Tighten deterministic positive substring assertions to exact line matches
status: approved
severity: low
sources: standards

**Problem**: code-quality.md lists "Substring assertions in tests when exact output is deterministic" as an anti-pattern. Several POSITIVE assertions use `strings.Contains` on individual line fragments where the full line is fully deterministic and could be asserted with exact equality (or `HasSuffix` on the complete line, as init_test.go already does well at :314 and :340). The cited sites are: internal/presenter/init_test.go:176 (`"✓"`), :179 (`"created .mint.toml"`), :199 (`"·"`), :202 (`"skipped release (exists, use --force)"`), and internal/presenter/pretty_gate_test.go:81 and :121 (`"    y  accept & proceed [default]"`). This is a minor rigour gap, well-mitigated by the byte-exact golden transcripts and exact per-event line assertions elsewhere — but the cited positive checks could drift weaker than necessary.

**Solution**: At each cited POSITIVE assertion site where the full line is deterministic, switch the `strings.Contains` on a fragment to exact equality on the complete line (or `HasSuffix` on the complete line where leading styling/escape codes make a full-line `==` impractical, matching the existing init_test.go:314/:340 pattern). LEAVE every NEGATIVE/absence `Contains` check as-is (e.g. pretty_gate_test.go's "[default] marker wrongly on a non-default line", "must NOT render an e line", and the "Continue?" absence checks) — `Contains` is the correct tool for proving absence. The line numbers above are approximate: grep the two cited test files for the positive fragment-`Contains` calls and tighten exactly those, not the negative ones.

**Outcome**: The six cited positive assertions assert the complete deterministic line (via `==` or full-line `HasSuffix`) rather than a fragment, making them strictly stronger. Negative/absence `Contains` checks are untouched and remain correct. No production change; no coverage lost — assertions only get stricter.

**Do**:
1. Grep init_test.go and pretty_gate_test.go for the positive fragment-`Contains` assertions at/near the cited lines. Confirm each targets a single, fully deterministic output line.
2. For each, determine the complete expected line. Where the captured value is a single line with no leading dynamic prefix, replace `if !strings.Contains(got, fragment)` with an exact `if got != want` (or compare the relevant trimmed line). Where the line carries leading ANSI/style codes or indentation that make full-line `==` brittle, use `strings.HasSuffix(line, completeLine)` on the complete line content, mirroring init_test.go:314/:340.
3. Keep the existing `t.Errorf` failure messages (or adjust wording minimally to reflect the stricter assertion).
4. Do NOT alter any negative/absence `Contains` check (lines asserting a marker must NOT appear, an option line must NOT render, "Continue?" must NOT appear under -y, etc.).
5. Run `go test ./internal/presenter/...` and confirm the tightened assertions still pass against the real output.

**Acceptance Criteria**:
- The six cited positive assertions assert the complete deterministic line via `==` or full-line `HasSuffix`, not a bare fragment `Contains`.
- All negative/absence `Contains` checks in both files are unchanged.
- No production (non-test) source is modified.
- Tightened assertions pass against the real rendered output.

**Tests**:
- `go test ./internal/presenter/...` passes with the tightened assertions.
- Sanity-verify each tightened assertion is genuinely stricter: confirm it would FAIL if the asserted line differed by surrounding/extra characters the old fragment-`Contains` would have tolerated (spot-check by temporarily perturbing the expected string), then revert.
