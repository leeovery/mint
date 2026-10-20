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

// FallbackBody builds the commit-subject list, the one non-AI body: the subjects since
// the last tag, produced by `git log --format=%s {lastTag}..HEAD` through the
// CommandRunner seam. It returns git's stdout verbatim (subjects, one per line).
//
// This metadata is a FALLBACK RECORD ONLY — it is NEVER fed to the AI. The AI ingests
// ONLY the diff (plus the Change Map and instructions); commit messages are out of
// scope for the AI input by design. FallbackBody exists purely to give the non-AI
// routes an honest, non-hallucinated record when the AI body is unavailable.
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
//     selector: the commit-subject list since the last tag, else (empty log) the
//     maintenance-release body. Return (body, nil).
//   - any OTHER value → ABORT. Load already rejects such values; aborting keeps an
//     unvalidated Release from tagging on a misconfigured value.
//
// A cancellation (context.Canceled) is not a notes failure: it is returned unchanged
// in either mode, so an interrupted run never ships a fallback body.
//
// VARIED CAUSES all route the SAME way: abort names whichever cause; fallback produces
// the body regardless of cause. The cause only changes the abort message's named text,
// never the routing.
//
// CONTRACT — NORMAL PATH ONLY: ResolveFailure is invoked solely when the generator
// surfaces a typed failure. Every non-AI branch of the notes precedence returns before
// the AI is called, so none reaches here; this function does not itself implement that
// precedence.
func ResolveFailure(ctx context.Context, r runner.CommandRunner, failure error, lastTag string, rel config.Release) (string, error) {
	if errors.Is(failure, context.Canceled) {
		return "", failure
	}
	if rel.OnNotesFailure == "fallback" {
		return resolveFallbackBody(ctx, r, lastTag)
	}
	return "", abortError(failure)
}

// resolveFallbackBody is the SHARED non-AI body selector every non-AI route calls,
// so they can never drift. It produces a NON-EMPTY body:
//
//   - the commit-subject list via FallbackBody; a genuine git error surfaces.
//   - if that list is empty or whitespace-only (no commits since the last tag), the
//     maintenance-release body (StubBody) instead, so the tag body is never empty.
func resolveFallbackBody(ctx context.Context, r runner.CommandRunner, lastTag string) (string, error) {
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

// CauseText derives the concise display phrase for a notes/AI failure, matched through
// the %w chain. A size sentinel's phrase wins over the call failure it may wrap, and
// only ErrTooLargeForModel's takes a location. A generation failure reads by whether
// its captured output holds any text, and one that belongs to one call of a chunked
// run ends with that call's place in it.
// It returns (phrase, true) for a known notes failure and ("", false) for any other
// error.
func CauseText(failure error) (string, bool) {
	var phrase string
	switch {
	case errors.Is(failure, ErrNoRoomForDiff):
		return ErrNoRoomForDiff.Error(), true
	case errors.Is(failure, ErrMergeTooLarge):
		return ErrMergeTooLarge.Error(), true
	case errors.Is(failure, ErrTooLargeForModel):
		phrase = ErrTooLargeForModel.Error()
	case errors.Is(failure, ai.ErrTimeout):
		phrase = "AI timed out"
	case errors.Is(failure, ai.ErrCommandMissing):
		phrase = "AI tool not installed"
	case errors.Is(failure, ai.ErrGenerationFailed):
		phrase = generationFailurePhrase(failure)
	default:
		return "", false
	}
	var call *callError
	if errors.As(failure, &call) {
		phrase += " (" + call.location + ")"
	}
	return phrase, true
}

func generationFailurePhrase(failure error) string {
	var genErr *ai.GenerationError
	if errors.As(failure, &genErr) && genErr.Output() != "" {
		return "AI call failed"
	}
	return "AI returned empty notes"
}

// causeText maps a known notes-failure sentinel to a readable cause phrase for the
// abort message. It delegates to the exported CauseText so the two cannot drift; an
// unknown cause falls back to the failure's own message so the abort error is always
// informative (preserving abortError's byte-identical output for unmapped causes).
func causeText(failure error) string {
	if phrase, known := CauseText(failure); known {
		return phrase
	}
	return failure.Error()
}
