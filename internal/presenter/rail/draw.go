package rail

import (
	"math"
	"strings"
)

// minWhere is the least of the header's where worth showing once it's cut, as
// agntc's is; shorter, where goes and the version stays.
const minWhere = 12

func (h Header) reached() bool { return true }

func (h Header) draw(l Layout) []line {
	left := line{margin, s("mint", accentBold), s(" ", plain), s("─", dim), s(" "+h.Command, plain)}
	right := h.right(l.width - left.width() - timeGap)
	if right == "" {
		return []line{left}
	}
	return []line{left.right(s(right, dim), l.width)}
}

// right is the header's right side in room cells: where and the version, else
// where cut from the left, else the version alone, else nothing.
func (h Header) right(room int) string {
	var version, suffix string
	if h.Version != "" {
		version = "v" + h.Version
		suffix = " · " + version
	}
	if h.Where == "" {
		return fitsOrNone(version, room)
	}
	if whole := h.Where + suffix; cells(whole) <= room {
		return whole
	}
	if whereRoom := room - cells(suffix); whereRoom >= minWhere {
		return cutStart(h.Where, whereRoom) + suffix
	}
	return fitsOrNone(version, room)
}

func fitsOrNone(text string, room int) string {
	if cells(text) > room {
		return ""
	}
	return text
}

func (st Station) reached() bool { return st.Kind != Pending }

func (st Station) rail() span {
	switch st.Kind {
	case Start:
		return solid
	case Pending:
		return dotted
	}
	return noRail
}

func (st Station) draw(l Layout) []line {
	mark, name, detail := s("◇", accent), bold, plain
	switch st.Kind {
	case Start:
		detail = dim
	case Pending:
		mark, name, detail = s("◇", dim), dim, dim
	case Landed:
		mark = s("◆", accent)
	}
	var text []span
	if st.Detail != "" {
		text = []span{s(st.Detail, detail)}
	}
	lines := l.row(row{mark: mark, name: s(st.Name, name), rail: st.rail(), time: st.Time, lay: l.fit(text, false)})
	if st.URL != "" {
		lines = append(lines, hung(st.rail(), l.detailCol(), []span{s(st.URL, accent)}))
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
		return s(spinner[n.Frame%len(spinner)], accent)
	case ToCome:
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
	case ToCome:
		return dim
	case Failed:
		return redBold
	}
	return bold
}

// detail is the node's detail and tail, or its bar: dim while it runs or is to
// come.
func (n Node) detail() []span {
	if n.Progress != nil {
		return n.Progress.spans()
	}
	st := plain
	if n.State == Running || n.State == ToCome {
		st = dim
	}
	var out []span
	if n.Detail != "" {
		out = append(out, s(n.Detail, st))
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
	lines := l.row(row{
		mark: n.mark(),
		name: s(n.Name, n.nameStyle()),
		rail: n.rail(),
		time: n.Time,
		live: live,
		lay:  l.fit(n.detail(), live),
	})
	for _, h := range n.Lines {
		lines = append(lines, l.hang(h, n.rail())...)
	}
	return lines
}

// barWidth is the progress bar's length in cells.
const barWidth = 24

func (p Progress) spans() []span {
	filled := 0
	if p.Total > 0 {
		filled = min(max(int(math.Round(float64(barWidth*p.Done)/float64(p.Total))), 0), barWidth)
	}
	bar := []span{s(strings.Repeat("━", filled), accent), s(strings.Repeat("─", barWidth-filled), dim)}
	if p.Label == "" {
		return bar
	}
	return append([]span{s(p.Label+"  ", dim)}, bar...)
}

// hang is h's lines hanging off a node whose rail is rail: output behind a dim
// gutter of its own, so it never reads as mint's words.
func (l Layout) hang(h Hang, rail span) []line {
	col := l.detailCol()
	st, gutter := dim, 0
	switch h.Kind {
	case Output:
		st, gutter = plain, cells("│ ")
	case Path:
		st = plain
	}
	room := l.width - col - gutter
	var lines []line
	for _, text := range textLines(h.Text) {
		for _, c := range wrap([]span{s(text, st)}, room, room) {
			if gutter > 0 {
				c = guttered(c)
			}
			lines = append(lines, hung(rail, col, c))
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

func (q Question) reached() bool { return true }

func (q Question) draw(l Layout) []line {
	return l.row(row{mark: s("?", accent), name: s(q.Name, bold), rail: dotted, lay: q.lay})
}

// lay is the question, then its choices three spaces apart, a choice never split
// from its action.
func (q Question) lay(first, rest int) [][]span {
	f := newFlow(first, rest)
	for _, w := range words(glyphsOf([]span{s(q.Text, bold)})) {
		f.put(w, nil)
	}
	sep := glyphsOf([]span{s("   ", plain)})
	for _, c := range q.Choices {
		f.put(glyphsOf([]span{s(c.Key, accentBold), s(" "+c.Action, plain)}), sep)
	}
	return f.done()
}

// ruleLabel is the point of no return's own words, the rail's to draw.
const ruleLabel = "━━ point of no return "

func (r Rule) reached() bool { return r.Crossed }

func (r Rule) draw(l Layout) []line {
	mark := s("┿", dim)
	if r.Crossed {
		mark = s("┿", accent)
	}
	rest := strings.Repeat("━", l.width-cells(ruleLabel)-2)
	return []line{{margin, mark, s(ruleLabel+rest, dim)}}
}

func (b Body) reached() bool { return true }

func (b Body) draw(l Layout) []line {
	room := l.width - bodyCol
	var lines []line
	for _, text := range textLines(b.Text) {
		for _, c := range wrap([]span{s(text, plain)}, room, room) {
			lines = append(lines, hung(solid, bodyCol, c))
		}
	}
	return lines
}

func (n Next) reached() bool { return true }

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
