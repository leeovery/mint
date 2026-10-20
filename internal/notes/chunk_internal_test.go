package notes

import (
	"errors"
	"strings"
	"testing"
)

func TestCutParts_RoomForChunksButNoneForAPieceAfterItsHeader_NoRoomForDiff(t *testing.T) {
	t.Parallel()

	file := "diff --git a/big.go b/big.go\n@@ -1 +1,9 @@\n" + strings.Repeat("+"+strings.Repeat("x", 63)+"\n", 200)
	maxParts := len(file)
	chunkFixed := len(composeChunkPrompt(DefaultPrompt, "", chunkPart{}.framing(maxParts, maxParts), ""))
	// A whole-file chunk has 10 bytes of room: less than the file, and far less than
	// a piece's longer framing needs.
	maxPromptBytes := chunkFixed + 10

	_, err := newCutter(DefaultPrompt, "", []string{file}).cutParts([]string{file}, maxPromptBytes)

	if !errors.Is(err, ErrNoRoomForDiff) {
		t.Errorf("cutParts error = %v, want ErrNoRoomForDiff", err)
	}
}

func TestCutParts_ChunkFixedPartFillsTheCap_NoRoomForDiff(t *testing.T) {
	t.Parallel()

	file := "diff --git a/a.go b/a.go\n@@ -1 +1 @@\n+x\n"
	maxParts := len(file)
	chunkFixed := len(composeChunkPrompt(DefaultPrompt, "", chunkPart{}.framing(maxParts, maxParts), ""))

	_, err := newCutter(DefaultPrompt, "", []string{file}).cutParts([]string{file}, chunkFixed)

	if !errors.Is(err, ErrNoRoomForDiff) {
		t.Errorf("cutParts error = %v, want ErrNoRoomForDiff", err)
	}
}

func TestCutParts_OneByteOfRoomForAOneByteFile_CutsOneChunk(t *testing.T) {
	t.Parallel()

	const file = "x"
	maxParts := len(file)
	chunkFixed := len(composeChunkPrompt(DefaultPrompt, "", chunkPart{}.framing(maxParts, maxParts), ""))

	parts, err := newCutter(DefaultPrompt, "", []string{file}).cutParts([]string{file}, chunkFixed+1)

	if err != nil {
		t.Fatalf("cutParts returned unexpected error: %v", err)
	}
	if len(parts) != 1 || parts[0].slice != file {
		t.Errorf("parts = %+v, want one chunk carrying the file", parts)
	}
}

func TestCutParts_PieceRoomSmallerThanACharacter_NoRoomForDiff(t *testing.T) {
	t.Parallel()

	const path = "accents.txt"
	file := "diff --git a/" + path + " b/" + path + "\n@@ -1 +1,9 @@\n" + strings.Repeat("+é\n", 200)
	header, _ := splitFileHeader(file)
	maxParts := len(file)
	pieceFixed := len(composeChunkPrompt(DefaultPrompt, "", chunkPart{file: path, piece: maxParts, pieces: maxParts}.framing(maxParts, maxParts), ""))
	// One byte of body room per piece: too little for the two-byte é.
	maxPromptBytes := pieceFixed + len(header) + 1

	parts, err := newCutter(DefaultPrompt, "", []string{file}).cutParts([]string{file}, maxPromptBytes)

	if !errors.Is(err, ErrNoRoomForDiff) {
		t.Errorf("cutParts error = %v with %d parts, want ErrNoRoomForDiff", err, len(parts))
	}
}
