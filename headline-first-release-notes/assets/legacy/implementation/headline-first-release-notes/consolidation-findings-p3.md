# Consolidation Findings: headline-first-release-notes (Phase 3)

## Findings

None.

## Comment Corrections

- internal/engine/notes_cannotcall_test.go:20 — false since this phase grew the default prompt. `partCapBytes` is `len(notes.DefaultPrompt) + 11,500`. That was 24,025 bytes at the phase base, when the default prompt was 12,525 bytes. Task 3-5 brought the prompt to 16,397 bytes, which makes `partCapBytes` 27,897, above `outsizedContext`'s 27,600 bytes (46 × 600). The "prompt override leaves no room for diff" case still stops before any call, but only because mint's own text pushes the fixed part over the cap: about 1 KB of chunk framing, closing reminder and Change Map. Task 3-5 resized `bigPartNotes`, the declaration three lines below, for the same growth, and left this line as it was.
  OLD: // outsizedContext is operator text larger than partCapBytes on its own.
  NEW: // outsizedContext is operator text long enough that, with mint's own text around
  // it, no diff fits under partCapBytes.
