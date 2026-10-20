package notes

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// fileDiffHeader opens each file's section of a git diff.
const fileDiffHeader = "diff --git "

// generateChunked writes the notes for a diff whose single-pass prompt is over
// maxPromptBytes: the diff's files are packed in path order into chunk calls, each
// carrying the release's instructions and whole-release Change Map and run
// concurrently, then one merge call turns every chunk's notes, in chunk order, into
// the body. The merge output is returned byte-identical.
func (g *Generator) generateChunked(ctx context.Context, instructions, changeMap, diff string, maxPromptBytes int) (string, error) {
	files := splitFileDiffs(diff)
	slices := packChunks(files, chunkRoom(instructions, changeMap, len(files), maxPromptBytes))

	partNotes, err := g.generateParts(ctx, instructions, changeMap, slices)
	if err != nil {
		return "", err
	}

	body, err := g.transport.Generate(ctx, composeMergePrompt(instructions, changeMap, partNotes))
	if err != nil {
		return "", fmt.Errorf("merging part notes: %w", err)
	}
	return body, nil
}

// maxChunkCallsInFlight bounds a wave's concurrent chunk calls, so a large release
// never lands all of its calls on the machine and the provider's limits at once.
const maxChunkCallsInFlight = 4

// generateParts makes one chunk call per slice, at most maxChunkCallsInFlight at a
// time, and returns their notes in slice order once every call has finished. Each call
// runs on ctx itself: the wave sets no deadline of its own, so a timeout is per call.
// A failure is reported for the lowest-numbered failed part.
func (g *Generator) generateParts(ctx context.Context, instructions, changeMap string, slices []string) ([]string, error) {
	partNotes := make([]string, len(slices))
	errs := make([]error, len(slices))
	slots := make(chan struct{}, maxChunkCallsInFlight)
	var wg sync.WaitGroup
	for i, slice := range slices {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			partNotes[i], errs[i] = g.transport.Generate(ctx, composeChunkPrompt(instructions, changeMap, i+1, len(slices), slice))
		})
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return nil, fmt.Errorf("generating notes for part %d of %d: %w", i+1, len(slices), err)
		}
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

// chunkRoom is the bytes of diff a chunk call can carry: the cap less the chunk
// prompt's fixed part. The framing is measured at maxParts, the most chunks the files
// can pack into, so no real part number makes a chunk prompt longer than measured.
func chunkRoom(instructions, changeMap string, maxParts, maxPromptBytes int) int {
	return maxPromptBytes - len(composeChunkPrompt(instructions, changeMap, maxParts, maxParts, ""))
}

// packChunks fills each chunk with whole files, in order, until the next file's diff
// would not fit in room. A file larger than room on its own takes a chunk to itself.
func packChunks(files []string, room int) []string {
	var chunks []string
	var current strings.Builder
	for _, file := range files {
		if current.Len() > 0 && current.Len()+len(file) > room {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		current.WriteString(file)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func composeChunkPrompt(instructions, changeMap string, part, parts int, slice string) string {
	return strings.Join([]string{instructions, changeMap, chunkFraming(part, parts), slice, OutputReminder}, "\n\n")
}

func chunkFraming(part, parts int) string {
	return fmt.Sprintf("This release's diff is too large to read in one pass, so it has been split into parts. "+
		"What follows is part %d of %d. The Change Map above covers the whole release; the diff below covers only this part. "+
		"Write release notes for the changes in this part, in the format above. They will be merged with the other parts' notes.",
		part, parts)
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
