package notes_test

import (
	"context"
	"encoding/json"
	"testing"

	"mint/internal/notes"
)

// nothingToList is the line a part returns when its slice holds nothing the notes
// would list, as the AI might write it.
const nothingToList = "This part holds nothing for the notes."

// nothingInPartTwo answers each chunk call of a three-part run with its part's notes,
// part two with its nothing-to-list line, and fails the merge.
func nothingInPartTwo(_ context.Context, prompt string) (string, error) {
	switch {
	case isClassifyPrompt(prompt):
		return "OTHER", nil
	case isChunkPrompt(prompt):
		if path := firstPath(sliceOf(prompt)); path != area(2) {
			return chunkNotesFor(path), nil
		}
		return nothingToList, nil
	default:
		return "", failedWith("Invalid API key · Please run /login")
	}
}

func TestGenerator_MergeFailsAfterAPartReturnedItsNothingToListLine_SavesThatLineAmongThePartsNotes(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))

	ns, err := s.generateSaving(t, transportFunc(nothingInPartTwo))

	if got := causeOf(err); got != "AI call failed (merging)" {
		t.Errorf("CauseText = %q, want the merge's failure", got)
	}
	ns.assertSavedOnce(t, err, chunkNotesFor(area(1))+"\n\n"+nothingToList+"\n\n"+chunkNotesFor(area(3)))
}

func TestKeepPartNotes_ReMergeFailsOverAPartsNothingToListLine_SavesThatLineAmongThePartsNotes(t *testing.T) {
	t.Parallel()

	partNotes := []string{"### Added\n- export to CSV\n", nothingToList, "### Fixed\n- login retries\n"}
	doc, err := json.Marshal(map[string]any{"instructions": "notes format", "change_map": "map", "part_notes": partNotes})
	if err != nil {
		t.Fatal(err)
	}
	var in notes.MergeInputs
	if err := json.Unmarshal(doc, &in); err != nil {
		t.Fatal(err)
	}
	ns := &noteSaver{}

	failure := notes.KeepPartNotes(failedWith("Invalid API key · Please run /login"), in, ns.save)

	ns.assertSavedOnce(t, failure, "### Added\n- export to CSV\n"+"\n"+nothingToList+"\n\n"+"### Fixed\n- login retries\n")
}

func TestGenerator_APartReturnedItsNothingToListLineAndAnotherFailedForGood_MarkerListsOnlyThePartWithNoNotes(t *testing.T) {
	t.Parallel()

	s := capOneFilePerChunk(t, tenKBFiles(3))
	transport := transportFunc(func(_ context.Context, prompt string) (string, error) {
		switch {
		case isClassifyPrompt(prompt):
			return "OTHER", nil
		case isChunkPrompt(prompt):
			switch path := firstPath(sliceOf(prompt)); path {
			case area(1):
				return nothingToList, nil
			case area(3):
				return "", failedWith("API Error: usage limit reached for this account")
			default:
				return chunkNotesFor(path), nil
			}
		default:
			return "merged", nil
		}
	})

	ns, err := s.generateSaving(t, transport)

	if got := causeOf(err); got != "AI call failed (part 3 of 3)" {
		t.Errorf("CauseText = %q, want part 3's failure", got)
	}
	ns.assertSavedOnce(t, err, notesMissing("`"+area(3)+"`")+nothingToList+"\n\n"+chunkNotesFor(area(2)))
}
