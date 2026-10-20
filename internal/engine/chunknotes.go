package engine

import (
	"mint/internal/notes"
	"mint/internal/presenter"
)

// chunkNotesKeeper saves a failed chunked run's finished chunk notes for one repo's
// tag and remembers why a save failed, so the run can warn once its notes stage has
// ended.
type chunkNotesKeeper struct {
	store     ChunkNotesSaver
	root, tag string
	saveErr   error
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
		return path, err
	}
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
