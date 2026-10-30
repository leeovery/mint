package frametest

import (
	"fmt"
	"strings"
)

// Difference is a cell, at its row and column counted from 1, that is drawn
// otherwise than its frame has it.
type Difference struct {
	Row, Col     int
	Frame, Drawn Cell
}

// Compare lists the cells drawn differs from frame in. A drawn row shorter than the
// frame's is filled with blank, unpainted cells, as a terminal shows what wasn't
// written; a blank's foreground, bold and faint aren't compared, as nothing shows
// them. The grids must have as many rows, and no drawn row may be wider than the
// frame's, blanks included, as it could take a row more on the terminal.
func Compare(frame, drawn Grid) ([]Difference, error) {
	if len(drawn) != len(frame) {
		return nil, fmt.Errorf("drawn %d rows, where the frame has %d", len(drawn), len(frame))
	}
	var diffs []Difference
	for r := range frame {
		if len(drawn[r]) > len(frame[r]) {
			return nil, fmt.Errorf("row %d drawn %d columns, where the frame has %d", r+1, len(drawn[r]), len(frame[r]))
		}
		for c := range frame[r] {
			want, got := frame[r][c], at(drawn[r], c)
			if !alike(want, got) {
				diffs = append(diffs, Difference{Row: r + 1, Col: c + 1, Frame: want, Drawn: got})
			}
		}
	}
	return diffs, nil
}

var blank = Cell{Glyph: " ", Pen: Pen{FG: Default, BG: Default}}

func at(row []Cell, c int) Cell {
	if c < len(row) {
		return row[c]
	}
	return blank
}

func alike(a, b Cell) bool {
	if a.Glyph != b.Glyph || a.BG != b.BG {
		return false
	}
	return a.Glyph == " " || a.Pen == b.Pen
}

// shown is how many cells a failure lists before it counts the rest.
const shown = 20

// Describe says where name is drawn otherwise than its frame: a line a cell, for
// the first 20 of them, then how many more.
func Describe(name string, diffs []Difference) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s draws %d cells otherwise than its frame:", name, len(diffs))
	for _, d := range diffs[:min(len(diffs), shown)] {
		fmt.Fprintf(&b, "\n  row %d, column %d: the frame's %s, drawn %s", d.Row, d.Col, d.Frame, d.Drawn)
	}
	if more := len(diffs) - shown; more > 0 {
		fmt.Fprintf(&b, "\n  and %d more", more)
	}
	return b.String()
}
