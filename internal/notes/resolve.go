package notes

import (
	"context"
	"errors"
	"fmt"

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
// driven by rel.OnNotesFailure. It is the implementation of the spec's
// `on_notes_failure` policy (default abort / opt-in fallback), interpreting the single
// documented config key as follows:
//
//   - "" or "abort" → ABORT mode (the default). Return ("", abortErr) where abortErr
//     NAMES the cause (mapping the known sentinels to readable text) and wraps the
//     original failure with %w so errors.Is(abortErr, <cause>) still holds. Nothing is
//     tagged — the run stops before the tag (the presenter surfaces it in a later
//     wiring task); this resolver just returns the abort error and NO body, and runs
//     NO git (no fallback record is built when aborting).
//   - "fallback" → FALLBACK mode with the DEFAULT body: the commit-subject list since
//     the last tag (see FallbackBody). Return (body, nil).
//   - any OTHER non-empty value → FALLBACK mode with that value used as the FIXED
//     fallback body string. Return (value, nil) and run NO git (the fixed string IS the
//     body, so no commit-subject record is needed).
//
// This mapping extends the documented value set ("abort | fallback") rather than
// inventing a second config key: a third "value" interpretation (the fixed string)
// rides on the same key, which is how the spec's "can be a fixed configurable string"
// maps onto the single on_notes_failure knob.
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
	switch rel.OnNotesFailure {
	case "", "abort":
		return "", abortError(failure)
	case "fallback":
		return FallbackBody(ctx, r, lastTag)
	default:
		// Any other non-empty value is a fixed fallback body string used verbatim.
		return rel.OnNotesFailure, nil
	}
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
