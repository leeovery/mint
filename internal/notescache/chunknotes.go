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
	base func() (string, error)
}

// NewChunkNotes builds a ChunkNotes rooted at base.
func NewChunkNotes(base string) *ChunkNotes {
	return &ChunkNotes{base: func() (string, error) { return base, nil }}
}

// NewUserChunkNotes builds a ChunkNotes rooted at the user cache dir's
// mint/chunk-notes, resolving the dir through userCacheDir (os.UserCacheDir in
// production) only when a save is due. Save returns a resolution error verbatim.
func NewUserChunkNotes(userCacheDir func() (string, error)) *ChunkNotes {
	return &ChunkNotes{base: func() (string, error) {
		dir, err := userCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "mint", "chunk-notes"), nil
	}}
}

// Save writes notes to {base}/{repo path, separators as dashes}/{tag}.md, replacing
// any file an earlier run saved for the tag, and returns the file's path.
func (c *ChunkNotes) Save(repoRoot, tag, notes string) (string, error) {
	base, err := c.base()
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, repoScope(repoRoot), tag+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("creating chunk notes dir: %w", err)
	}
	if err := fsutil.WriteFile(path, []byte(notes), chunkNotesPerm); err != nil {
		return "", fmt.Errorf("writing chunk notes: %w", err)
	}
	return path, nil
}
