package notes

import (
	"context"

	"mint/internal/config"
	"mint/internal/runner"
)

// NoAIBody produces the notes body for a DELIBERATE --no-ai skip. Unlike the normal
// AI path, --no-ai is not a failure: it never calls the AI, never consults
// on_notes_failure, and NEVER aborts — it simply returns a body.
//
// The body comes from the SHARED fallback-body selector (resolveFallbackBody), the
// SAME builder on_notes_failure=fallback uses, so the two paths can never drift:
//
//   - rel.Fallback set → that fixed string verbatim (no git call);
//   - else the commit-subject list since the last tag (`git log --format=%s`);
//   - else (no commits since the last tag) a non-empty minimal record floor, so the
//     tag body is never empty.
//
// CONTRACT — NO AI, NO ABORT: this file imports nothing AI-related (no internal/ai),
// so there is structurally no transport to reach. The only error it can return is a
// GENUINE git failure surfaced by the selector — the no-AI policy itself never
// produces an error. on_notes_failure (rel.OnNotesFailure) is deliberately NOT
// consulted here: it governs only the normal AI path.
//
// The notes-path precedence (a later task) selects this provider when --no-ai is set
// (after the first-release and degenerate checks); the flag parsing and wiring are
// separate tasks. This provides only the body behaviour the flag selects.
func NoAIBody(ctx context.Context, r runner.CommandRunner, lastTag string, rel config.Release) (string, error) {
	return resolveFallbackBody(ctx, r, lastTag, rel.Fallback)
}
