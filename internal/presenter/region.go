package presenter

import (
	"fmt"
	"io"
	"strings"
)

// unknownRows is the height a terminal that gives none is taken as.
const unknownRows = 24

// The escapes the live region moves with: back to the line's start, up n rows, and
// erase from the cursor to the end of the screen.
const (
	toLineStart = "\r"
	eraseBelow  = "\x1b[J"
)

func cursorUp(n int) string { return fmt.Sprintf("\x1b[%dA", n) }

// liveRegion owns the terminal below the last settled line: the lines settled above
// it, which never change once printed, and the region under them, which it redraws in
// place. It knows nothing of what the lines say. Each line given it must be one
// terminal row, so the rows it moves up are the lines it wrote.
//
// It moves the cursor up only within its own region, never uses the alternate
// screen and never clears the screen. The region is written with a newline after
// each line, so the cursor rests at the start of the row under it; up is how many
// rows the cursor sits below the region's first, the count a redraw moves up by.
//
// It never moves up more rows than the terminal has above the cursor's row, its
// height less one, so a terminal made shorter since the last draw takes it back only
// as far as the screen's top. A typed line's echo it never counts: the terminal
// draws it however the line was edited, so the line is read at a question printed
// last, nothing drawn below it, and nothing goes back over it.
type liveRegion struct {
	out io.Writer
	// shown is whether a region is on the terminal to be redrawn.
	shown bool
	up    int
	// rows is the terminal's height at the last draw, 0 where it gave none.
	rows int
}

func newLiveRegion(out io.Writer) *liveRegion { return &liveRegion{out: out} }

// Draw prints settled under the lines already settled, then the region under them,
// in one write: from the second draw on, it first goes back up to the region's first
// row and erases from there down. The region keeps to rows less one, the terminal's
// height less the row the cursor rests on (24 rows where rows is 0, a terminal that
// gives none): a cursor can't go back up into rows that have scrolled away. Lines
// past that are dropped from its foot. An empty region leaves nothing under the
// settled lines.
func (r *liveRegion) Draw(settled, region []string, rows int) {
	r.rows = rows
	region = region[:min(len(region), regionRows(rows))]
	var b strings.Builder
	r.back(&b)
	for _, lines := range [][]string{settled, region} {
		for _, ln := range lines {
			b.WriteString(ln)
			b.WriteString("\n")
		}
	}
	r.shown, r.up = len(region) > 0, len(region)
	r.write(b.String())
}

// regionRows is the most rows a region may take on a terminal rows tall, 0 where it
// gives none.
func regionRows(rows int) int {
	if rows <= 0 {
		rows = unknownRows
	}
	return max(rows-1, 0)
}

// Erase takes the region off the terminal, leaving the cursor where its first row
// was, as before the editor takes the terminal; the next Draw puts it back.
func (r *liveRegion) Erase() {
	var b strings.Builder
	r.back(&b)
	r.shown, r.up = false, 0
	r.write(b.String())
}

// Ask takes the region off the terminal and prints lines under those settled, the
// last with no newline after it: a question read as a typed line, the cursor at its
// end and nothing drawn below it, so the terminal's echo lands only on blank rows.
// rows is the terminal's height, as Draw takes it.
func (r *liveRegion) Ask(lines []string, rows int) {
	r.rows = rows
	var b strings.Builder
	r.back(&b)
	b.WriteString(strings.Join(lines, "\n"))
	r.shown, r.up = false, 0
	r.write(b.String())
}

// Forget drops the region without going back over it, as where the shell printed
// under it and moved the cursor on: the next Draw starts where the cursor is.
func (r *liveRegion) Forget() { r.shown, r.up = false, 0 }

// Released ends a typed read at the question Ask printed, the question and the line
// staying as typed: a line the input closed on before an Enter gets the newline it
// lacks, so what is drawn next starts on a row of its own.
func (r *liveRegion) Released(ended bool) {
	if !ended {
		r.write("\n")
	}
}

// back writes the way back to the region's first row, or the screen's top where
// that scrolled away, and the erase below it, where a region is shown.
func (r *liveRegion) back(b *strings.Builder) {
	if !r.shown {
		return
	}
	b.WriteString(toLineStart)
	if n := min(r.up, regionRows(r.rows)); n > 0 {
		b.WriteString(cursorUp(n))
	}
	b.WriteString(eraseBelow)
}

// write sends s, whose write error has nowhere to go: the presenter's methods return
// nothing, as the other presenters' writef discards it.
func (r *liveRegion) write(s string) {
	if s == "" {
		return
	}
	_, _ = io.WriteString(r.out, s)
}
