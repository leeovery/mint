package rail

// The layout's widths, as the doc gives them.
const (
	minWidth     = 60
	maxWidth     = 100
	unknownWidth = 80
	// narrowBelow is the width below which times drop and a detail goes on the line
	// under its name.
	narrowBelow = 70
)

// The layout's columns, counted from 0: a one-space margin, the rail, two spaces,
// the 14-column name column, then the detail.
const (
	nameCol    = 4
	nameWidth  = 14
	detailCol  = nameCol + nameWidth
	bodyCol    = 6
	leadCol    = 3
	commandCol = 5
	timeGap    = 2
	// maxTime is the most a time takes; a longer one is cut.
	maxTime = 12
)

// Layout is where a run is drawn: the terminal's width, the layout's, and the colour
// it's drawn in.
type Layout struct {
	term  int
	width int
	level Level
	// bare drops the rail, for a run with no station.
	bare bool
}

// NewLayout lays a run out for a terminal termWidth wide, 0 where the terminal
// gives none, at level.
func NewLayout(termWidth int, level Level) Layout {
	width := unknownWidth
	if termWidth > 0 {
		width = min(max(termWidth, minWidth), maxWidth)
	}
	return Layout{term: termWidth, width: width, level: level}
}

// Width is the layout's width: the column right-aligned times end on.
func (l Layout) Width() int { return l.width }

// RegionFits is whether a live region can be drawn: the terminal is as wide as the
// layout, or gives no width and is taken as 80. In a narrower one every line of the
// layout's takes two rows, so the region couldn't be counted back up; only settled
// lines are drawn there.
func (l Layout) RegionFits() bool { return l.term <= 0 || l.term >= minWidth }

// Settled is r's settled items from index from on, drawn: the lines to print under
// those already printed.
func (l Layout) Settled(r Run, from int) []string {
	var above Item
	if from > 0 {
		above = r.Settled[from-1]
	}
	return l.render(l.forRun(r).drawn(above, r.Settled[from:]))
}

// Region is r's live region, drawn: as many lines as it is tall, each one row.
func (l Layout) Region(r Run) []string {
	return l.render(l.forRun(r).drawn(lastSettled(r), region(r)))
}

// Draw is the whole of r: its settled items, then its region.
func (l Layout) Draw(r Run) []string {
	return append(l.Settled(r, 0), l.Region(r)...)
}

func lastSettled(r Run) Item {
	if n := len(r.Settled); n > 0 {
		return r.Settled[n-1]
	}
	return nil
}

func region(r Run) []Item {
	items := make([]Item, len(r.Region))
	for i, it := range r.Region {
		items[i] = it
	}
	return items
}

// forRun is l for r: bare where r has no station.
func (l Layout) forRun(r Run) Layout {
	l.bare = true
	for _, it := range append(r.Settled, region(r)...) {
		switch it.(type) {
		case Station, End:
			l.bare = false
		}
	}
	return l
}

// drawn is items drawn, a group of lines an item: the line between it and the item
// above, then its own.
func (l Layout) drawn(above Item, items []Item) [][]line {
	groups := make([][]line, len(items))
	for i, it := range items {
		if gap, ok := l.between(above, it); ok {
			groups[i] = append(groups[i], gap)
		}
		groups[i] = append(groups[i], it.draw(l)...)
		above = it
	}
	return groups
}

func (l Layout) render(groups [][]line) []string {
	var out []string
	for _, group := range groups {
		for _, ln := range group {
			out = append(out, ln.render(l.level))
		}
	}
	return out
}

func (l Layout) narrow() bool { return l.width < narrowBelow }

// detailCol is where a detail under its name, and every line hanging off a node,
// starts.
func (l Layout) detailCol() int {
	if l.narrow() {
		return nameCol
	}
	return detailCol
}

var margin = s(" ", plain)

// between is the line drawn between above and it, if any: a blank under the header
// and over next; elsewhere the rail alone, under a start station and around a body,
// over the rule and an end station, and where the run's reach ends. It is solid
// where the run has come both sides of it, dotted otherwise.
func (l Layout) between(above, it Item) (line, bool) {
	if above == nil {
		return nil, false
	}
	_, header := above.(Header)
	_, next := it.(Next)
	if header || next {
		return line{}, true
	}
	if l.bare || !gapped(above, it) {
		return nil, false
	}
	if above.reached() && it.reached() {
		return line{margin, solid}, true
	}
	return line{margin, dotted}, true
}

func gapped(above, it Item) bool {
	switch i := it.(type) {
	case Rule, Body, End:
		return true
	case Station:
		return i.Kind != Start
	}
	switch a := above.(type) {
	case Body:
		return true
	case Station:
		return a.Kind == Start
	}
	return above.reached() && !it.reached()
}

// The rail's glyphs: solid where the run has been, dotted below the live node, red
// down a failed node's lines, and none under the end station.
var (
	solid   = s("│", accent)
	dotted  = s("┆", dim)
	redRail = s("│", red)
	noRail  = s(" ", plain)
)

// hung is a line hanging off the rail: rail in its column, then content from col.
func (l Layout) hung(rail span, col int, content []span) line {
	if l.bare {
		rail = noRail
	}
	ln := line{margin, rail}
	switch {
	case len(content) > 0:
		return append(ln.to(col), content...)
	case rail == noRail:
		return line{}
	}
	return ln
}

// row is the head of a node or a station: its mark and name, its content beside the
// name or under it, its time on the right of its first line.
type row struct {
	mark span
	name span
	rail span
	time string
	// live cuts the row's lines to the width rather than wrapping them.
	live bool
	// lay lays the content out in lines first wide, then rest.
	lay func(first, rest int) [][]span
}

func (l Layout) row(r row) []line {
	var time []span
	if r.time != "" && !l.narrow() {
		time = cut([]span{s(r.time, dim)}, maxTime)
	}
	right := l.width
	if time != nil {
		right -= line(time).width() + timeGap
	}
	head := line{margin, r.mark}.to(nameCol)
	var lines []line
	if col, inline := l.inline(r.name.text); inline {
		content := r.lay(right-col, l.width-col)
		first := head
		if r.name.text != "" {
			first = append(first, r.name)
		}
		lines = append(lines, hungFrom(first, col, content[0]))
		for _, c := range content[1:] {
			lines = append(lines, l.hung(r.rail, col, c))
		}
	} else {
		names := l.fit([]span{r.name}, r.live)(right-nameCol, l.width-nameCol)
		lines = append(lines, append(head, names[0]...))
		for _, c := range names[1:] {
			lines = append(lines, l.hung(r.rail, nameCol, c))
		}
		col := l.detailCol()
		for _, c := range r.lay(l.width-col, l.width-col) {
			if len(c) > 0 {
				lines = append(lines, l.hung(r.rail, col, c))
			}
		}
	}
	if time != nil {
		lines[0] = lines[0].right(time, l.width)
	}
	return lines
}

// inline is whether a row's content sits beside its name, and the column it starts
// at: after the name column where the name leaves a space in it, at the name
// column where there's no name.
func (l Layout) inline(name string) (int, bool) {
	switch {
	case name == "":
		return nameCol, true
	case !l.narrow() && cells(name) < nameWidth:
		return detailCol, true
	}
	return 0, false
}

func hungFrom(head line, col int, content []span) line {
	if len(content) == 0 {
		return head
	}
	return append(head.to(col), content...)
}

// fit lays text out: wrapped, or cut to its first line where live.
func (l Layout) fit(text []span, live bool) func(first, rest int) [][]span {
	return func(first, rest int) [][]span {
		if live {
			return [][]span{cut(text, first)}
		}
		return wrap(text, first, rest)
	}
}
