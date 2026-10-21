package engine

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"mint/internal/notes"
	"mint/internal/presenter"
)

// notesStage narrates the blocking notes stage. As the notes.Progress of the
// generation it runs around, it relays a chunked run's advance to the presenter, and
// names the part count on the completion line only once a merge has written the
// body; any other body completes as "Generated release notes".
type notesStage struct {
	p         presenter.Presenter
	succeeded func(detail, sentence string)
	parts     int
	halved    bool
	// largestCall is the biggest prompt of the run's succeeded calls carrying diff.
	largestCall int
	// fallbackCause is the failure the commit-subject list ships in place of, or nil.
	fallbackCause error
}

var _ notes.Progress = (*notesStage)(nil)

const notesStageText = "generating release notes…"

func startNotesStage(p presenter.Presenter) *notesStage {
	return &notesStage{p: p, succeeded: startBlockingStage(p, "notes", notesStageText)}
}

func (s *notesStage) PartsDone(promptBytes, parts, done int) {
	s.p.NotesProgress(presenter.NotesProgress{PromptBytes: promptBytes, Parts: parts, Done: done})
}

func (s *notesStage) Halved(part, parts int) {
	s.halved = true
	s.p.NotesHalving(presenter.NotesHalving{Part: part, Parts: parts})
}

func (s *notesStage) Merging(parts int) {
	s.p.NotesMerging(presenter.NotesMerge{Parts: parts})
}

func (s *notesStage) Merged(parts int) {
	s.parts = parts
}

func (s *notesStage) CallSucceeded(promptBytes int) {
	s.largestCall = max(s.largestCall, promptBytes)
}

func (s *notesStage) fellBack(failure error) {
	s.fallbackCause = failure
}

// warnFallback names both commands for saved chunk notes: when it prints, whether this
// release will go out is not yet known. Call it only once the stage has ended.
func (s *notesStage) warnFallback(version string) {
	s.warnFallbackReleasing(func(saved string) string {
		return "once this release is out, put them on it with " + notesFileCommand("mint release regenerate "+version, saved) +
			", or if the run ends before the push, release them with " + notesFileCommand("mint release", saved)
	})
}

// warnDryRunFallback names only the --notes-file release for saved chunk notes: a dry
// run never puts the version out. Call it only once the stage has ended.
func (s *notesStage) warnDryRunFallback() {
	s.warnFallbackReleasing(func(saved string) string {
		return "release them with " + notesFileCommand("mint release", saved)
	})
}

func (s *notesStage) warnFallbackReleasing(release func(saved string) string) {
	if s.fallbackCause == nil {
		return
	}
	message := failureMessage(s.fallbackCause) + "; falling back to the commit-subject list"
	if saved := notes.SavedChunkNotesPath(s.fallbackCause); saved != "" {
		message += "; part notes saved to " + saved + " — " + release(saved)
	}
	s.p.Warn(presenter.Warning{Label: "notes", Message: message, Output: notesFailureOutput(s.fallbackCause)})
}

// lowerCap is the cap to suggest: the largest succeeded call carrying diff when a
// halved run's merge wrote its notes, else 0.
func (s *notesStage) lowerCap() int {
	return suggestedCap(s.halved, s.parts, s.largestCall)
}

func suggestedCap(halved bool, mergedParts, largestCall int) int {
	if !halved || mergedParts == 0 {
		return 0
	}
	return largestCall
}

// regenStage is the notes.Progress of the `r` generations at a run's review gate. An r
// that makes one call narrates nothing; one that becomes a chunked run opens its own
// notes line at its first update and narrates as the run's notes stage does. An r's
// succeeded calls fold into the run's notes stage, so the cap suggested after an r is
// the largest succeeded call of the whole run.
type regenStage struct {
	run    *notesStage
	halved bool
	parts  int
	// hint is handed the cap to suggest once an r's notes are shown, 0 for none.
	hint func(lowerCap int)
	// began is when the r now generating started.
	began time.Time
	// succeeded closes the notes line the r now generating opened, nil while it has none.
	succeeded func(detail, sentence string)
}

var _ notes.Progress = (*regenStage)(nil)

func (s *notesStage) regens(hint func(lowerCap int)) *regenStage {
	return &regenStage{run: s, hint: hint}
}

// review is the review gate's `r` over regenerator, narrated as one of these r's,
// with k saving the part notes of one that fails.
func (r *regenStage) review(regenerator Regenerator, k *chunkNotesKeeper) reviewRegen {
	return reviewRegen{regenerator: r.narrating(regenerator), shown: r.shown, keep: k.keepRegen}
}

// narrating wraps regenerator so the notes line an r opens closes once its notes are
// written, before they are shown. A nil regenerator stays nil.
func (r *regenStage) narrating(regenerator Regenerator) Regenerator {
	if regenerator == nil {
		return nil
	}
	return regeneratorFunc(func(ctx context.Context, oneTimeContext string) (string, error) {
		r.began, r.succeeded = time.Now(), nil
		body, err := regenerator.Regenerate(ctx, oneTimeContext)
		if err == nil && r.succeeded != nil {
			r.succeeded(mergedCompletion(r.parts))
		}
		return body, err
	})
}

// chunked opens the r's notes line, timed from the r's start, unless it is open.
func (r *regenStage) chunked() {
	if r.succeeded == nil {
		r.succeeded = startBlockingStageSince(r.run.p, "notes", notesStageText, r.began)
	}
}

func (r *regenStage) PartsDone(promptBytes, parts, done int) {
	r.chunked()
	r.run.p.NotesProgress(presenter.NotesProgress{PromptBytes: promptBytes, Parts: parts, Done: done})
}

func (r *regenStage) Halved(part, parts int) {
	r.halved = true
	r.chunked()
	r.run.p.NotesHalving(presenter.NotesHalving{Part: part, Parts: parts})
}

func (r *regenStage) Merging(parts int) {
	r.run.p.NotesMerging(presenter.NotesMerge{Parts: parts})
}

func (r *regenStage) Merged(parts int) {
	r.parts = parts
}

func (r *regenStage) CallSucceeded(promptBytes int) {
	r.run.CallSucceeded(promptBytes)
}

// shown hands hint the cap to suggest for the r whose notes were just shown, then
// forgets that r's halving, so the next r is judged on its own.
func (r *regenStage) shown() {
	lowerCap := suggestedCap(r.halved, r.parts, r.run.largestCall)
	r.halved, r.parts = false, 0
	r.hint(lowerCap)
}

// batchLowerCap gathers the one lower-the-cap hint of a batch from its versions' notes
// stages: the largest succeeded call carrying diff in any version, suggested once any
// version's notes were written after halving.
type batchLowerCap struct {
	largestCall int
	halved      bool
}

// tried folds in the calls of a version's generation, whether or not its notes were
// written.
func (b *batchLowerCap) tried(s *notesStage) {
	b.largestCall = max(b.largestCall, s.largestCall)
}

// written folds in a version whose notes were written.
func (b *batchLowerCap) written(s *notesStage) {
	b.tried(s)
	b.halved = b.halved || s.lowerCap() != 0
}

// regenWritten folds in an `r` whose notes were written, lowerCap being the cap it
// suggests; its calls fold in through its version's notes stage.
func (b *batchLowerCap) regenWritten(lowerCap int) {
	b.halved = b.halved || lowerCap != 0
}

func (b *batchLowerCap) lowerCap() int {
	if !b.halved {
		return 0
	}
	return b.largestCall
}

// hintLowerCap writes bytes exact, never rounded: a rounded value can sit above the
// largest call the model accepted.
func hintLowerCap(p presenter.Presenter, bytes int) {
	if bytes == 0 {
		return
	}
	p.Warn(presenter.Warning{
		Label:   "max_prompt_bytes",
		Message: "set higher than this model accepts; set max_prompt_bytes = " + strconv.Itoa(bytes) + ", the largest call that succeeded",
	})
}

// done narrates the stage's success. Call it only once the generation has returned.
func (s *notesStage) done() {
	if s.parts == 0 {
		s.succeeded("generated", "Generated release notes")
		return
	}
	s.succeeded(mergedCompletion(s.parts))
}

// mergedCompletion is the detail and sentence of a notes stage whose merge wrote the
// body from parts.
func mergedCompletion(parts int) (detail, sentence string) {
	return fmt.Sprintf("generated in %d parts, merged", parts), fmt.Sprintf("Notes written (%d parts, merged)", parts)
}
