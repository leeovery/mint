package notescache

import (
	"fmt"
	"os"
	"path/filepath"

	"mint/internal/fsutil"
)

// chunkNotesPerm is the on-disk mode of a saved chunk-notes file, which the operator
// opens, tidies and releases.
const chunkNotesPerm = 0o644

// ChunkNotes keeps a failed chunked run's finished chunk notes, one file per repo and
// tag, under a base directory of its own so the note cache's pruning never reaches
// them.
type ChunkNotes struct {
	base string
}

// NewChunkNotes builds a ChunkNotes rooted at base: the user cache dir's
// mint/chunk-notes in production, a t.TempDir() in tests.
func NewChunkNotes(base string) *ChunkNotes {
	return &ChunkNotes{base: base}
}

// Save writes notes to {base}/{repo path, separators as dashes}/{tag}.md, replacing
// any file an earlier run saved for the tag, and returns the file's path.
func (c *ChunkNotes) Save(repoRoot, tag, notes string) (string, error) {
	path := filepath.Join(c.base, repoScope(repoRoot), tag+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating chunk notes dir: %w", err)
	}
	if err := fsutil.WriteFile(path, []byte(notes), chunkNotesPerm); err != nil {
		return "", fmt.Errorf("writing chunk notes: %w", err)
	}
	return path, nil
}
