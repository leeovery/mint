package notes

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// fileDiffHeader opens each file's section of a git diff.
const fileDiffHeader = "diff --git "

// generateChunked writes the notes for a diff whose single-pass prompt is over
// maxPromptBytes: the diff's files are cut in path order into chunk calls, each
// carrying the release's instructions and whole-release Change Map and run
// concurrently, then one merge call turns every chunk's notes, in chunk order, into
// the body. The merge output is returned byte-identical. promptBytes is the
// single-pass prompt's size, reported to g.progress with the run's advance. No call
// over maxPromptBytes is sent: ErrNoRoomForDiff before any call, ErrMergeTooLarge in
// place of the merge.
func (g *Generator) generateChunked(ctx context.Context, instructions, changeMap, diff string, promptBytes, maxPromptBytes int) (string, error) {
	parts, err := cutParts(instructions, changeMap, splitFileDiffs(diff), maxPromptBytes)
	if err != nil {
		return "", err
	}

	partNotes, err := g.generateParts(ctx, instructions, changeMap, parts, promptBytes)
	if err != nil {
		return "", err
	}

	mergePrompt := composeMergePrompt(instructions, changeMap, partNotes)
	if len(mergePrompt) > maxPromptBytes {
		return "", ErrMergeTooLarge
	}
	g.progress.Merging(len(parts))
	body, err := g.transport.Generate(ctx, mergePrompt)
	if err != nil {
		return "", fmt.Errorf("merging part notes: %w", err)
	}
	g.progress.Merged(len(parts))
	return body, nil
}

// maxChunkCallsInFlight bounds a wave's concurrent chunk calls, so a large release
// never lands all of its calls on the machine and the provider's limits at once.
const maxChunkCallsInFlight = 4

// generateParts makes one chunk call per part, at most maxChunkCallsInFlight at a
// time, and returns their notes in part order once every call has finished. Each call
// runs on ctx itself: the wave sets no deadline of its own, so a timeout is per call.
// The first call to fail stops the wave: no further call starts, the calls already
// running finish, and that failure is returned. ctx ending stops it too, and its error
// is returned over any earlier failure. Each part that succeeds is reported to
// g.progress as it finishes.
func (g *Generator) generateParts(ctx context.Context, instructions, changeMap string, parts []chunkPart, promptBytes int) ([]string, error) {
	partNotes := make([]string, len(parts))
	slots := make(chan struct{}, maxChunkCallsInFlight)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	done := 0
	stopped := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return firstErr != nil || ctx.Err() != nil
	}
	g.progress.PartsDone(promptBytes, len(parts), done)
	for i, part := range parts {
		slots <- struct{}{}
		if stopped() {
			<-slots
			break
		}
		wg.Go(func() {
			// Released last, after the outcome is recorded, so the wave sees a failure
			// before it can start a call in the freed slot.
			defer func() { <-slots }()
			prompt := composeChunkPrompt(instructions, changeMap, part.framing(i+1, len(parts)), part.slice)
			notes, err := g.transport.Generate(ctx, prompt)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("generating notes for part %d of %d: %w", i+1, len(parts), err)
				}
				return
			}
			partNotes[i] = notes
			done++
			g.progress.PartsDone(promptBytes, len(parts), done)
		})
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return partNotes, nil
}

// splitFileDiffs cuts diff into one section per file, in path order. git emits path
// order by default; the stable sort restores it when diff.orderFile has reordered the
// output, and leaves it untouched otherwise.
func splitFileDiffs(diff string) []string {
	var files []string
	for rest := diff; rest != ""; {
		end := len(rest)
		if i := strings.Index(rest, "\n"+fileDiffHeader); i >= 0 {
			end = i + 1
		}
		files = append(files, rest[:end])
		rest = rest[end:]
	}
	sort.SliceStable(files, func(i, j int) bool {
		return headerPath(files[i]) < headerPath(files[j])
	})
	return files
}

// headerPath is the path a file section sorts under: a rename's or copy's
// destination, else the path its header names. git C-quotes a path holding special
// or non-ASCII bytes, and its escapes are Go string-literal syntax. A header in any
// other form, such as diff.noprefix's, keys on itself whole.
func headerPath(fileDiff string) string {
	header, rest, _ := strings.Cut(fileDiff, "\n")
	if dest, ok := destinationPath(rest); ok {
		return dest
	}
	paths := strings.TrimPrefix(header, fileDiffHeader)
	if strings.HasPrefix(paths, `"`) {
		if quoted, err := strconv.QuotedPrefix(paths); err == nil {
			if unquoted, err := strconv.Unquote(quoted); err == nil {
				return strings.TrimPrefix(unquoted, "a/")
			}
		}
		return paths
	}
	// "a/P b/P": git's prefixed form, both sides naming the same path.
	half := (len(paths) - 1) / 2
	if len(paths)%2 == 1 && half >= 2 && strings.HasPrefix(paths, "a/") &&
		paths[half:half+3] == " b/" && paths[2:half] == paths[half+3:] {
		return paths[2:half]
	}
	return paths
}

// destinationPath reads the bare path from a section's "rename to" or "copy to"
// extended header line, which precede the section's first hunk.
func destinationPath(lines string) (string, bool) {
	for rest := lines; rest != "" && !strings.HasPrefix(rest, "@@"); {
		var line string
		line, rest, _ = strings.Cut(rest, "\n")
		for _, prefix := range []string{"rename to ", "copy to "} {
			if path, ok := strings.CutPrefix(line, prefix); ok {
				if unquoted, err := strconv.Unquote(path); err == nil {
					return unquoted, true
				}
				return path, true
			}
		}
	}
	return "", false
}

// chunkPart is one chunk call's slice of the diff: whole files, or one piece of a
// file too big for a chunk on its own.
type chunkPart struct {
	slice string
	// file is the path of the file a piece is cut from, empty for whole files.
	file          string
	piece, pieces int
}

// cutParts cuts files, in order, into chunk calls: whole
// files packed into chunks, and a file bigger than a chunk's room cut into pieces in
// its place. Room is measured with framing numbered at the diff's byte length, which
// bounds the part and piece counts since every part carries at least one byte of it.
// It fails with ErrNoRoomForDiff when any call it would cut has no room for diff.
func cutParts(instructions, changeMap string, files []string, maxPromptBytes int) ([]chunkPart, error) {
	maxParts := 0
	for _, file := range files {
		maxParts += len(file)
	}
	fixedPart := func(p chunkPart) int {
		return len(composeChunkPrompt(instructions, changeMap, p.framing(maxParts, maxParts), ""))
	}
	room := maxPromptBytes - fixedPart(chunkPart{})
	if room <= 0 {
		return nil, ErrNoRoomForDiff
	}

	var parts []chunkPart
	var fitting []string
	flush := func() {
		for _, chunk := range packChunks(fitting, room) {
			parts = append(parts, chunkPart{slice: chunk})
		}
		fitting = nil
	}
	for _, file := range files {
		if len(file) <= room {
			fitting = append(fitting, file)
			continue
		}
		flush()
		path := headerPath(file)
		pieceRoom := maxPromptBytes - fixedPart(chunkPart{file: path, piece: maxParts, pieces: maxParts})
		pieces, err := cutPieces(file, path, pieceRoom)
		if err != nil {
			return nil, err
		}
		parts = append(parts, pieces...)
	}
	flush()
	return parts, nil
}

// packChunks fills each chunk with whole items, in order, until the next item would
// not fit in room.
func packChunks(items []string, room int) []string {
	var chunks []string
	var current strings.Builder
	for _, item := range items {
		if current.Len() > 0 && current.Len()+len(item) > room {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(item)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

// cutPieces cuts one file's diff into pieces whose slices each repeat the file's
// header and fit room. Cuts fall between hunks where each hunk fits, else between
// lines, else between characters. A room the header fills is ErrNoRoomForDiff.
func cutPieces(file, path string, room int) ([]chunkPart, error) {
	header, body := splitFileHeader(file)
	bodyRoom := room - len(header)
	if bodyRoom <= 0 {
		return nil, ErrNoRoomForDiff
	}

	var segments []string
	for _, hunk := range splitBefore(body, "\n@@") {
		if len(hunk) > bodyRoom {
			segments = append(segments, strings.SplitAfter(hunk, "\n")...)
		} else {
			segments = append(segments, hunk)
		}
	}
	bodies, err := packPieceBodies(segments, bodyRoom)
	if err != nil {
		return nil, err
	}
	pieces := make([]chunkPart, len(bodies))
	for i, b := range bodies {
		pieces[i] = chunkPart{slice: header + b, file: path, piece: i + 1, pieces: len(bodies)}
	}
	return pieces, nil
}

// packPieceBodies packs segments like packChunks, except that a segment longer than
// room, a single line, fills the rest of the current piece and as many more as it
// needs, each cut on a UTF-8 character boundary. A character longer than room is
// ErrNoRoomForDiff; an invalid byte is cut on its own.
func packPieceBodies(segments []string, room int) ([]string, error) {
	var bodies []string
	var current strings.Builder
	flush := func() {
		bodies = append(bodies, current.String())
		current.Reset()
	}
	for _, segment := range segments {
		if current.Len() > 0 && current.Len()+len(segment) > room {
			if len(segment) > room {
				end := characterBoundary(segment, room-current.Len())
				current.WriteString(segment[:end])
				segment = segment[end:]
			}
			flush()
		}
		for len(segment) > room {
			end := characterBoundary(segment, room)
			if end == 0 {
				r, size := utf8.DecodeRuneInString(segment)
				if r != utf8.RuneError || size > 1 {
					return nil, ErrNoRoomForDiff
				}
				end = size
			}
			bodies = append(bodies, segment[:end])
			segment = segment[end:]
		}
		current.WriteString(segment)
	}
	if current.Len() > 0 {
		flush()
	}
	return bodies, nil
}

// characterBoundary is the length of the longest prefix of s, at most n bytes long,
// that ends on a UTF-8 character boundary.
func characterBoundary(s string, n int) int {
	if n >= len(s) {
		return len(s)
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return n
}

// splitFileHeader parts a file's diff into its header, the lines before its first
// hunk, and the rest. A file with no hunk keeps only its "diff --git" line as header.
func splitFileHeader(file string) (header, body string) {
	if i := strings.Index(file, "\n@@"); i >= 0 {
		return file[:i+1], file[i+1:]
	}
	line, _, _ := strings.Cut(file, "\n")
	end := min(len(line)+1, len(file))
	return file[:end], file[end:]
}

// splitBefore cuts s just after each sep's leading byte, so every segment past the
// first opens with the rest of sep.
func splitBefore(s, sep string) []string {
	var out []string
	for {
		i := strings.Index(s, sep)
		if i < 0 {
			return append(out, s)
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
}

func composeChunkPrompt(instructions, changeMap, framing, slice string) string {
	return strings.Join([]string{instructions, changeMap, framing, slice, OutputReminder}, "\n\n")
}

const chunkFramingOpening = "This release's diff is too large to read in one pass, so it has been split into parts. "

func (p chunkPart) framing(part, parts int) string {
	if p.file == "" {
		return fmt.Sprintf(chunkFramingOpening+
			"What follows is part %d of %d. The Change Map above covers the whole release; the diff below covers only this part. "+
			"Write release notes for the changes in this part, in the format above. They will be merged with the other parts' notes.",
			part, parts)
	}
	return fmt.Sprintf(chunkFramingOpening+
		"What follows is part %d of %d: piece %d of %d of the diff for %s, a file too large to read in one part, so its diff has been cut into pieces. "+
		"The Change Map above covers the whole release; the diff below covers only this piece of that file. "+
		"Write release notes for the changes in this piece, in the format above. They will be merged with the other parts' notes.",
		part, parts, p.piece, p.pieces, p.file)
}

func composeMergePrompt(instructions, changeMap string, partNotes []string) string {
	sections := []string{instructions, changeMap, mergeFraming(len(partNotes))}
	for i, notes := range partNotes {
		sections = append(sections, fmt.Sprintf("Notes for part %d of %d:\n\n%s", i+1, len(partNotes), notes))
	}
	return strings.Join(append(sections, OutputReminder), "\n\n")
}

func mergeFraming(parts int) string {
	return fmt.Sprintf("This release's diff was too large to read in one pass, so it was read in %d parts and release notes were written for each part. "+
		"Those notes follow, in part order. From them, write one set of release notes for the whole release, in the format above: "+
		"combine related changes across parts into single bullets, drop duplicates, and rank by the Change Map above.",
		parts)
}
