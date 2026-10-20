package notes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"mint/internal/ai"
	"mint/internal/config"
	"mint/internal/record"
	"mint/internal/runner"
)

// rangeSubjects lists the commit subjects over revRange, git's stdout verbatim.
//
// This metadata is a FALLBACK RECORD ONLY — it is NEVER fed to the AI. The AI ingests
// ONLY the diff (plus the Change Map and instructions); commit messages are out of
// scope for the AI input by design. The list exists purely to give the non-AI routes
// an honest, non-hallucinated record when the AI body is unavailable.
func rangeSubjects(ctx context.Context, r runner.CommandRunner, revRange string) (string, error) {
	res, err := r.Run(ctx, "git", "log", "--format=%s", revRange)
	if err != nil {
		return "", fmt.Errorf("building commit-subject fallback for %s: %w", revRange, err)
	}
	return res.Stdout, nil
}

// ResolveFailure routes a NORMAL-PATH notes failure to a body or an abort error,
// driven by rel.OnNotesFailure. It implements the spec's `on_notes_failure` policy
// (default abort / opt-in fallback) as a MODE-ONLY knob (typed "abort | fallback"):
//
//   - "" or "abort" → ABORT mode (the default). Return ("", abortErr) where abortErr
//     NAMES the cause (mapping the known sentinels to readable text) and wraps the
//     original failure with %w so errors.Is(abortErr, <cause>) still holds. It returns
//     NO body and runs NO git (no fallback record is built when aborting).
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

func resolveFallbackBody(ctx context.Context, r runner.CommandRunner, lastTag string) (string, error) {
	return RangeNoAIBody(ctx, r, lastTag+"..HEAD")
}

// RangeNoAIBody is the commit-subject list over revRange, or the maintenance-release
// body when the range lists none.
func RangeNoAIBody(ctx context.Context, r runner.CommandRunner, revRange string) (string, error) {
	body, err := rangeSubjects(ctx, r, revRange)
	if err != nil {
		return "", err
	}
	return floorEmptySubjects(body), nil
}

// VersionNoAIBody is the commit-subject list over a released version's own revRange,
// leaving out the version's bookkeeping commit as its forward release's list never held
// it, or the maintenance-release body when no subject is left. The bookkeeping commit
// is the one tag sits on when its subject ends in " Release {tag}", under any
// commit_prefix the version was released with.
func VersionNoAIBody(ctx context.Context, r runner.CommandRunner, revRange, tag string) (string, error) {
	body, err := rangeSubjects(ctx, r, revRange)
	if err != nil {
		return "", err
	}
	return floorEmptySubjects(withoutBookkeepingCommit(body, tag)), nil
}

// withoutBookkeepingCommit drops the first subject when it is tag's bookkeeping
// commit. git log lists a range's tip first, so the first subject is the commit the
// tag sits on.
func withoutBookkeepingCommit(subjects, tag string) string {
	first, rest, _ := strings.Cut(subjects, "\n")
	if strings.HasSuffix(first, record.BookkeepingSubject("", tag)) {
		return rest
	}
	return subjects
}

func floorEmptySubjects(body string) string {
	if strings.TrimSpace(body) == "" {
		return StubBody()
	}
	return body
}

// abortError builds the abort-mode error: a readable cause phrase derived from the
// failure, wrapping the original failure with %w so the original sentinel stays
// matchable via errors.Is. An unmapped cause falls back to the failure's own message.
func abortError(failure error) error {
	return fmt.Errorf("notes generation failed (%s): %w", causeText(failure), failure)
}

// Cause is a known notes-failure cause; its zero value is none.
type Cause int

const (
	CauseNoRoomForDiff Cause = iota + 1
	CauseMergeTooLarge
	CauseTooLargeForModel
	CauseTimeout
	CauseCommandMissing
	CauseGenerationFailed
)

// ClassifyFailure sorts a notes/AI failure into its Cause, matched through the %w
// chain. Order matters: a size sentinel may wrap the call's own ai.ErrGenerationFailed,
// and its size cause wins. It returns (cause, true) for a known notes failure and
// (0, false) for any other error.
func ClassifyFailure(failure error) (Cause, bool) {
	switch {
	case errors.Is(failure, ErrNoRoomForDiff):
		return CauseNoRoomForDiff, true
	case errors.Is(failure, ErrMergeTooLarge):
		return CauseMergeTooLarge, true
	case errors.Is(failure, ErrTooLargeForModel):
		return CauseTooLargeForModel, true
	case errors.Is(failure, ai.ErrTimeout):
		return CauseTimeout, true
	case errors.Is(failure, ai.ErrCommandMissing):
		return CauseCommandMissing, true
	case errors.Is(failure, ai.ErrGenerationFailed):
		return CauseGenerationFailed, true
	}
	return 0, false
}

// CauseText derives the concise display phrase for a notes/AI failure from its
// ClassifyFailure cause. Of the size causes only a too-large-for-the-model takes a
// location. A generation failure reads by whether its captured output holds any text,
// and one that belongs to one call of a chunked run ends with that call's place in it.
// It returns (phrase, true) for a known notes failure and ("", false) for any other
// error.
func CauseText(failure error) (string, bool) {
	cause, known := ClassifyFailure(failure)
	if !known {
		return "", false
	}
	var phrase string
	switch cause {
	case CauseNoRoomForDiff:
		return ErrNoRoomForDiff.Error(), true
	case CauseMergeTooLarge:
		return ErrMergeTooLarge.Error(), true
	case CauseTooLargeForModel:
		phrase = ErrTooLargeForModel.Error()
	case CauseTimeout:
		phrase = "AI timed out"
	case CauseCommandMissing:
		phrase = "AI tool not installed"
	case CauseGenerationFailed:
		phrase = generationFailurePhrase(failure)
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
