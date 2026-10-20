package notes

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// fileDiffHeader opens each file's section of a git diff.
const fileDiffHeader = "diff --git "

// generateChunked writes the notes for a diff that cannot go in one call: the diff's
// files are cut in path order into chunk calls of at most maxPromptBytes, each
// carrying the release's instructions and whole-release Change Map and run
// concurrently, then one merge call of at most maxPromptBytes turns every chunk's
// notes, in chunk order, into the body. The merge output is returned byte-identical.
// promptBytes is the single-pass prompt's size, reported to g.progress with the run's
// split and advance. No call over its bound is sent: ErrNoRoomForDiff in place of a
// call with no room for diff under maxPromptBytes, ErrMergeTooLarge in place of the
// merge. A merge call whose failure is classified SIZE is never halved: it fails as
// ErrMergeTooLarge, carrying the call's own failure.
func (g *Generator) generateChunked(ctx context.Context, instructions, changeMap, diff string, promptBytes, maxPromptBytes int) (string, error) {
	cut, parts, err := cutRun(instructions, changeMap, diff, maxPromptBytes)
	if err != nil {
		return "", err
	}
	g.progress.PartsDone(promptBytes, len(parts), 0)
	return g.writeParts(ctx, cut, parts, 0, promptBytes, maxPromptBytes)
}

// generateHalved is generateChunked for a single-pass call of promptBytes that failed
// with failure, classified SIZE: its chunk calls are at most half its size, counting
// as the run's first halving, and the re-cut is reported to g.progress in place of the
// split. A re-cut with no room for diff fails as ErrTooLargeForModel, carrying failure.
func (g *Generator) generateHalved(ctx context.Context, instructions, changeMap, diff string, failure error, promptBytes, maxPromptBytes int) (string, error) {
	cut, parts, err := cutRun(instructions, changeMap, diff, promptBytes/2)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrTooLargeForModel, failure)
	}
	g.progress.Halved(0, len(parts))
	return g.writeParts(ctx, cut, parts, 1, promptBytes, maxPromptBytes)
}

func cutRun(instructions, changeMap, diff string, target int) (cutter, []chunkPart, error) {
	files := splitFileDiffs(diff)
	cut := newCutter(instructions, changeMap, files)
	parts, err := cut.cutParts(files, target)
	return cut, parts, err
}

func (g *Generator) writeParts(ctx context.Context, cut cutter, parts []chunkPart, halvings, promptBytes, maxPromptBytes int) (string, error) {
	sent, partNotes, err := newWave(g, cut, parts, halvings, promptBytes, maxPromptBytes).run(ctx)
	if err == nil {
		var body string
		if body, err = g.merge(ctx, cut, partNotes, maxPromptBytes); err == nil {
			return body, nil
		}
	}
	return "", g.keepChunkNotes(err, sent, partNotes)
}

func (g *Generator) merge(ctx context.Context, cut cutter, partNotes []string, maxPromptBytes int) (string, error) {
	mergePrompt := composeMergePrompt(cut.instructions, cut.changeMap, partNotes)
	if len(mergePrompt) > maxPromptBytes {
		return "", ErrMergeTooLarge
	}
	g.progress.Merging(len(partNotes))
	body, code, err := g.generate(ctx, mergePrompt, maxPromptBytes)
	if err != nil {
		if code == codeSize {
			return "", fmt.Errorf("%w: %w", ErrMergeTooLarge, err)
		}
		return "", &callError{action: "merging part notes", location: "merging", err: err}
	}
	g.progress.Merged(len(partNotes))
	return body, nil
}

// callError is a chunked run's failed call, carrying where the call sat in the run so
// a failure's cause phrase can name it.
type callError struct {
	action   string
	location string
	err      error
}

func partCallError(part, parts int, err error) *callError {
	location := fmt.Sprintf("part %d of %d", part, parts)
	return &callError{action: "generating notes for " + location, location: location, err: err}
}

func (e *callError) Error() string { return e.action + ": " + e.err.Error() }

func (e *callError) Unwrap() error { return e.err }

// maxHalvings bounds how many times one slice of the diff is halved, counted from its
// first SIZE answer: three halvings take it to an eighth of the size that first failed.
const maxHalvings = 3

// maxChunkCallsInFlight bounds a wave's concurrent chunk calls, so a large release
// never lands all of its calls on the machine and the provider's limits at once.
const maxChunkCallsInFlight = 4

// wave makes a chunked run's chunk calls, at most maxChunkCallsInFlight at a time.
// Parts are numbered by their place in chunk order at the moment each call is
// composed, so a halved part's new parts, taking its place, move every later part's
// number up. Its calls run on the run's ctx itself: the wave sets no deadline of its
// own, so a timeout is per call.
type wave struct {
	g              *Generator
	cut            cutter
	promptBytes    int
	maxPromptBytes int

	mu      sync.Mutex
	changed *sync.Cond
	// slots is every part in chunk order; queue the parts not yet sent, in send order.
	slots    []*waveSlot
	queue    []*waveSlot
	inFlight int
	done     int
	failure  error
}

type waveSlot struct {
	part  chunkPart
	notes string
	// halvings is how many times the slice this part was cut from has been halved. It
	// never changes once the slot is made, so a call's goroutine reads it unlocked.
	halvings int
}

func newWave(g *Generator, cut cutter, parts []chunkPart, halvings, promptBytes, maxPromptBytes int) *wave {
	w := &wave{g: g, cut: cut, promptBytes: promptBytes, maxPromptBytes: maxPromptBytes}
	w.changed = sync.NewCond(&w.mu)
	for _, part := range parts {
		w.slots = append(w.slots, &waveSlot{part: part, halvings: halvings})
	}
	w.queue = slices.Clone(w.slots)
	return w
}

// run sends every part and returns the parts, halved parts in their slice's place,
// and their notes in chunk order once every call has finished, a part with none
// holding "". A call whose failure is classified SIZE has its slice re-cut at half
// the call's size, and the new parts are queued behind the parts not yet sent; with
// its slice already halved maxHalvings times, or no room for diff at half the size,
// the call fails as ErrTooLargeForModel. Any other failure, and that one, ends the
// wave: no further call starts, the calls already running finish, a failure among
// them is not halved, nor classified when it comes after the end, and the first
// such failure is returned with the parts and notes. ctx ending stops it too, and
// its error is returned, with no parts or notes, over any earlier failure. Each
// part that succeeds, and each re-cut, is reported to g.progress as it happens.
func (w *wave) run(ctx context.Context) ([]chunkPart, []string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for {
		for w.inFlight > 0 && !w.canSend(ctx) {
			w.changed.Wait()
		}
		if !w.canSend(ctx) {
			break
		}
		slot := w.queue[0]
		w.queue = w.queue[1:]
		w.inFlight++
		go w.send(ctx, slot, slot.part, w.prompt(slot))
	}

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	parts := make([]chunkPart, len(w.slots))
	partNotes := make([]string, len(w.slots))
	for i, slot := range w.slots {
		parts[i], partNotes[i] = slot.part, slot.notes
	}
	return parts, partNotes, w.failure
}

func (w *wave) canSend(ctx context.Context) bool {
	return w.failure == nil && ctx.Err() == nil && len(w.queue) > 0 && w.inFlight < maxChunkCallsInFlight
}

func (w *wave) prompt(slot *waveSlot) string {
	part := slices.Index(w.slots, slot) + 1
	return composeChunkPrompt(w.cut.instructions, w.cut.changeMap, slot.part.framing(part, len(w.slots)), slot.part.slice)
}

// send makes one part's chunk call and records its outcome. part is slot's part as
// it was when the call was composed, copied under the lock, since a halving
// elsewhere can renumber slot's piece while the call runs. The call's place in flight
// is freed in the same step that records the outcome, so the wave sees a failure, or
// a halved part's new parts, before it can send another call in that place.
func (w *wave) send(ctx context.Context, slot *waveSlot, part chunkPart, prompt string) {
	notes, err := w.g.transport.Generate(ctx, prompt)
	code := codeOther
	if err != nil && !w.ended() {
		code, err = w.g.readFailure(ctx, err, w.maxPromptBytes)
	}
	var halves []chunkPart
	var recutErr error
	halving := err != nil && code == codeSize && slot.halvings < maxHalvings
	if halving {
		halves, recutErr = w.cut.recut(part, len(prompt)/2)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	defer w.changed.Broadcast()
	w.inFlight--
	switch {
	case err == nil:
		slot.notes = notes
		w.done++
		w.g.progress.CallSucceeded(len(prompt))
		w.g.progress.PartsDone(w.promptBytes, len(w.slots), w.done)
	case w.failure != nil:
		// The wave has already ended: a later failure is neither recorded nor halved.
	case halving && recutErr == nil:
		w.halve(slot, halves)
	case code == codeSize:
		w.failure = partCallError(slices.Index(w.slots, slot)+1, len(w.slots), fmt.Errorf("%w: %w", ErrTooLargeForModel, err))
	default:
		w.failure = partCallError(slices.Index(w.slots, slot)+1, len(w.slots), err)
	}
}

// ended reports whether a part has failed for good, which ends the wave.
func (w *wave) ended() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.failure != nil
}

// halve reports slot's re-cut, then puts the parts its slice was re-cut into in its
// place in chunk order and queues them. The pieces of a re-cut piece's file are
// renumbered by their place among that file's pieces.
func (w *wave) halve(slot *waveSlot, halves []chunkPart) {
	replacements := make([]*waveSlot, len(halves))
	for i, part := range halves {
		replacements[i] = &waveSlot{part: part, halvings: slot.halvings + 1}
	}
	at := slices.Index(w.slots, slot)
	w.g.progress.Halved(at+1, len(halves))
	w.slots = slices.Replace(w.slots, at, at+1, replacements...)
	w.queue = append(w.queue, replacements...)

	if slot.part.file == "" {
		return
	}
	var pieces []*waveSlot
	for _, s := range w.slots {
		if s.part.file == slot.part.file && s.part.header == slot.part.header {
			pieces = append(pieces, s)
		}
	}
	for i, s := range pieces {
		s.part.piece, s.part.pieces = i+1, len(pieces)
	}
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
	// file is the path of the file a piece is cut from, empty for whole files, and
	// header the file's diff header the piece's slice opens with.
	file, header  string
	piece, pieces int
}

// cutter cuts a run's diff into chunk calls carrying its instructions and Change Map.
type cutter struct {
	instructions, changeMap string
	// numberBound bounds the part and piece numbers any of the run's framing names:
	// the diff's byte length, since every part carries at least one byte of it.
	numberBound int
}

func newCutter(instructions, changeMap string, files []string) cutter {
	diffBytes := 0
	for _, file := range files {
		diffBytes += len(file)
	}
	return cutter{instructions: instructions, changeMap: changeMap, numberBound: diffBytes}
}

// fixedPart is the size of p's call without its slice, its framing numbered at the
// bound so the call fits whatever place it takes in the run.
func (c cutter) fixedPart(p chunkPart) int {
	return len(composeChunkPrompt(c.instructions, c.changeMap, p.framing(c.numberBound, c.numberBound), ""))
}

// cutParts cuts files, in order, into chunk calls of at most target bytes: whole
// files packed into chunks, and a file bigger than a chunk's room cut into pieces in
// its place. It fails with ErrNoRoomForDiff when any call it would cut has no room
// for diff.
func (c cutter) cutParts(files []string, target int) ([]chunkPart, error) {
	room := target - c.fixedPart(chunkPart{})
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
		header, body := splitFileHeader(file)
		pieces, err := c.cutPieces(header, body, headerPath(file), target)
		if err != nil {
			return nil, err
		}
		parts = append(parts, pieces...)
	}
	flush()
	return parts, nil
}

// recut cuts p's slice again into calls of at most target bytes: a chunk's files by
// the packing and piece rules, a piece's diff into smaller pieces of its file.
func (c cutter) recut(p chunkPart, target int) ([]chunkPart, error) {
	if p.file == "" {
		return c.cutParts(splitFileDiffs(p.slice), target)
	}
	return c.cutPieces(p.header, strings.TrimPrefix(p.slice, p.header), p.file, target)
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

// cutPieces cuts the body of path's diff into piece calls of at most target bytes,
// each slice repeating the file's header. Cuts fall between hunks where each hunk
// fits, else between lines, else between characters. A room the header fills is
// ErrNoRoomForDiff.
func (c cutter) cutPieces(header, body, path string, target int) ([]chunkPart, error) {
	room := target - c.fixedPart(chunkPart{file: path, piece: c.numberBound, pieces: c.numberBound})
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
		pieces[i] = chunkPart{slice: header + b, file: path, header: header, piece: i + 1, pieces: len(bodies)}
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
