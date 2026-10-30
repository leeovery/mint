package rail

import (
	"math"
	"strings"
)

// minWhere is the least of the header's where worth showing once it's cut, as
// agntc's is; shorter, where goes and the version stays.
const minWhere = 12

func (Header) reached() bool { return true }

func (h Header) draw(l Layout) []line {
	left := line{margin, s("mint", accentBold), s(" ", plain), s("─", dim), s(" ", plain)}
	left = append(left, cut([]span{s(h.Command, plain)}, l.width-left.width())...)
	right := h.right(l.width - left.width() - timeGap)
	if right == "" {
		return []line{left}
	}
	return []line{left.right([]span{s(right, dim)}, l.width)}
}

// right is the header's right side in room cells: where and the version, else
// where cut from the left, else the version alone, else nothing.
func (h Header) right(room int) string {
	where, version, suffix := oneRow(h.Where), "", ""
	if h.Version != "" {
		version = "v" + oneRow(h.Version)
		suffix = " · " + version
	}
	if where == "" {
		return fitsOrNone(version, room)
	}
	if whole := where + suffix; cells(whole) <= room {
		return whole
	}
	if whereRoom := room - cells(suffix); whereRoom >= minWhere {
		return cutStart(where, whereRoom) + suffix
	}
	return fitsOrNone(version, room)
}

func fitsOrNone(text string, room int) string {
	if cells(text) > room {
		return ""
	}
	return text
}

func (Version) reached() bool { return true }

func (v Version) draw(l Layout) []line {
	head := line{margin, s("mint", accent)}
	return []line{append(head, cut([]span{s(" v"+v.Value, plain)}, l.width-head.width())...)}
}

func (st Station) reached() bool { return st.Kind != Pending }

func (st Station) draw(l Layout) []line {
	mark, name, rail := s("◇", accent), bold, solid
	if st.Kind == Pending {
		mark, name, rail = s("◇", dim), dim, dotted
	}
	var text []span
	if st.Detail != "" {
		text = []span{s(st.Detail, dim)}
	}
	return l.row(row{mark: mark, name: s(st.Name, name), rail: rail, lay: l.fit(text, false)})
}

func (End) reached() bool { return true }

func (e End) draw(l Layout) []line {
	mark := s("◇", accent)
	if e.Landed {
		mark = s("◆", accent)
	}
	var text []span
	if e.Detail != "" {
		text = []span{s(e.Detail, plain)}
	}
	lines := l.row(row{mark: mark, name: s(e.Name, bold), rail: noRail, time: e.Time, lay: l.fit(text, false)})
	if e.URL != "" {
		lines = append(lines, l.hung(noRail, l.detailCol(), []span{s(e.URL, accent)}))
	}
	return lines
}

// spinner is the running mark's frames, in turn.
var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (n Node) reached() bool { return n.State != ToCome }

func (n Node) rail() span {
	switch n.State {
	case Failed:
		return redRail
	case Running, ToCome:
		return dotted
	}
	return solid
}

func (n Node) mark() span {
	switch n.State {
	case Running:
		frame := n.Frame % len(spinner)
		if frame < 0 {
			frame += len(spinner)
		}
		return s(spinner[frame], accent)
	case ToCome, LeftAlone:
		return s("○", dim)
	case Failed:
		return s("✗", red)
	case Warning:
		return s("!", yellow)
	case Unwind:
		return s("↩", accent)
	}
	return s("●", accent)
}

func (n Node) nameStyle() style {
	switch n.State {
	case ToCome, LeftAlone:
		return dim
	case Failed:
		return redBold
	}
	return bold
}

// detailStyle is dim while the node runs, is to come, or was left alone.
func (n Node) detailStyle() style {
	switch n.State {
	case Running, ToCome, LeftAlone:
		return dim
	}
	return plain
}

// detail is the detail's first line, then the tail.
func (n Node) detail(first string) []span {
	var out []span
	if first != "" {
		out = append(out, s(first, n.detailStyle()))
	}
	switch {
	case n.Tail == "":
	case len(out) > 0:
		out = append(out, s(" · "+n.Tail, dim))
	default:
		out = append(out, s(n.Tail, dim))
	}
	return out
}

func (n Node) draw(l Layout) []line {
	live := n.State == Running
	details := textLines(n.Detail)
	lay := l.fit(n.detail(details[0]), live)
	if n.Progress != nil {
		lay = func(first, _ int) [][]span { return [][]span{n.Progress.fit(first)} }
	}
	lines := l.row(row{mark: n.mark(), name: s(n.Name, n.nameStyle()), rail: n.rail(), time: n.Time, live: live, lay: lay})
	for _, text := range details[1:] {
		lines = append(lines, l.hangText(text, n.detailStyle(), false, n.rail())...)
	}
	for _, h := range n.Lines {
		lines = append(lines, l.hang(h, n.rail())...)
	}
	return lines
}

const (
	// barWidth is the progress bar's length in cells, and minBar the least it
	// shrinks to before its label is cut.
	barWidth = 24
	minBar   = 12
	barGap   = 2
)

// fit is the bar in room cells: its label, then the bar, the bar shrunk before the
// label is cut, and the label cut before the bar.
func (p Progress) fit(room int) []span {
	label := []span{s(p.Label, dim)}
	width := min(barWidth, max(minBar, room-line(label).width()-barGap), room)
	bar := p.bar(width)
	labelRoom := room - width - barGap
	if p.Label == "" || labelRoom < 1 {
		return bar
	}
	return append(append(cut(label, labelRoom), s(strings.Repeat(" ", barGap), dim)), bar...)
}

func (p Progress) bar(width int) []span {
	filled := 0
	if p.Total > 0 {
		filled = min(max(int(math.Round(float64(width*p.Done)/float64(p.Total))), 0), width)
	}
	return []span{s(strings.Repeat("━", filled), accent), s(strings.Repeat("─", width-filled), dim)}
}

// hang is h's lines hanging off a node whose rail is rail.
func (l Layout) hang(h Hang, rail span) []line {
	switch h.Kind {
	case Output:
		return l.hangText(h.Text, plain, true, rail)
	case Path:
		return l.hangText(h.Text, plain, false, rail)
	}
	return l.hangText(h.Text, dim, false, rail)
}

// hangText is text's lines hanging off the rail at the detail column, each wrapped
// to the width; behind a dim gutter of its own where gutter is set, so a command's
// output never reads as mint's words.
func (l Layout) hangText(text string, st style, gutter bool, rail span) []line {
	col, room := l.detailCol(), l.width-l.detailCol()
	if gutter {
		room -= cells("│ ")
	}
	var lines []line
	for _, t := range textLines(text) {
		for _, c := range wrap([]span{s(t, st)}, room, room) {
			if gutter {
				c = guttered(c)
			}
			lines = append(lines, l.hung(rail, col, c))
		}
	}
	return lines
}

func guttered(c []span) []span {
	if len(c) == 0 {
		return []span{s("│", dim)}
	}
	return append([]span{s("│ ", dim)}, c...)
}

func (Question) reached() bool { return true }

func (q Question) draw(l Layout) []line {
	if q.Under {
		return q.under(l)
	}
	return l.row(row{mark: s("?", accent), rail: dotted, lay: q.lay})
}

// lay is the question, then its keys.
func (q Question) lay(first, rest int) [][]span {
	f := newFlow(first, rest)
	for _, w := range words(glyphsOf([]span{s(q.Text, bold)})) {
		f.put(w, nil)
	}
	q.keys(f)
	return f.done()
}

// keys puts the question's choices on f, three spaces apart, a choice never split
// from its action, then, for a question typing, the input marker and a cell for the
// cursor, so what is typed reads as the answer, not one more choice.
func (q Question) keys(f *flow) {
	apart := glyphsOf([]span{s("   ", plain)})
	for _, c := range q.Choices {
		f.put(glyphsOf([]span{s(c.Key, accentBold), s(" "+c.Action, plain)}), apart)
	}
	if q.Typing {
		f.put(glyphsOf([]span{s(inputMarker, accent), s("  ", plain)}), glyphsOf([]span{s(" ", plain)}))
	}
}

// subIndent is how far in from its ? a question under a node draws the lines after
// its first.
const subIndent = 2

// under is the question drawn as sub-matter of the node above it, from the detail
// column with the node's rail solid down every line: the ? and the question, then,
// indented, what it is about and, on a line of its own, its keys.
func (q Question) under(l Layout) []line {
	room := l.width - l.detailCol() - subIndent
	indent := s(strings.Repeat(" ", subIndent), plain)
	var lines []line
	add := func(lead span, content [][]span) {
		for _, c := range content {
			lines = append(lines, l.hung(solid, l.detailCol(), append([]span{lead}, c...)))
			lead = indent
		}
	}
	add(s("? ", accent), wrap([]span{s(q.Text, bold)}, room, room))
	if q.About != "" {
		add(indent, wrap([]span{s(q.About, dim)}, room, room))
	}
	if len(q.Choices) > 0 || q.Typing {
		f := newFlow(room, room)
		q.keys(f)
		add(indent, f.done())
	}
	return lines
}

// inputMarker stands before what is typed at a question.
const inputMarker = "›"

// ruleLabel is the point of no return's own words, the rail's to draw.
const ruleLabel = "━━ point of no return "

func (r Rule) reached() bool { return r.Reached || r.Crossed }

func (r Rule) draw(l Layout) []line {
	mark := s("┿", dim)
	if r.Crossed {
		mark = s("┿", accent)
	}
	rest := strings.Repeat("━", l.width-cells(ruleLabel)-2)
	return []line{{margin, mark, s(ruleLabel+rest, dim)}}
}

func (Body) reached() bool { return true }

func (b Body) draw(l Layout) []line {
	room := l.width - bodyCol
	var lines []line
	for _, text := range textLines(b.Text) {
		for _, c := range wrap([]span{s(text, plain)}, room, room) {
			lines = append(lines, l.hung(solid, bodyCol, c))
		}
	}
	return lines
}

func (Next) reached() bool { return true }

func (n Next) draw(l Layout) []line {
	lines := []line{{margin, s("next", accentBold)}}
	room := l.width - leadCol
	for _, m := range n.Moves {
		for _, c := range wrap([]span{s(m.Lead, plain)}, room, room) {
			lines = append(lines, append(line{}.to(leadCol), c...))
		}
		if m.Command != "" {
			lines = append(lines, append(line{}.to(commandCol), s(m.Command, accent)))
		}
	}
	return lines
}
