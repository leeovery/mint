package rail

import (
	"math"
	"strings"
	"unicode/utf8"
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

// detailStyle is dim while the node runs, is to come, or was left alone, and red,
// every line of it, where it failed, as its mark, name and rail are.
func (n Node) detailStyle() style {
	switch n.State {
	case Running, ToCome, LeftAlone:
		return dim
	case Failed:
		return red
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

// lay is the question and any tail, then its keys on a line of their own.
func (q Question) lay(first, rest int) [][]span {
	f := newFlow(first, rest)
	for _, w := range words(glyphsOf(q.tailed([]span{s(q.Text, bold)}))) {
		f.put(w, nil)
	}
	if len(q.Choices) > 0 {
		f.next()
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
		f.put(glyphsOf(c.spans()), apart)
	}
	if q.Typing {
		f.put(glyphsOf([]span{s(inputMarker, accent), s("  ", plain)}), glyphsOf([]span{s(" ", plain)}))
	}
}

// defaultMark follows the action of a default whose key is a word, which no capital
// can mark.
const defaultMark = " (default)"

// spans is the choice as its question draws it: its key bold in the accent, its
// action, what it is about, and the default marked.
func (c Choice) spans() []span {
	single := utf8.RuneCountInString(c.Key) == 1
	key := c.Key
	if c.Default && single {
		key = strings.ToUpper(key)
	}
	out := []span{s(key, accentBold), s(" "+c.Action, plain)}
	if c.About != "" {
		out = append(out, s(" ("+c.About+")", dim))
	}
	if c.Default && !single {
		out = append(out, s(defaultMark, dim))
	}
	return out
}

// tailed is last, the question's last line, with its tail after it.
func (q Question) tailed(last []span) []span {
	if q.Tail == "" {
		return last
	}
	return append(last, s(" · "+q.Tail, dim))
}

// subIndent is how far in from its ? a question under a node draws the lines after
// its first.
const subIndent = 2

var indent = s(strings.Repeat(" ", subIndent), plain)

// sub is content hanging off the solid rail as a node's sub-matter, from the detail
// column: lead before its first line, indent before the rest.
func (l Layout) sub(lead span, content [][]span) []line {
	lines := make([]line, 0, len(content))
	for _, c := range content {
		lines = append(lines, l.hung(solid, l.detailCol(), append([]span{lead}, c...)))
		lead = indent
	}
	return lines
}

// asked is whether the question waits on its answer, as a dry run's preview never
// does.
func (q Question) asked() bool { return len(q.Choices) > 0 || q.Typing }

// under is the question drawn as sub-matter of the node above it, from the detail
// column with the node's rail solid down every line: the ? and the question, then,
// indented, what it is about and, on a line of its own, its keys. While it is asked,
// a branch joins its first line to the rail.
func (q Question) under(l Layout) []line {
	room := l.width - l.detailCol() - subIndent
	text := []span{s(q.Text, bold)}
	if q.About == "" {
		text = q.tailed(text)
	}
	question := wrap(text, room, room)
	lead := s("? ", accent)
	var lines []line
	if q.asked() {
		lines = append(lines, l.branch(append([]span{lead}, question[0]...)))
		question, lead = question[1:], indent
	}
	lines = append(lines, l.sub(lead, question)...)
	if q.About != "" {
		lines = append(lines, l.sub(indent, wrap(q.tailed([]span{s(q.About, dim)}), room, room))...)
	}
	if q.asked() {
		f := newFlow(room, room)
		q.keys(f)
		lines = append(lines, l.sub(indent, f.done())...)
	}
	return lines
}

// branch is content hanging off the solid rail at the detail column, joined to it:
// the rail's column a ├, then a dim ┄ up to a space before the content. A run with
// no rail has nothing to join it to.
func (l Layout) branch(content []span) line {
	col := l.detailCol()
	if l.bare {
		return l.hung(solid, col, content)
	}
	joined := line{margin, s("├", accent), s(strings.Repeat("┄", col-cells(" ├ ")), dim), s(" ", plain)}
	return append(joined, content...)
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
		for _, c := range wrap(b.spans(text), room, room) {
			lines = append(lines, l.hung(solid, bodyCol, c))
		}
	}
	return lines
}

// spans is a line of the body as drawn.
func (b Body) spans(text string) []span {
	if b.Markdown {
		return markdown(shown(text))
	}
	return []span{s(text, plain)}
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
