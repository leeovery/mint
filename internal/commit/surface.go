package commit

import (
	"path/filepath"
	"time"

	"mint/internal/presenter"
)

// surface narrates a stage failure through the presenter and returns the cause so the
// orchestrator aborts. It mirrors the engine's surface helper: a failed stage emits a
// StageFailed (the presenter renders it) and the cause flows up to the cmd layer,
// which maps any non-nil commit error to a non-zero exit. The bare commit path
// mutates nothing before the commit sink, so there is never anything to unwind — a
// failure is always a plain surface.
func surface(p presenter.Presenter, stage string, cause error) error {
	return surfaceOutput(p, stage, cause, "")
}

// surfaceOutput is surface with the failed command's captured output passed through
// VERBATIM as StageFailure.Output. An empty output degrades to exactly surface's
// rendering, so callers pass whatever output they captured unconditionally.
func surfaceOutput(p presenter.Presenter, stage string, cause error, output string) error {
	return surfaceFailure(p, presenter.StageFailure{Name: stage, Message: cause.Error(), Output: output}, cause)
}

// surfaceTimed is surface for a blocking stage that ran for elapsed before it failed.
func surfaceTimed(p presenter.Presenter, stage string, cause error, elapsed time.Duration) error {
	return surfaceFailure(p, presenter.StageFailure{Name: stage, Message: cause.Error(), Elapsed: elapsed}, cause)
}

// surfaceFailure renders failure, keyed to the plan stage its label fails and marked
// interrupted where cause is the run's cancel, and returns cause.
func surfaceFailure(p presenter.Presenter, failure presenter.StageFailure, cause error) error {
	failure.Key, failure.Interrupted = stageKey(failure.Name), presenter.Interrupted(cause)
	p.StageFailed(failure)
	return cause
}

// The plan stage keys of a commit run.
const (
	stageDiff    = "diff"
	stageMessage = "message"
	stageStage   = "stage"
	stageCommit  = "commit"
	stagePush    = "push"
)

// stageKey is the plan stage a failure labelled label fails: the preflight finding
// nothing to commit fails the diff, and every failure on the way to a message fails
// the message.
func stageKey(label string) string {
	switch label {
	case "preflight":
		return stageDiff
	case "generate", "editor", "review", "regenerate":
		return stageMessage
	default:
		return label
	}
}

// projectName derives the project label from the repo root's final path segment —
// the same stand-in release uses for the brand/header lines.
func projectName(root string) string {
	return filepath.Base(root)
}
