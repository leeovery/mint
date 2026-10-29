package engine

import (
	"errors"
	"testing"

	"mint/internal/presenter"
)

// savingChunkNotes saves to path, or fails with err.
type savingChunkNotes struct {
	path  string
	err   error
	saved *string
}

func (s savingChunkNotes) Save(_, _, notes string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	*s.saved = notes
	return s.path, nil
}

// TestHealMove_WritesTheReleaseFromTheNotesThePushedChangelogHolds pins the heal a
// post-push publish failure names: the run's own source given explicitly, read
// again; a supplied body's notes file; a floor body saved where chunk notes go; and,
// where it can't be saved, a file the reader writes.
func TestHealMove_WritesTheReleaseFromTheNotesThePushedChangelogHolds(t *testing.T) {
	t.Parallel()

	const body = "Initial release.\n"
	var saved string
	for _, tc := range []struct {
		name   string
		source RegenerateSource
		notes  ChunkNotesSaver
		want   presenter.NextMove
	}{
		{
			name:   "fresh notes, written fresh again",
			source: RegenerateSourceFresh,
			want:   presenter.NextMove{Lead: "Re-heal the GitHub release:", Command: "mint release regenerate v1.4.0 --source fresh --target release"},
		},
		{
			name:   "the tag's notes, read again",
			source: RegenerateSourceTag,
			want:   presenter.NextMove{Lead: "Re-heal the GitHub release:", Command: "mint release regenerate v1.4.0 --source tag --target release"},
		},
		{
			name:   "a supplied body, from its file",
			source: regenerateSourceSupplied,
			want:   presenter.NextMove{Lead: "Re-heal the GitHub release:", Command: "mint release regenerate v1.4.0 --target release --notes-file 'my notes.md'"},
		},
		{
			name:   "a floor body, saved",
			source: regenerateSourceFloor,
			notes:  savingChunkNotes{path: "/cache/v1.4.0.md", saved: &saved},
			want: presenter.NextMove{
				Lead:    "Re-heal the GitHub release from its notes, saved to /cache/v1.4.0.md:",
				Command: "mint release regenerate v1.4.0 --target release --notes-file /cache/v1.4.0.md",
			},
		},
		{
			name:   "a floor body that can't be saved",
			source: regenerateSourceFloor,
			notes:  savingChunkNotes{err: errors.New("no space left on device")},
			want: presenter.NextMove{
				Lead:    "Write the notes CHANGELOG.md now holds to a file, then re-heal the GitHub release with it:",
				Command: "mint release regenerate v1.4.0 --target release --notes-file <file>",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := RegenerateWriteRequest{Source: tc.source, Target: RegenerateTargetBoth, Tag: "v1.4.0", NotesFile: "my notes.md"}
			got := healMove(ReleaseDeps{ChunkNotes: tc.notes}, "/repo", req, body, "GitHub release")
			if got != tc.want {
				t.Errorf("heal = %+v, want %+v", got, tc.want)
			}
		})
	}
	if saved != body {
		t.Errorf("saved notes = %q, want the body the changelog holds, %q", saved, body)
	}
}
