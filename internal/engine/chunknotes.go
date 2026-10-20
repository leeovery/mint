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

// warnUnsaved warns, with the save's error verbatim beneath, when finished chunk notes
// were due to be saved and could not be. Call it only once the notes stage has ended,
// so the warning follows the line that ends it.
func (k *chunkNotesKeeper) warnUnsaved(p presenter.Presenter) {
	if k.saveErr == nil {
		return
	}
	p.Warn(presenter.Warning{
		Label:   "notes",
		Message: "the finished chunk notes could not be saved",
		Output:  k.saveErr.Error(),
	})
}
