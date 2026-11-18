TASK: mint-release-tool-2-13 — Editor resolution for `e` (edit choice)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented. internal/engine/editor.go: ResolveEditor ($VISUAL → $EDITOR → vi via non-empty checks); EditorLauncher.Edit writes temp file, splits editor value with strings.Fields + appends temp path, brackets launch with SuspendSpinner/deferred ResumeSpinner, missing-editor (errors.Is ErrCommandNotFound) → Warn + ErrEditorReturnToGate, other launch error wrapped, reads back verbatim on success, temp file always removed via defer. Gate loop (release.go) branches on ErrEditorReturnToGate via errors.Is and re-presents unchanged.

TESTS:
- Status: Adequate. editor_test.go: resolution order table, success write/launch/read-back-verbatim, arg-splitting `code --wait`, suspend-before/resume-after via Kinds(), no-launchable-editor → Warn + ErrEditorReturnToGate + temp cleanup + deferred resume, genuine launch error wrapped (not mistaken for sentinel). Temp-file cleanup asserted on both paths.

CODE QUALITY:
- Followed conventions (accept-interfaces seam, sentinels + errors.Is, %w, deliberate _= on cleanup). SOLID/DRY good. Low complexity, modern idioms (os.CreateTemp, deferred cleanup), good readability.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [bug] internal/engine/editor.go:99 — `args := append(fields[1:], tmpPath)` will produce an empty `name` and panic at fields[0] (line 98) if the resolved editor value is whitespace-only (e.g. EDITOR=" "), since strings.Fields returns an empty slice. ResolveEditor only guards against empty string, not whitespace-only. Latent, low-likelihood (operator misconfiguration), non-blocking. Fix: in Edit, guard len(fields)==0 after splitting and treat as "no launchable editor" (Warn + ErrEditorReturnToGate), or have ResolveEditor fall through to vi when the value is blank after trimming.
