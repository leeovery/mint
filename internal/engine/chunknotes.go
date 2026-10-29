package engine

import (
	"context"

	"mint/internal/config"
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
// saved, and names a saved file with command, the run that releases it.
func (k *chunkNotesKeeper) reportKept(p presenter.Presenter, cause error, command string) {
	k.warnUnsaved(p)
	if saved := notes.SavedChunkNotesPath(cause); saved != "" {
		p.NextMoves(presenter.NextMoves{Stage: "regenerate", Moves: []presenter.NextMove{notesFileMove(command, saved)}})
	}
}

// reviewGate runs the review gate with regen as its `r`, then reports what an abort
// kept, a saved file named with command.
func (k *chunkNotesKeeper) reviewGate(ctx context.Context, deps ReleaseDeps, regen reviewRegen, gate presenter.Gate, body, versionKey, command string) (string, error) {
	body, err := reviewGate(ctx, deps, regen, gate, body, versionKey)
	if err != nil {
		k.reportKept(deps.Presenter, err, command)
	}
	return body, err
}

// regenRun is what an `r` on the run's notes regenerates from, its chunked run
// reporting to progress and kept here. Its Merged reads the keeper's merge inputs live,
// so the `r` re-merges from the same inputs keepRegen saves from.
func (k *chunkNotesKeeper) regenRun(progress notes.Progress) RegenRun {
	return RegenRun{Merged: k.heldMerged, NotesSinks: k.sinks(progress)}
}

func (k *chunkNotesKeeper) heldMerged() *notes.MergeInputs {
	return k.merged
}

// regenerateFunc is a verb's size-check generation for an `r`: generator, wired to the
// run's sinks, re-runs the AI path with oneTimeContext appended.
type regenerateFunc func(ctx context.Context, generator *notes.Generator, oneTimeContext string) (string, error)

// regenerator is the review gate's `r` rule. While run holds merge inputs, those of the
// chunked run that wrote the notes now at the gate, an `r` makes one merge call over
// those part notes with only its own one-time context in the release's instructions.
// Otherwise it runs regenerate, the verb's size-check generation, over generator wired
// to run's sinks; of a chunked run that `r` becomes, they save a failed one's finished
// chunk notes and keep a written one's merge inputs for the next `r`.
func (run RegenRun) regenerator(generator *notes.Generator, cfg config.Config, regenerate regenerateFunc) regeneratorFunc {
	wired := run.wire(generator)
	return func(ctx context.Context, oneTimeContext string) (string, error) {
		if run.Merged != nil {
			if in := run.Merged(); in != nil {
				return wired.Remerge(ctx, *in, cfg, oneTimeContext)
			}
		}
		return regenerate(ctx, wired, oneTimeContext)
	}
}
