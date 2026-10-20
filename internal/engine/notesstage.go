package engine

import (
	"fmt"
	"strconv"

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

func startNotesStage(p presenter.Presenter) *notesStage {
	return &notesStage{p: p, succeeded: startBlockingStage(p, "notes", "generating release notes…")}
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
		return "once this release is out, put them on it with mint release regenerate " + version + " --notes-file " + saved +
			", or if the run ends before the push, release them with mint release --notes-file " + saved
	})
}

// warnDryRunFallback names only the --notes-file release for saved chunk notes: a dry
// run never puts the version out. Call it only once the stage has ended.
func (s *notesStage) warnDryRunFallback() {
	s.warnFallbackReleasing(func(saved string) string {
		return "release them with mint release --notes-file " + saved
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
	if !s.halved || s.parts == 0 {
		return 0
	}
	return s.largestCall
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
	s.succeeded(fmt.Sprintf("generated in %d parts, merged", s.parts), fmt.Sprintf("Notes written (%d parts, merged)", s.parts))
}
