package commit

import (
	"path/filepath"

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
	p.StageFailed(presenter.StageFailure{
		Name:    stage,
		Message: cause.Error(),
		Output:  output,
	})
	return cause
}

// projectName derives the project label from the repo root's final path segment —
// the same stand-in release uses for the brand/header lines.
func projectName(root string) string {
	return filepath.Base(root)
}
