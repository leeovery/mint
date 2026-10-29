package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"mint/internal/config"
	"mint/internal/notes"
	"mint/internal/presenter"
)

// notesStage narrates the blocking notes stage. As the notes.Progress of the
// generation it runs around, it relays a chunked run's advance to the presenter, and
// names the part count on the completion line only once a merge has written the
// body; any other body completes as "Generated release notes".
type notesStage struct {
	p     presenter.Presenter
	stage *blockingStage
	// checking is when the exclusion check's call started, and checked how long it took.
	checking time.Time
	checked  time.Duration
	// checkSettled reports that the check's node was filled: its call ended, it
	// failed, or the keep list left it nothing to call about.
	checkSettled bool
	parts        int
	halved       bool
	// largestCall is the biggest prompt of the run's succeeded calls carrying diff.
	largestCall int
	// fallbackCause is the failure the commit-subject list ships in place of, or nil.
	fallbackCause error
	// suggestions are what the exclusion check reported on an unattended run.
	suggestions []presenter.ExclusionSuggestion
}

var (
	_ notes.Progress       = (*notesStage)(nil)
	_ notes.CheckNarration = (*notesStage)(nil)
)

const notesStageText = "generating release notes…"

func startNotesStage(p presenter.Presenter, now func() time.Time) *notesStage {
	return &notesStage{p: p, stage: startBlockingStage(p, now, stageNotes, notesStageText)}
}

// Checking narrates the exclusion check's call starting and times it, so Checked can
// narrate its end with the call's elapsed; the pauses after it are not timed.
func (s *notesStage) Checking() {
	s.checking = s.stage.now()
	s.p.NotesChecking(presenter.NotesCheck{Key: stageCheck})
}

// Checked narrates the check's call ending, its result stated where the answer flagged
// nothing; whatever it flagged has its result stated by Resolved once its pauses or
// its report are done, and an unusable call's warning, next, ends the check in place
// of a success.
func (s *notesStage) Checked(outcome notes.CheckOutcome) {
	s.checkSettled = true
	s.checked = s.stage.now().Sub(s.checking)
	check := presenter.NotesCheck{Key: stageCheck, Elapsed: s.checked}
	switch outcome {
	case notes.CheckClear:
		check.Detail = "nothing to leave out"
	case notes.CheckUnusable:
		check.Failed = true
	}
	s.p.NotesChecked(check)
}

// Resolved states what the check's suggestions came to on its row, in the call's time,
// Quiet: plain has already said it, a line per answer or per suggestion reported.
func (s *notesStage) Resolved(result notes.CheckResult) {
	success := presenter.Settled(stageCheck, checkResultText(result))
	success.Blocking, success.Elapsed = true, s.checked
	s.p.StageSucceeded(success)
}

// checkResultText is what the check's suggestions came to: the one answer, as it hangs
// under the check while the pauses run; for several, how many were left out and how
// many kept; for those reported, how many were flagged.
func checkResultText(result notes.CheckResult) string {
	switch len(result.Answered) {
	case 0:
		if result.Flagged > 0 {
			return fmt.Sprintf("%d flagged, not asked", result.Flagged)
		}
		return "nothing to leave out"
	case 1:
		a := result.Answered[0]
		return a.Glob + " " + answerWord(a.Answer) + " · " + a.Reason
	}
	excluded := 0
	for _, a := range result.Answered {
		if a.Answer == notes.AnswerExclude {
			excluded++
		}
	}
	var counts []string
	if excluded > 0 {
		counts = append(counts, fmt.Sprintf("%d left out", excluded))
	}
	if kept := len(result.Answered) - excluded; kept > 0 {
		counts = append(counts, fmt.Sprintf("%d kept", kept))
	}
	return strings.Join(counts, " · ")
}

func answerWord(answer notes.Answer) string {
	if answer == notes.AnswerExclude {
		return "excluded"
	}
	return "kept"
}

// AllKept settles the check's node quietly: the keep list covers every changed path.
func (s *notesStage) AllKept() {
	s.skipCheck("every changed path is on the keep list")
}

// settleUncheckedCheck settles the check's node quietly when the release planned it
// and its notes resolved without it, so no planned node is left hollow. Every way out
// of a check that ran settles it, and the keep list covering every path settles it
// too, so what is left is a diff with no source changes, which the check never sees.
func (s *notesStage) settleUncheckedCheck(planned bool) {
	if planned && !s.checkSettled {
		s.skipCheck("no source changes to check")
	}
}

func (s *notesStage) skipCheck(why string) {
	s.checkSettled = true
	s.p.StageSucceeded(presenter.Settled(stageCheck, "skipped: "+why))
}

// failure is the StageFailed payload for the notes stage failing on cause, timed.
func (s *notesStage) failure(cause error) presenter.StageFailure {
	return s.stage.failure(cause)
}

func (s *notesStage) CheckFailed(output string) {
	s.checkSettled = true
	s.p.SuspendSpinner()
	s.p.Warn(presenter.Warning{
		Key:     stageCheck,
		Label:   "notes",
		Message: "couldn't check for files that don't belong in the notes",
		Output:  output,
	})
	s.p.ResumeSpinner()
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

func (s *notesStage) suggested(suggestion notes.Suggestion, m notes.Matches) {
	s.suggestions = append(s.suggestions, presenter.ExclusionSuggestion{
		Glob:    suggestion.Glob,
		Reason:  suggestion.Reason,
		Matches: describeMatches(m),
	})
}

// showSuggestions prints the suggestions an unattended run's check reported, each with the
// [release].diff_exclude line that would exclude it: releaseExclude, the key as
// loaded, with its glob appended.
func (s *notesStage) showSuggestions(releaseExclude []string) {
	if len(s.suggestions) == 0 {
		return
	}
	shown := slices.Clone(s.suggestions)
	for i := range shown {
		shown[i].Line = diffExcludeLine(releaseExclude, shown[i].Glob)
	}
	s.p.ExclusionSuggestions(presenter.ExclusionSuggestions{Stage: "notes", Table: releaseTable, File: config.FileName, Suggestions: shown})
}

// diffExcludeLine is the diff_exclude key holding entries with glob appended.
func diffExcludeLine(entries []string, glob string) string {
	return config.ListLine("diff_exclude", append(slices.Clip(entries), glob))
}

// warnFallback names both commands for saved chunk notes, the release again being the
// run's own: when it prints, whether this release will go out is not yet known. Call
// it only once the stage has ended.
func (s *notesStage) warnFallback(again rerun, version string) {
	s.warnFallbackReleasing(func(saved string) string {
		return "once this release is out, put them on it with " + notesFileCommand(regenerateRerun(version).command, saved) +
			", or if the run ends before the push, release them with " + notesFileCommand(again.command, saved)
	})
}

// warnDryRunFallback names only the --notes-file release for saved chunk notes, the
// run's own again: a dry run never puts the version out. Call it only once the stage
// has ended.
func (s *notesStage) warnDryRunFallback(again rerun) {
	s.warnFallbackReleasing(func(saved string) string {
		return "release them with " + notesFileCommand(again.command, saved)
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

// regenStage is the notes.Progress of the `r` generations at a run's review gate. Each
// r runs the notes stage again: Quiet while it makes one call, or one merge call over
// the part notes it re-merges, for which plain has never had a line; one that becomes a
// chunked run opens its own notes line at its first update and narrates as the run's
// notes stage does. An r's succeeded calls fold into the run's notes stage, so the cap
// suggested after an r is the largest succeeded call of the whole run.
type regenStage struct {
	run    *notesStage
	halved bool
	parts  int
	// hint is handed the cap to suggest once an r's notes are shown, 0 for none.
	hint func(lowerCap int)
	// quiet is the r now generating, as the Quiet notes stage it starts as.
	quiet *blockingStage
	// stage is the notes line the r now generating opened, nil while it has none.
	stage *blockingStage
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
		r.quiet, r.stage = startQuietStage(r.run.p, r.run.stage.now, stageNotes, notesStageText), nil
		body, err := regenerator.Regenerate(ctx, oneTimeContext)
		if err == nil {
			r.done()
		}
		return body, err
	})
}

// done narrates the r's notes written: on the notes line it opened, or Quiet where it
// opened none, as the one call that wrote them or the merge of the saved part notes
// it re-merged.
func (r *regenStage) done() {
	switch {
	case r.stage != nil:
		r.stage.succeeded(mergedCompletion(r.parts))
	case r.parts > 0:
		r.quiet.succeeded(presenter.StageSuccess{
			Detail:   fmt.Sprintf("merged from %d parts", r.parts),
			Sentence: fmt.Sprintf("Notes merged again (%d parts)", r.parts),
		})
	default:
		r.quiet.succeeded(notesCompletion(0))
	}
}

// chunked opens the r's notes line, timed from the r's start, unless it is open.
func (r *regenStage) chunked() {
	if r.stage == nil {
		r.stage = startBlockingStageSince(r.run.p, r.quiet.now, stageNotes, notesStageText, r.quiet.started)
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
	s.stage.succeeded(notesCompletion(s.parts))
}

// doneFrom narrates the stage's success where its body came from the source kind
// names: the rail's row names a source nothing generated, notesFile's name for a
// supplied body; plain keeps the detail it has always had.
func (s *notesStage) doneFrom(kind notes.Kind, notesFile string) {
	success := notesCompletion(s.parts)
	success.Row = notesSource(kind, notesFile)
	s.stage.succeeded(success)
}

// notesSource names where a body nothing generated came from, "" for one the AI wrote.
func notesSource(kind notes.Kind, notesFile string) string {
	switch kind {
	case notes.KindSupplied:
		return "read from " + filepath.Base(notesFile)
	case notes.KindNoAI, notes.KindFallback:
		return "the commit-subject list"
	case notes.KindFirstRelease:
		return "the first release's default"
	case notes.KindDegenerate:
		return "the stub for a release with no source changes"
	}
	return ""
}

// notesCompletion is the success of a notes stage whose body was written in one call,
// or, by a merge, from parts.
func notesCompletion(parts int) presenter.StageSuccess {
	if parts == 0 {
		return presenter.StageSuccess{Detail: "generated", Sentence: "Generated release notes"}
	}
	return mergedCompletion(parts)
}

// mergedCompletion is the success of a notes stage whose merge wrote the body from
// parts.
func mergedCompletion(parts int) presenter.StageSuccess {
	return presenter.StageSuccess{
		Detail:   fmt.Sprintf("generated in %d parts, merged", parts),
		Sentence: fmt.Sprintf("Notes written (%d parts, merged)", parts),
	}
}
