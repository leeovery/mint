TASK: Fix the Whitespace-Only $EDITOR Panic (Promoted Bug) — mint-release-tool-10-8 (type: bug, severity: high)

ACCEPTANCE CRITERIA:
- `EDITOR=" "` (whitespace-only) does not panic.
- The run either launches `vi` (fall-through) or returns to the gate with a Warn + `ErrEditorReturnToGate`, with the temp file cleaned up.

STATUS: Complete

SPEC CONTEXT:
Specification "Interactive Confirmation & Notes Review" (`e` edit, line 453) states: "Editor resolution: $VISUAL then $EDITOR, falling back to a sensible default (vi); if no editor can be launched, mint reports the problem and returns to the gate rather than crashing." The chosen remediation (treat whitespace-only as unset → fall through to vi) is fully consistent with this: a whitespace-only $VISUAL/$EDITOR is functionally equivalent to "unset", so falling through to the vi default matches the documented precedence exactly, and "rather than crashing" is the explicit anti-panic requirement this task enforces.

IMPLEMENTATION:
- Status: Implemented (option 2 — trim-in-ResolveEditor fall-through)
- Location: `internal/engine/editor.go:45-53` (ResolveEditor)
- Notes:
  - `ResolveEditor` now guards each candidate with `strings.TrimSpace(...) != ""` (lines 46, 49) rather than the previous bare-empty-string check. A blank-after-trim $VISUAL falls through to $EDITOR; a blank-after-trim $EDITOR falls through to `defaultEditor` ("vi", line 30/52).
  - This makes `fields[0]` at line 102 and `append(fields[1:], tmpPath)` at line 103 provably safe: any value returned by `ResolveEditor` either passed `TrimSpace != ""` (so `strings.Fields` yields >= 1 element) or is the literal "vi" constant. The empty-slice index panic is no longer reachable. Root cause is eliminated at source rather than patched at the call site.
  - The temp-file cleanup (`defer func() { _ = os.Remove(tmpPath) }()`, line 98) was already unconditional and is unaffected — it runs on every return path including the new fall-through-to-vi path.
  - The doc comments (lines 36-38) accurately describe the new behaviour and the reasoning ("strings.Fields would split it into an empty slice ... falls through to the next candidate exactly as an unset variable does").
  - Consistency: the chosen approach (fall-through to vi) is one of the two plan-sanctioned options and is internally consistent — the missing-editor path (Warn + ErrEditorReturnToGate) still applies only when a launchable editor name resolves but the binary is not found (lines 110-119). A whitespace-only value never reaches that branch because it resolves to vi first. This is the correct division: whitespace-only == unset (resolution concern), missing-binary == launch failure (launcher concern).

TESTS:
- Status: Adequate
- Coverage:
  - `TestResolveEditor_PrefersVisualThenEditorThenVi` (editor_test.go:21-52) adds three whitespace-only cases (lines 37-39): whitespace VISUAL falls to nano EDITOR; whitespace VISUAL + whitespace EDITOR falls to vi; empty VISUAL + whitespace EDITOR falls to vi. These pin the resolution-level fix directly and cover both tab and space whitespace (`"\t "`, `" "`, `"   "`).
  - `TestEditorLauncher_Edit_WhitespaceOnlyEditor_FallsThroughToVi` (editor_test.go:137-171) is the end-to-end anti-panic test the task requires: sets `VISUAL="   "`, `EDITOR=" "`, runs `Edit`, asserts no error/panic, asserts the resolved binary launched is `vi` (line 157), asserts exactly the temp path is appended (lines 160-166), and asserts the temp file is cleaned up on the success path (lines 167-170). This directly verifies both acceptance criteria.
- Notes:
  - Both required behaviours (no panic; launches vi with temp cleanup) are asserted. The test would fail if the fix regressed — without the trim guard, `ResolveEditor` would return `" "`, `strings.Fields` would yield an empty slice, and `fields[0]` would panic, failing the test (panic surfaces as a test failure).
  - Not over-tested: the unit-level resolution cases and the one integration-level launcher case are complementary, not redundant — the table test pins resolution precedence cheaply; the launcher test proves the panic is actually gone through the real `Edit` call path including temp-file lifecycle. No excess mocking; the existing `writeBackRunner` double is reused.
  - The pre-existing test doc at lines 22-25 still says ResolveEditor "checks for non-empty" / `""` stands in for "unset" — slightly stale now that the check is TrimSpace-based, but the comment's intent (empty == unset) remains true and the whitespace cases are explained separately at lines 35-36. Cosmetic only.

CODE QUALITY:
- Project conventions: Followed. Aligns with golang-safety guidance — the slice-index panic risk (skill's "Slice: index into nil/empty → panic") is eliminated at the source by ensuring the producer never emits a blank value. No bare slice indexing on untrusted-length input remains reachable.
- SOLID principles: Good. The fix respects single-responsibility: resolution normalises the editor value; the launcher splits/launches. Whitespace handling lives where precedence logic already lives.
- Complexity: Low. Two `strings.TrimSpace` calls replace two equality checks; no new branches in the launcher.
- Modern idioms: Yes. Idiomatic `strings.TrimSpace(x) != ""` guard.
- Readability: Good. Doc comment explains the why (empty-slice fall-through) clearly.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/engine/editor_test.go:22-25 — Refresh the comment block: it states ResolveEditor "checks for non-empty" and treats `""` as "unset"; the check is now `strings.TrimSpace(...) != ""`, so reword to "treats empty-or-blank as unset" to match the new behaviour the same test now exercises at lines 37-39.
