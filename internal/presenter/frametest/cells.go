// Package frametest holds drawn terminal output to golden frames cell by cell, as
// Switchboard's capture tests do. A frame is kept as <name>.ansi, as drawn, and
// <name>.txt, its text; Parse reads the ANSI into the cells a terminal shows, and
// Compare lists the cells two grids differ in. It is test support: only tests import
// it, so it is never linked into the binary.
package frametest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Cell is one terminal cell: its glyph, in the pen it is drawn with. A wide glyph
// takes two cells, the second with no glyph of its own.
type Cell struct {
	Glyph string
	Pen
}

// String reads as a failure names the cell.
func (c Cell) String() string {
	var attrs string
	if c.Bold {
		attrs += " bold"
	}
	if c.Faint {
		attrs += " faint"
	}
	return fmt.Sprintf("%q %s on %s%s", c.Glyph, c.FG, c.BG, attrs)
}

// Pen is what a glyph is drawn in: its colours, and whether it is bold or faint.
type Pen struct {
	FG, BG Colour
	Bold   bool
	Faint  bool
}

// Colour is a cell's colour as the SGR that set it says: the terminal's own
// ("default"), one of the theme's sixteen ("ansi:6"), an xterm-256 colour
// ("256:79") or a 24-bit one ("#5FD7AF").
type Colour string

// Default is the colour the terminal draws where none is set.
const Default Colour = "default"

// Grid is a frame's cells, a row of them a line.
type Grid [][]Cell

// Text is the grid's glyphs, a line a row, trailing blanks trimmed.
func (g Grid) Text() []string {
	lines := make([]string, len(g))
	for i, row := range g {
		var b strings.Builder
		for _, c := range row {
			b.WriteString(c.Glyph)
		}
		lines[i] = strings.TrimRight(b.String(), " ")
	}
	return lines
}

// Parse reads a frame's ANSI into its cells as a terminal draws them, each glyph in
// the pen its SGR left set, carried from cell to cell and across lines until reset.
// A newline ends a row, so one at the frame's end opens an empty last row. A glyph
// that takes no cell joins the cell before it. Anything it can't read as a terminal
// would, an escape that isn't SGR, an SGR attribute it doesn't know, a control
// character or a glyph with no cell before it to join, is an error rather than a
// guess.
func Parse(frame string) (Grid, error) {
	if !utf8.ValidString(frame) {
		return nil, errors.New("not UTF-8")
	}
	g := Grid{nil}
	pen := Pen{FG: Default, BG: Default}
	state := -1
	for rest := frame; rest != ""; {
		var err error
		switch rest[0] {
		case '\x1b':
			var size int
			size, err = pen.apply(rest)
			rest, state = rest[size:], -1
		case '\n':
			g = append(g, nil)
			rest, state = rest[1:], -1
		default:
			var cluster string
			var width int
			cluster, rest, width, state = uniseg.FirstGraphemeClusterInString(rest, state)
			err = place(&g[len(g)-1], cluster, width, pen)
		}
		if err != nil {
			return nil, fmt.Errorf("row %d, column %d: %w", len(g), len(g[len(g)-1])+1, err)
		}
	}
	return g, nil
}

func place(row *[]Cell, cluster string, width int, pen Pen) error {
	r, _ := utf8.DecodeRuneInString(cluster)
	switch {
	case r < ' ' || r == 0x7f || r >= 0x80 && r <= 0x9f:
		return fmt.Errorf("control character %q", r)
	case width == 0:
		return join(*row, cluster)
	case width > 2:
		return fmt.Errorf("%q takes %d cells", cluster, width)
	}
	*row = append(*row, Cell{Glyph: cluster, Pen: pen})
	if width == 2 {
		*row = append(*row, Cell{Pen: pen})
	}
	return nil
}

// join adds a cluster that takes no cell to the glyph of the cell before it, the
// first of a wide glyph's two.
func join(row []Cell, cluster string) error {
	for i := len(row) - 1; i >= 0; i-- {
		if row[i].Glyph != "" {
			row[i].Glyph += cluster
			return nil
		}
	}
	return fmt.Errorf("%q takes no cell, and no cell before it takes it", cluster)
}

// apply sets the pen as the SGR sequence s starts with says, and returns the
// sequence's length in bytes.
func (p *Pen) apply(s string) (int, error) {
	if !strings.HasPrefix(s, "\x1b[") {
		return 0, fmt.Errorf("escape %q isn't SGR", s[:min(len(s), 2)])
	}
	end := strings.IndexFunc(s[2:], func(r rune) bool { return r >= 0x40 && r <= 0x7e })
	if end < 0 {
		return 0, fmt.Errorf("escape %q never ends", s)
	}
	if s[2+end] != 'm' {
		return 0, fmt.Errorf("escape %q isn't SGR", s[:3+end])
	}
	if err := p.set(s[2 : 2+end]); err != nil {
		return 0, fmt.Errorf("SGR %q: %w", s[2:2+end], err)
	}
	return 3 + end, nil
}

// set sets the pen as the SGR parameters given say, in turn.
func (p *Pen) set(params string) error {
	codes := strings.Split(params, ";")
	for i := 0; i < len(codes); i++ {
		n, err := attribute(codes[i])
		if err != nil {
			return err
		}
		switch {
		case n == 0:
			*p = Pen{FG: Default, BG: Default}
		case n == 1:
			p.Bold = true
		case n == 2:
			p.Faint = true
		case n == 22:
			p.Bold, p.Faint = false, false
		case n == 39:
			p.FG = Default
		case n == 49:
			p.BG = Default
		case n >= 30 && n <= 37:
			p.FG = ansi(n - 30)
		case n >= 90 && n <= 97:
			p.FG = ansi(n - 90 + 8)
		case n >= 40 && n <= 47:
			p.BG = ansi(n - 40)
		case n >= 100 && n <= 107:
			p.BG = ansi(n - 100 + 8)
		case n == 38 || n == 48:
			c, used, err := extended(codes[i+1:])
			if err != nil {
				return err
			}
			if n == 38 {
				p.FG = c
			} else {
				p.BG = c
			}
			i += used
		default:
			return fmt.Errorf("attribute %d isn't read", n)
		}
	}
	return nil
}

func ansi(n int) Colour { return Colour("ansi:" + strconv.Itoa(n)) }

// attribute is the SGR parameter given, an empty one 0, as a terminal reads it.
func attribute(code string) (int, error) {
	if code == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		return 0, fmt.Errorf("parameter %q isn't a number", code)
	}
	return n, nil
}

var errColour = errors.New("its colour is neither 5;n nor 2;r;g;b")

// extended is the colour the parameters after 38 or 48 give, and how many of them
// it takes: 5 and an xterm-256 index, or 2 and a red, green and blue.
func extended(codes []string) (Colour, int, error) {
	if len(codes) == 0 {
		return "", 0, errColour
	}
	switch codes[0] {
	case "5":
		n, err := channels(codes[1:], 1)
		if err != nil {
			return "", 0, err
		}
		return Colour("256:" + strconv.Itoa(int(n[0]))), 2, nil
	case "2":
		n, err := channels(codes[1:], 3)
		if err != nil {
			return "", 0, err
		}
		return Colour(fmt.Sprintf("#%02X%02X%02X", n[0], n[1], n[2])), 4, nil
	}
	return "", 0, errColour
}

func channels(codes []string, count int) ([]uint8, error) {
	if len(codes) < count {
		return nil, errColour
	}
	out := make([]uint8, count)
	for i, code := range codes[:count] {
		n, err := strconv.ParseUint(code, 10, 8)
		if err != nil {
			return nil, errColour
		}
		out[i] = uint8(n)
	}
	return out, nil
}
