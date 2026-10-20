package engine

import (
	"mint/internal/notes"
	"mint/internal/presenter"
)

// chunkNotesKeeper keeps a run's chunk notes for one repo's tag: a failed chunked
// run's finished notes it saves, remembering what it saved and why a save failed, and
// a written run's it holds with the rest of what their merge was composed from.
type chunkNotesKeeper struct {
	store     ChunkNotesSaver
	root, tag string
	saved     string
	saveErr   error
	// merged is nil unless the run's notes were written by a chunked run.
	merged *notes.MergeInputs
}

func newChunkNotesKeeper(store ChunkNotesSaver, root, tag string) *chunkNotesKeeper {
	return &chunkNotesKeeper{store: store, root: root, tag: tag}
}

// saver is the generation's chunk-notes save, or nil, keeping none, when no store is
// wired.
func (k *chunkNotesKeeper) saver() notes.SaveChunkNotes {
	if k.store == nil {
		return nil
	}
	return func(chunkNotes string) (string, error) {
		path, err := k.store.Save(k.root, k.tag, chunkNotes)
		k.saveErr = err
		if err == nil {
			k.saved = chunkNotes
		}
		return path, err
	}
}

func (k *chunkNotesKeeper) keepMerged(in notes.MergeInputs) {
	k.merged = &in
}

func (k *chunkNotesKeeper) sinks(progress notes.Progress) NotesSinks {
	return NotesSinks{Progress: progress, ChunkNotes: k.saver(), MergeInputs: k.keepMerged}
}

// warnUnsaved warns once, with the save's error verbatim beneath, when finished chunk
// notes were due to be saved and could not be. Call it only once the line that ends
// the stage whose save failed is shown, so the warning follows it.
func (k *chunkNotesKeeper) warnUnsaved(p presenter.Presenter) {
	if k.saveErr == nil {
		return
	}
	p.Warn(presenter.Warning{
		Label:   "notes",
		Message: "the finished chunk notes could not be saved",
		Output:  k.saveErr.Error(),
	})
	k.saveErr = nil
}

// keepRegen saves the part notes of the chunked notes at the review gate when an `r`
// on them fails, returning failure naming the saved file.
func (k *chunkNotesKeeper) keepRegen(failure error) error {
	if k.merged == nil {
		return failure
	}
	return notes.KeepPartNotes(failure, *k.merged, k.saver())
}

// reportKept warns where the chunk notes of a review-gate abort, cause, could not be
// saved, and names a saved file with the command that releases it.
func (k *chunkNotesKeeper) reportKept(p presenter.Presenter, cause error) {
	k.warnUnsaved(p)
	if saved := notes.SavedChunkNotesPath(cause); saved != "" {
		p.NextMoves(presenter.NextMoves{Stage: "regenerate", Moves: []string{notesFileMove("mint release", saved)}})
	}
}
