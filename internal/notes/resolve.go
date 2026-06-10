package notes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/runner"
)

// FallbackBody builds the DEFAULT non-AI fallback body: the commit-subject list
// since the last tag, produced by `git log --format=%s {lastTag}..HEAD` through the
// CommandRunner seam. It returns git's stdout verbatim (subjects, one per line).
//
// This metadata is a FALLBACK RECORD ONLY — it is NEVER fed to the AI. The AI ingests
// ONLY the diff (plus the Change Map and instructions); commit messages are out of
// scope for the AI input by design. FallbackBody exists purely to give the fallback
// path (on_notes_failure=fallback here, and --no-ai in a later task, which shares this
// builder) an honest, non-hallucinated record when the AI body is unavailable.
//
// It is factored as an exported helper so the --no-ai path (task 2-9) can consume the
// same builder rather than re-deriving the git invocation.
func FallbackBody(ctx context.Context, r runner.CommandRunner, lastTag string) (string, error) {
	res, err := r.Run(ctx, "git", "log", "--format=%s", lastTag+"..HEAD")
	if err != nil {
		return "", fmt.Errorf("building commit-subject fallback for %s..HEAD: %w", lastTag, err)
	}
	return res.Stdout, nil
}

// ResolveFailure routes a NORMAL-PATH notes failure to a body or an abort error,
// driven by rel.OnNotesFailure. It implements the spec's `on_notes_failure` policy
// (default abort / opt-in fallback) as a MODE-ONLY knob (typed "abort | fallback"):
//
//   - "" or "abort" → ABORT mode (the default). Return ("", abortErr) where abortErr
//     NAMES the cause (mapping the known sentinels to readable text) and wraps the
//     original failure with %w so errors.Is(abortErr, <cause>) still holds. Nothing is
//     tagged — the run stops before the tag (the presenter surfaces it in a later
//     wiring task); this resolver just returns the abort error and NO body, and runs
//     NO git (no fallback record is built when aborting).
//   - "fallback" → FALLBACK mode. The body comes from the SHARED resolveFallbackBody
//     selector driven by rel.Fallback: the fixed [release].fallback string when set,
//     else the commit-subject list since the last tag, else (empty log) a non-empty
//     minimal record. Return (body, nil).
//   - any OTHER value → treated as ABORT for Phase 2. Phase 6's typed validation will
//     reject unknown on_notes_failure values up front; until then this resolver
//     defaults defensively to abort rather than tagging on a misconfigured value. The
//     fixed-fallback-string overload that previously rode on this key is GONE — the
//     fixed string now comes only from the dedicated rel.Fallback key.
//
// VARIED CAUSES all route the SAME way: abort names whichever cause; fallback produces
// the body regardless of cause. The cause only changes the abort message's named text,
// never the routing.
//
// CONTRACT — NORMAL PATH ONLY: ResolveFailure governs ONLY the normal AI path. It is
// invoked solely when the generator surfaces a typed failure. The notes-path precedence
// (a later task) sits IN FRONT and ensures first-release, degenerate/empty-diff, and
// --no-ai NEVER reach here — those paths never call the AI, so they can never produce a
// failure for this resolver to route. This is a pure function of (failure, lastTag,
// rel): it is meaningful ONLY given a real failure, and does not itself implement that
// precedence.
func ResolveFailure(ctx context.Context, r runner.CommandRunner, failure error, lastTag string, rel config.Release) (string, error) {
	if rel.OnNotesFailure == "fallback" {
		return resolveFallbackBody(ctx, r, lastTag, rel.Fallback)
	}
	// "" / "abort" and any unknown (Phase-6-rejected) value abort, tagging nothing.
	return "", abortError(failure)
}

// resolveFallbackBody is the SHARED fallback-body selector both fallback paths call —
// on_notes_failure=fallback (ResolveFailure) and --no-ai (NoAIBody) — so the two can
// never drift. It produces a NON-EMPTY body via a fixed precedence:
//
//   - fixedBody non-empty → return it verbatim, no error, and run NO git (the fixed
//     string IS the body, so no commit-subject record is built).
//   - else build the commit-subject list via FallbackBody; surface a genuine git error.
//   - else, if that list is empty or whitespace-only (no commits since the last tag),
//     floor to the non-empty minimal record (StubBody) so the tag body is never empty.
func resolveFallbackBody(ctx context.Context, r runner.CommandRunner, lastTag, fixedBody string) (string, error) {
	if fixedBody != "" {
		return fixedBody, nil
	}

	body, err := FallbackBody(ctx, r, lastTag)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) == "" {
		return StubBody(), nil
	}
	return body, nil
}

// abortError builds the abort-mode error: a readable cause phrase derived from the
// failure, wrapping the original failure with %w so the original sentinel stays
// matchable via errors.Is. An unmapped cause falls back to the failure's own message.
func abortError(failure error) error {
	return fmt.Errorf("notes generation failed (%s): %w", causeText(failure), failure)
}

// causeText maps a known notes-failure sentinel to a readable cause phrase for the
// abort message. An unknown cause yields the failure's own message so the abort error
// is always informative.
func causeText(failure error) string {
	switch {
	case errors.Is(failure, ai.ErrTimeout):
		return "AI timed out"
	case errors.Is(failure, ErrDiffTooLarge):
		return "diff too large"
	case errors.Is(failure, ai.ErrCommandMissing):
		return "AI tool not installed"
	case errors.Is(failure, ai.ErrNotesFailure):
		return "AI returned empty/invalid notes after retry"
	default:
		return failure.Error()
	}
}
