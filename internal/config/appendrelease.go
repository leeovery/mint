package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"

	"mint/internal/fsutil"
)

// ReleaseGlobs are pathspec globs to append to the [release] table's lists.
type ReleaseGlobs struct {
	DiffExclude []string
	DiffKeep    []string
}

const (
	releaseTable   = "release"
	diffExcludeKey = "diff_exclude"
	diffKeepKey    = "diff_keep"
)

// AppendReleaseGlobs appends globs to root's .mint.toml [release] lists, after any
// entries they hold, adding the key, the [release] table, or the file holding only that
// table, as needed. A [release] table written inline or with dotted keys takes them in
// that form. Every other byte of the file is left as written. A file that does not
// parse as TOML is an error and is left untouched.
func AppendReleaseGlobs(root string, globs ReleaseGlobs) error {
	path := filepath.Join(root, FileName)
	src, mode, err := readForAppend(path)
	if err != nil {
		return err
	}
	out, err := appendReleaseGlobs(src, globs)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", FileName, err)
	}
	if err := fsutil.WriteFile(path, out, mode); err != nil {
		return fmt.Errorf("writing %s: %w", FileName, err)
	}
	return nil
}

func readForAppend(path string) ([]byte, fs.FileMode, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("reading %s: %w", FileName, err)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("reading %s: %w", FileName, err)
	}
	return src, info.Mode().Perm(), nil
}

// ListLine is the TOML key/value line setting key to entries, each a basic string.
func ListLine(key string, entries []string) string {
	quoted := make([]string, len(entries))
	for i, e := range entries {
		quoted[i] = basicString(e)
	}
	return key + " = [" + strings.Join(quoted, ", ") + "]"
}

func basicString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// releaseForm is how a document writes its [release] table.
type releaseForm int

const (
	releaseAbsent releaseForm = iota
	releaseHeader
	releaseInline
	releaseDotted
)

// releaseLayout is where a document's [release] table and its lists sit, as offsets:
// the parser reuses its nodes from one expression to the next.
type releaseLayout struct {
	form  releaseForm
	lists map[string]list
	// last is the end of the table's last key/value, or of its header's key when it has
	// none; an inline table's last key/value, or its opening brace when it has none.
	last int
	// indent is the leading whitespace of the table's last key/value line.
	indent string
}

// list is where a key's value sits: an array's closing bracket and its last entry.
type list struct {
	key     string
	array   bool
	closing int
	// lastStart and lastEnd bound the last entry; lastEnd is 0 for an empty array.
	lastStart, lastEnd int
}

// edit inserts text at offset.
type edit struct {
	offset int
	text   string
}

func appendReleaseGlobs(src []byte, globs ReleaseGlobs) ([]byte, error) {
	layout, err := locateRelease(src)
	if err != nil {
		return nil, err
	}
	eol := lineEnding(src)
	var edits []edit
	var missing []string
	for _, l := range []struct {
		key   string
		globs []string
	}{{diffExcludeKey, globs.DiffExclude}, {diffKeepKey, globs.DiffKeep}} {
		if len(l.globs) == 0 {
			continue
		}
		if spot, ok := layout.lists[l.key]; ok {
			listEdits, err := appendToList(src, spot, l.globs, eol)
			if err != nil {
				return nil, err
			}
			edits = append(edits, listEdits...)
			continue
		}
		missing = append(missing, ListLine(l.key, l.globs))
	}
	if len(missing) > 0 {
		edits = append(edits, addKeys(src, layout, missing, eol))
	}
	return applyEdits(src, edits), nil
}

func locateRelease(src []byte) (releaseLayout, error) {
	var p unstable.Parser
	p.Reset(src)
	layout := releaseLayout{lists: map[string]list{}}
	in := scopeRoot
	for p.NextExpression() {
		e := p.Expression()
		switch e.Kind {
		case unstable.Table, unstable.ArrayTable:
			in = scopeOther
			if e.Kind == unstable.Table && slices.Equal(keyParts(e.Key()), []string{releaseTable}) {
				in = scopeRelease
				layout.form = releaseHeader
				layout.last = rangeEnd(lastKeyNode(e))
			}
		case unstable.KeyValue:
			locateKeyValue(&layout, src, in, e)
		}
	}
	if err := p.Error(); err != nil {
		return releaseLayout{}, err
	}
	return layout, nil
}

// scope is the table a document's key/values belong to.
type scope int

const (
	scopeRoot scope = iota
	scopeRelease
	scopeOther
)

func locateKeyValue(layout *releaseLayout, src []byte, in scope, kv *unstable.Node) {
	key := keyParts(kv.Key())
	switch {
	case in == scopeRelease:
		recordReleaseKey(layout, src, key, kv)
	case in == scopeRoot && len(key) == 1 && key[0] == releaseTable && kv.Value().Kind == unstable.InlineTable:
		layout.form = releaseInline
		layout.last = rangeEnd(kv.Value())
		for inner := kv.Value().Children(); inner.Next(); {
			if innerKey := keyParts(inner.Node().Key()); len(innerKey) == 1 {
				layout.lists[innerKey[0]] = listOf(innerKey[0], inner.Node())
			}
			layout.last = rangeEnd(inner.Node())
		}
	case in == scopeRoot && len(key) > 1 && key[0] == releaseTable:
		layout.form = releaseDotted
		recordReleaseKey(layout, src, key[1:], kv)
	}
}

func recordReleaseKey(layout *releaseLayout, src []byte, key []string, kv *unstable.Node) {
	if len(key) == 1 {
		layout.lists[key[0]] = listOf(key[0], kv)
	}
	layout.last = rangeEnd(kv)
	layout.indent = leadingWhitespace(src, int(kv.Raw.Offset))
}

func keyParts(it unstable.Iterator) []string {
	var parts []string
	for it.Next() {
		parts = append(parts, string(it.Node().Data))
	}
	return parts
}

func lastKeyNode(n *unstable.Node) *unstable.Node {
	var last *unstable.Node
	for it := n.Key(); it.Next(); {
		last = it.Node()
	}
	return last
}

func rangeEnd(n *unstable.Node) int {
	return int(n.Raw.Offset + n.Raw.Length)
}

func listOf(key string, kv *unstable.Node) list {
	l := list{key: key, array: kv.Value().Kind == unstable.Array, closing: rangeEnd(kv) - 1}
	for it := kv.Value().Children(); it.Next(); {
		l.lastStart, l.lastEnd = int(it.Node().Raw.Offset), rangeEnd(it.Node())
	}
	return l
}

// appendToList inserts globs after the list's last entry, or before an empty list's
// closing bracket: on the entry's own line when the array closes there, else each on a
// line of its own, indented as that entry's line is and comma-terminated as it is.
func appendToList(src []byte, l list, globs []string, eol string) ([]edit, error) {
	if !l.array {
		return nil, fmt.Errorf("%s is not an array", l.key)
	}
	closing, lastEnd := l.closing, l.lastEnd
	quoted := make([]string, len(globs))
	for i, g := range globs {
		quoted[i] = basicString(g)
	}
	if lastEnd == 0 {
		return []edit{{closing, strings.Join(quoted, ", ")}}, nil
	}
	comma, hasComma := trailingComma(src, lastEnd, closing)
	anchor := lastEnd
	if hasComma {
		anchor = comma + 1
	}
	lineEnd := bytes.IndexByte(src[anchor:closing], '\n')
	if lineEnd < 0 {
		return []edit{{lastEnd, ", " + strings.Join(quoted, ", ")}}, nil
	}
	lineEnd += anchor
	if lineEnd > anchor && src[lineEnd-1] == '\r' {
		lineEnd--
	}
	indent := leadingWhitespace(src, l.lastStart)
	var lines strings.Builder
	for i, q := range quoted {
		lines.WriteString(eol + indent + q)
		if hasComma || i < len(quoted)-1 {
			lines.WriteByte(',')
		}
	}
	edits := []edit{{lineEnd, lines.String()}}
	if !hasComma {
		edits = append([]edit{{lastEnd, ","}}, edits...)
	}
	return edits, nil
}

// trailingComma finds the comma after an array's last entry, between from and the
// array's closing bracket, past any whitespace, newlines and comments.
func trailingComma(src []byte, from, closing int) (int, bool) {
	for i := from; i < closing; i++ {
		switch src[i] {
		case ',':
			return i, true
		case '#':
			next := bytes.IndexByte(src[i:closing], '\n')
			if next < 0 {
				return 0, false
			}
			i += next
		}
	}
	return 0, false
}

// addKeys adds lines, each a key/value, to the [release] table in the form it is
// written, or as a new table at the end of a document that has none.
func addKeys(src []byte, layout releaseLayout, lines []string, eol string) edit {
	switch layout.form {
	case releaseInline:
		return addInlineKeys(src, layout.last, lines)
	case releaseDotted:
		for i, l := range lines {
			lines[i] = releaseTable + "." + l
		}
		fallthrough
	case releaseHeader:
		var text strings.Builder
		for _, l := range lines {
			text.WriteString(eol + layout.indent + l)
		}
		return edit{lineEndAfter(src, layout.last), text.String()}
	}
	var text strings.Builder
	if len(src) > 0 {
		if !bytes.HasSuffix(src, []byte(eol)) {
			text.WriteString(eol)
		}
		if !bytes.HasSuffix(src, []byte(eol+eol)) {
			text.WriteString(eol)
		}
	}
	text.WriteString("[" + releaseTable + "]" + eol)
	for _, l := range lines {
		text.WriteString(l + eol)
	}
	return edit{len(src), text.String()}
}

// addInlineKeys adds lines after last, an inline table's last key/value or, in an empty
// one, its opening brace.
func addInlineKeys(src []byte, last int, lines []string) edit {
	joined := strings.Join(lines, ", ")
	if src[last-1] != '{' {
		return edit{last, ", " + joined}
	}
	text := " " + joined
	if src[last] == '}' {
		text += " "
	}
	return edit{last, text}
}

// lineEndAfter is the offset of the line break ending the line offset is on, or the
// end of src on its last line.
func lineEndAfter(src []byte, offset int) int {
	i := bytes.IndexByte(src[offset:], '\n')
	if i < 0 {
		return len(src)
	}
	end := offset + i
	if end > offset && src[end-1] == '\r' {
		end--
	}
	return end
}

func leadingWhitespace(src []byte, offset int) string {
	start := bytes.LastIndexByte(src[:offset], '\n') + 1
	end := start
	for end < offset && (src[end] == ' ' || src[end] == '\t') {
		end++
	}
	return string(src[start:end])
}

func lineEnding(src []byte) string {
	if bytes.Contains(src, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

// applyEdits inserts each edit's text at its offset in src; edits at one offset go in
// the order given.
func applyEdits(src []byte, edits []edit) []byte {
	slices.SortStableFunc(edits, func(a, b edit) int { return a.offset - b.offset })
	var out bytes.Buffer
	at := 0
	for _, e := range edits {
		out.Write(src[at:e.offset])
		out.WriteString(e.text)
		at = e.offset
	}
	out.Write(src[at:])
	return out.Bytes()
}
