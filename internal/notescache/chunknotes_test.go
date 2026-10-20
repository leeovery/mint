package notescache_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mint/internal/notescache"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestChunkNotes_Save_WritesTheTagsFileUnderTheRepoPathWithSeparatorsAsDashes(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	const saved = "- part one notes\n\n- part two notes"

	path, err := notescache.NewChunkNotes(base).Save("/Users/lee/Code/app", "v1.4.0", saved)
	if err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	if want := filepath.Join(base, "-Users-lee-Code-app", "v1.4.0.md"); path != want {
		t.Errorf("Save path = %q, want %q", path, want)
	}
	if got := readFile(t, path); got != saved {
		t.Errorf("saved file = %q, want the notes byte for byte %q", got, saved)
	}
}

func TestChunkNotes_Save_ALaterSaveForTheSameTagOverwritesTheFile(t *testing.T) {
	t.Parallel()

	store := notescache.NewChunkNotes(t.TempDir())
	if _, err := store.Save("/Users/lee/Code/app", "v1.4.0", "- the earlier run's notes\n- and more of them"); err != nil {
		t.Fatalf("first Save returned unexpected error: %v", err)
	}

	path, err := store.Save("/Users/lee/Code/app", "v1.4.0", "- the later run's notes")
	if err != nil {
		t.Fatalf("second Save returned unexpected error: %v", err)
	}

	if got := readFile(t, path); got != "- the later run's notes" {
		t.Errorf("saved file = %q, want only the later run's notes", got)
	}
}

func TestChunkNotes_Save_SavedFileOutlivesTheNoteCachePrune(t *testing.T) {
	t.Parallel()

	userCache := t.TempDir()
	const root = "/Users/lee/Code/app"
	path, err := notescache.NewChunkNotes(filepath.Join(userCache, "mint", "chunk-notes")).Save(root, "v1.4.0", "- saved notes")
	if err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}
	cache := notescache.NewStore(filepath.Join(userCache, "mint", "cache"), func() time.Time { return fixedClock })
	if err := cache.Write(root, "keep", "previewed body"); err != nil {
		t.Fatalf("cache Write returned unexpected error: %v", err)
	}

	cache.Prune(root, "keep")

	if got := readFile(t, path); got != "- saved notes" {
		t.Errorf("saved file after the cache prune = %q, want it untouched", got)
	}
}

func TestUserChunkNotes_Save_WritesUnderTheUserCacheDirsMintChunkNotes(t *testing.T) {
	t.Parallel()

	userCache := t.TempDir()
	store := notescache.NewUserChunkNotes(func() (string, error) { return userCache, nil })

	path, err := store.Save("/Users/lee/Code/app", "v1.4.0", "- saved notes")
	if err != nil {
		t.Fatalf("Save returned unexpected error: %v", err)
	}

	if want := filepath.Join(userCache, "mint", "chunk-notes", "-Users-lee-Code-app", "v1.4.0.md"); path != want {
		t.Errorf("Save path = %q, want %q", path, want)
	}
	if got := readFile(t, path); got != "- saved notes" {
		t.Errorf("saved file = %q, want the notes byte for byte", got)
	}
}

func TestUserChunkNotes_Save_AnUnresolvableUserCacheDirReturnsItsErrorVerbatim(t *testing.T) {
	t.Parallel()

	unresolved := errors.New("neither $XDG_CACHE_HOME nor $HOME are defined")
	store := notescache.NewUserChunkNotes(func() (string, error) { return "", unresolved })

	path, err := store.Save("/Users/lee/Code/app", "v1.4.0", "- saved notes")

	if path != "" || err == nil || err.Error() != unresolved.Error() {
		t.Errorf("Save = (%q, %v), want no path and the resolution error %q verbatim", path, err, unresolved)
	}
}
