package config

import (
	"bytes"
	"slices"
)

// dropReleaseGlobs drops from src's [release] lists every entry drop names for each,
// duplicates included, one at a time, src parsed again after each: removeEntry takes
// out the entry with its separator and, where the entry has a line to itself, that
// line, comment and all, leaving every other byte as written. An entry drop names
// that a list doesn't hold, or a list that isn't an array, is passed over.
func dropReleaseGlobs(src []byte, drop ReleaseGlobs) ([]byte, error) {
	for _, l := range []struct {
		key    string
		values []string
	}{{diffExcludeKey, drop.DiffExclude}, {diffKeepKey, drop.DiffKeep}} {
		for _, value := range l.values {
			for {
				layout, err := locateRelease(src)
				if err != nil {
					return nil, err
				}
				spot, ok := layout.lists[l.key]
				if !ok || !spot.array {
					break
				}
				i := slices.IndexFunc(spot.entries, func(e entry) bool { return e.isString && e.value == value })
				if i < 0 {
					break
				}
				src = removeEntry(src, spot.entries, i)
			}
		}
	}
	return src, nil
}

// removeEntry takes entries[i], of one array, out of src:
//
//   - alone on its line, past its comma and any comment: the whole line;
//   - after another entry on its line: from just past that entry's comma through its
//     own comma, or, last in the array, from the end of that entry through its own;
//   - otherwise, first on its line: the entry, its comma and the blanks after it.
func removeEntry(src []byte, entries []entry, i int) []byte {
	e := entries[i]
	lineStart := bytes.LastIndexByte(src[:e.start], '\n') + 1
	firstOnLine := isBlank(src[lineStart:e.start])
	afterEntry := skipBlanks(src, e.end)
	hasComma := afterEntry < len(src) && src[afterEntry] == ','
	afterComma := afterEntry
	if hasComma {
		afterComma = skipBlanks(src, afterEntry+1)
	}
	switch {
	case firstOnLine && endsLine(src, afterComma):
		return cut(src, lineStart, nextLineStart(src, afterComma))
	case i > 0 && !bytes.ContainsRune(src[entries[i-1].end:e.start], '\n'):
		if hasComma {
			return cut(src, skipBlanks(src, entries[i-1].end)+1, afterEntry+1)
		}
		return cut(src, entries[i-1].end, e.end)
	}
	return cut(src, e.start, afterComma)
}

// endsLine reports whether offset is where src's line ends, or a comment that runs to
// its end starts.
func endsLine(src []byte, offset int) bool {
	return offset == len(src) || src[offset] == '\n' || src[offset] == '\r' || src[offset] == '#'
}

// nextLineStart is the offset just past the line break ending the line offset is on,
// or the end of src on its last line.
func nextLineStart(src []byte, offset int) int {
	i := bytes.IndexByte(src[offset:], '\n')
	if i < 0 {
		return len(src)
	}
	return offset + i + 1
}

// skipBlanks is the offset of the first byte at or after offset that is neither a
// space nor a tab.
func skipBlanks(src []byte, offset int) int {
	for offset < len(src) && (src[offset] == ' ' || src[offset] == '\t') {
		offset++
	}
	return offset
}

func isBlank(b []byte) bool {
	return len(bytes.Trim(b, " \t")) == 0
}

// cut is src without src[from:to].
func cut(src []byte, from, to int) []byte {
	return slices.Concat(src[:from], src[to:])
}
