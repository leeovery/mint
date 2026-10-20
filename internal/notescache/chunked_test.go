package notescache_test

import (
	"encoding/json"
	"testing"
	"time"

	"mint/internal/notes"
	"mint/internal/notescache"
)

// mergeInputsOf decodes doc into the merge inputs a chunked run would have kept.
func mergeInputsOf(t *testing.T, doc string) notes.MergeInputs {
	t.Helper()
	var in notes.MergeInputs
	if err := json.Unmarshal([]byte(doc), &in); err != nil {
		t.Fatalf("decoding merge inputs: %v", err)
	}
	return in
}

func TestStore_LookupOfAChunkedEntry_ReturnsItsMergeInputsWithItsBody(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := notescache.NewStore(t.TempDir(), func() time.Time { return fixedClock })
	merge := mergeInputsOf(t, `{"instructions":"PROMPT","change_map":"MAP","part_notes":["- part 1","- part 2","- part 3"]}`)

	if err := store.Write(repoRoot, "chunked", "merged body", &merge); err != nil {
		t.Fatalf("Write returned unexpected error: %v", err)
	}
	entry, found, err := store.Lookup(repoRoot, "chunked")
	if err != nil || !found {
		t.Fatalf("Lookup = (found %v, err %v), want a live entry", found, err)
	}

	if entry.Body != "merged body" {
		t.Errorf("entry Body = %q, want %q", entry.Body, "merged body")
	}
	if entry.Merge == nil {
		t.Fatalf("entry Merge = nil, want the chunked run's merge inputs")
	}
	got, err := json.Marshal(entry.Merge)
	if err != nil {
		t.Fatalf("encoding the looked-up merge inputs: %v", err)
	}
	want, _ := json.Marshal(merge)
	if string(got) != string(want) {
		t.Errorf("looked-up merge inputs = %s, want those written %s", got, want)
	}
}

func TestStore_LookupOfASinglePassEntry_CarriesNoMergeInputs(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	store := notescache.NewStore(t.TempDir(), func() time.Time { return fixedClock })

	if err := store.Write(repoRoot, "single", "single-pass body", nil); err != nil {
		t.Fatalf("Write returned unexpected error: %v", err)
	}
	entry, found, err := store.Lookup(repoRoot, "single")
	if err != nil || !found {
		t.Fatalf("Lookup = (found %v, err %v), want a live entry", found, err)
	}

	if entry.Merge != nil {
		t.Errorf("entry Merge = %+v, want none for single-pass notes", entry.Merge)
	}
}
