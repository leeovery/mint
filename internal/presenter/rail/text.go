package rail

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// span is a run of text in one style.
type span struct {
	text  string
	style style
}

// s is text drawn in st on one row. Every piece of text enters a line through it, so
// none can move the cursor or change the pen (see oneRow).
func s(text string, st style) span { return span{text: oneRow(text), style: st} }

// sgr is a well-formed SGR sequence: the text's own colour, which the rail draws its
// own over.
var sgr = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

var lineBreaks = strings.NewReplacer("\r\n", " ", "\n", " ")

// oneRow is text as one row draws it: each line break a space, then shown.
func oneRow(text string) string { return shown(lineBreaks.Replace(text)) }

// tabStop is the columns from one tab stop to the next.
const tabStop = 8

// shown is a line of text as the rail draws it: its SGR removed, each tab expanded
// to the next stop counted from the line's start, and every other control character,
// a lone escape included, drawn as its control picture. It is the one exception to
// "no character changed", and it changes only how the text looks on the terminal:
// what ships, and plain, keep the text byte for byte.
func shown(text string) string {
	text = sgr.ReplaceAllString(text, "")
	var b strings.Builder
	col, state := 0, -1
	for rest := text; rest != ""; {
		var cluster string
		var width int
		cluster, rest, width, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if pic, ok := picture(cluster, col); ok {
			cluster, width = pic, cells(pic)
		}
		b.WriteString(cluster)
		col += width
	}
	return b.String()
}

// picture is what stands in for a cluster the terminal would act on rather than
// draw: spaces to the next tab stop for a tab, U+FFFD for a byte that isn't UTF-8,
// the control picture for C0 and DEL, and for C1, which is ESC and a letter in 8
// bits, ␛ and that letter.
func picture(cluster string, col int) (string, bool) {
	r, size := utf8.DecodeRuneInString(cluster)
	switch {
	case r == '\t':
		return strings.Repeat(" ", tabStop-col%tabStop), true
	case r == utf8.RuneError && size <= 1:
		return "�", true
	case r < 0x20:
		return string(0x2400 + r), true
	case r == 0x7f:
		return "␡", true
	case r >= 0x80 && r <= 0x9f:
		return "␛" + string(r-0x40), true
	}
	return "", false
}

// line is one terminal row, its spans left to right from the first column.
type line []span

// cells is how many terminal cells text takes, a wide glyph two.
func cells(text string) int { return uniseg.StringWidth(text) }

func (ln line) width() int {
	n := 0
	for _, sp := range ln {
		n += cells(sp.text)
	}
	return n
}

// render is ln as the terminal is sent it at l, neighbouring spans of one style
// painted as one.
func (ln line) render(l Level) string {
	var b strings.Builder
	for i := 0; i < len(ln); {
		run, st := ln[i].text, ln[i].style
		for i++; i < len(ln) && ln[i].style == st; i++ {
			run += ln[i].text
		}
		b.WriteString(l.paint(run, st))
	}
	return b.String()
}

// to is ln padded with blanks out to column col, the cells left of col taken.
func (ln line) to(col int) line {
	if gap := col - ln.width(); gap > 0 {
		return append(ln, s(strings.Repeat(" ", gap), plain))
	}
	return ln
}

// right is ln with right after it, ending on column width.
func (ln line) right(right []span, width int) line {
	return append(ln.to(width-line(right).width()), right...)
}

// glyph is one grapheme cluster, the unit a terminal draws in a cell or two.
type glyph struct {
	text  string
	width int
	style style
}

func glyphsOf(spans []span) []glyph {
	var gs []glyph
	for _, sp := range spans {
		state := -1
		for rest := sp.text; rest != ""; {
			var cluster string
			var width int
			cluster, rest, width, state = uniseg.FirstGraphemeClusterInString(rest, state)
			gs = append(gs, glyph{text: cluster, width: width, style: sp.style})
		}
	}
	return gs
}

func widthOf(gs []glyph) int {
	n := 0
	for _, g := range gs {
		n += g.width
	}
	return n
}

func spansOf(gs []glyph) []span {
	var out []span
	for _, g := range gs {
		if n := len(out); n > 0 && out[n-1].style == g.style {
			out[n-1].text += g.text
			continue
		}
		out = append(out, span{text: g.text, style: g.style})
	}
	return out
}

// words are text's breaking units: each word with the spaces after it, so a line
// broken after one keeps its spaces and the lines joined again are the text.
func words(gs []glyph) [][]glyph {
	var out [][]glyph
	start := 0
	for i := 1; i < len(gs); i++ {
		if gs[i-1].text == " " && gs[i].text != " " {
			out = append(out, gs[start:i])
			start = i
		}
	}
	if start < len(gs) {
		out = append(out, gs[start:])
	}
	return out
}

// flow lays units out in lines, first wide for the first and rest for each after,
// starting a line where the next unit won't fit and splitting a unit no line holds.
type flow struct {
	lines [][]glyph
	cur   []glyph
	room  int
	rest  int
}

func newFlow(first, rest int) *flow { return &flow{room: first, rest: rest} }

// put adds unit, after sep where the line already holds something.
func (f *flow) put(unit, sep []glyph) {
	for len(unit) > 0 {
		lead := sep
		if len(f.cur) == 0 {
			lead = nil
		}
		need := widthOf(lead) + widthOf(unit)
		switch {
		case need <= f.room-widthOf(f.cur):
			f.cur = append(append(f.cur, lead...), unit...)
			unit = nil
		case len(f.cur) > 0:
			f.next()
		default:
			n := split(unit, f.room)
			f.cur = append(f.cur, unit[:n]...)
			unit = unit[n:]
			f.next()
		}
	}
}

func (f *flow) next() {
	f.lines = append(f.lines, f.cur)
	f.cur, f.room = nil, f.rest
}

// done is the lines laid out, one at least.
func (f *flow) done() [][]span {
	if len(f.cur) > 0 || len(f.lines) == 0 {
		f.lines = append(f.lines, f.cur)
	}
	out := make([][]span, len(f.lines))
	for i, gs := range f.lines {
		out[i] = spansOf(gs)
	}
	return out
}

// fitting is how many of gs fit in width cells, one at least so a line always
// takes something.
func fitting(gs []glyph, width int) int {
	used := 0
	for i, g := range gs {
		if used+g.width > width {
			return max(i, 1)
		}
		used += g.width
	}
	return len(gs)
}

// split is how much of a unit too long for any line goes on this one: what fits,
// back to just after its last slash where it has one, so a path breaks between its
// parts.
func split(gs []glyph, width int) int {
	n := fitting(gs, width)
	for i := n; i > 1; i-- {
		if gs[i-1].text == "/" {
			return i
		}
	}
	return n
}

// wrap is text in lines first wide, then rest, broken after a space or inside a
// word no line holds, and no character changed: joined, the lines are the text as
// shown.
func wrap(text []span, first, rest int) [][]span {
	f := newFlow(first, rest)
	for _, w := range words(glyphsOf(text)) {
		f.put(w, nil)
	}
	return f.done()
}

// ellipsis marks a cut.
const ellipsis = "…"

// cut is text cut to width cells, its last cell an ellipsis where any was cut.
func cut(text []span, width int) []span {
	gs := glyphsOf(text)
	if widthOf(gs) <= width {
		return text
	}
	n := 0
	for used := 0; n < len(gs) && used+gs[n].width <= width-cells(ellipsis); n++ {
		used += gs[n].width
	}
	last := plain
	if n > 0 {
		last = gs[n-1].style
	}
	return append(spansOf(gs[:n]), s(ellipsis, last))
}

// cutStart is text, already shown, cut from the left to width cells, an ellipsis
// first.
func cutStart(text string, width int) string {
	gs := glyphsOf([]span{{text: text}})
	if widthOf(gs) <= width {
		return text
	}
	n := len(gs)
	for used := 0; n > 0 && used+gs[n-1].width <= width-cells(ellipsis); n-- {
		used += gs[n-1].width
	}
	var b strings.Builder
	b.WriteString(ellipsis)
	for _, g := range gs[n:] {
		b.WriteString(g.text)
	}
	return b.String()
}

// textLines are text's lines: a newline ends a line, so one at the very end opens
// none after it, and a carriage return just before one goes with it.
func textLines(text string) []string {
	parts := strings.Split(text, "\n")
	lines, open := parts[:len(parts)-1], parts[len(parts)-1]
	for i := range lines {
		lines[i] = strings.TrimSuffix(lines[i], "\r")
	}
	if open != "" || len(lines) == 0 {
		lines = append(lines, open)
	}
	return lines
}
