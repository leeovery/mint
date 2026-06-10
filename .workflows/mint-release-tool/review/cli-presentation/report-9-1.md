TASK: cli-presentation-9-1 — Extract shared AST import-scanning guard helper across the two dependency-guard tests

ACCEPTANCE CRITERIA:
- A single shared helper runs the parse-and-scan loop; neither test file contains its own copy of the parser.ParseFile ImportsOnly + file.Imports range loop.
- Both TestPlainPresenterImportsNoUILibrary and TestPromptPathImportsNoSubprocessDependency call the shared helper with their own sources, marker slice, and match mode.
- The scanned == 0 defence lives in the shared helper and protects both guards.
- The plain guard still uses substring matching; the prompt guard still uses exact-equality matching.
- No production (non-test) source is modified.

STATUS: Complete

SPEC CONTEXT: Pure test-quality refactor (severity low, source: duplication). Two dependency guards encode spec contracts — plain presenter's zero-UI-dependency/token-efficiency property and the prompt path's render-only contract (presenter cannot spawn editor/subprocess). Both re-authored the identical go/parser ImportsOnly scan; this consolidates the scan mechanics into one helper without weakening either guard.

IMPLEMENTATION:
- Status: Implemented
- Location: import_guard_helpers_test.go:24-58 (new shared assertImportsExclude(t, sources, markers, exact) — sole parser.ParseFile ImportsOnly + file.Imports loop, exact/substring switch, scanned==0 defence); plain_test.go:1011-1015 (reduced to assertImportsExclude(..., uiLibraryMarkers, false)); prompt_render_only_test.go:60-64 (reduced to assertImportsExclude(..., subprocessMarkers, true); old inline loop removed, go/parser+go/token imports dropped). Source-glob helpers + marker slices preserved.
- Notes: No production source touched (all changes in *_test.go, package presenter_test). Grep confirms the only remaining ImportsOnly+file.Imports loop is the single helper.

TESTS:
- Status: Adequate
- Coverage: task IS test code. Both guards retain banned-import detection (helper t.Errorf on match). Match modes preserved — plain exact=false (substring, e.g. "lipgloss"), prompt exact=true (e.g. "os/exec"). scanned==0 t.Fatal defence now in helper (:55-57), runs for every caller — both inherit it, plain gains it (previously had none).
- Notes: No under/over-testing introduced. Failure-injection/empty-glob steps are manual; correctly absent from committed code.

CODE QUALITY:
- Project conventions: Followed — t.Helper(), external test package, structural assertions without testify (consistent with existing guards).
- SOLID principles: Good — single helper, one responsibility, parameterised on sources/markers/match-mode.
- Complexity: Low.
- Modern idioms: Yes — go/parser ImportsOnly, strings.Trim, t.Fatalf on parse error.
- Readability: Good — doc comment explains exact/substring distinction and why scanned==0 lives there; diagnostic message names file/import/marker/mode.
- Issues: None.

BLOCKING ISSUES: None.

NON-BLOCKING NOTES: None.
