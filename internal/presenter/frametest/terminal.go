package frametest

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Terminal is a terminal a test writes to: a screen of cells Width wide and Height
// tall, with the rows that scroll off its top kept above it, as a terminal's
// scrollback keeps them. It applies what a presenter writes as a terminal would,
// newlines through the output processing a terminal's driver does (a newline is a
// carriage return and a line feed), and holds the first thing it can't apply as one,
// an escape or control character it doesn't model included, as its error rather than
// a guess. Writes from goroutines are taken one at a time.
type Terminal struct {
	mu            sync.Mutex
	width, height int
	scrollback    Grid
	screen        Grid
	row, col      int
	pen           Pen
	err           error
}

// NewTerminal is a blank terminal width cells wide and height tall, its cursor at the
// top left.
func NewTerminal(width, height int) *Terminal {
	t := &Terminal{width: width, height: height, pen: blank.Pen}
	for range height {
		t.screen = append(t.screen, t.blankRow())
	}
	return t
}

func (t *Terminal) blankRow() []Cell {
	row := make([]Cell, t.width)
	for i := range row {
		row[i] = blank
	}
	return row
}

// Write applies p. Its error is the first the terminal met, in this write or one
// before: a terminal that has gone wrong stays wrong.
func (t *Terminal) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.err == nil {
		t.err = t.apply(string(p))
	}
	return len(p), t.err
}

// Err is the first thing the terminal was written that it couldn't apply, nil for
// none.
func (t *Terminal) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// Cursor is where the cursor is on the screen, its row and column counted from 0.
func (t *Terminal) Cursor() (row, col int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.row, min(t.col, t.width-1)
}

// Grid is everything written, the scrollback and then the screen, each row to its
// last cell that isn't blank and the blank rows at the foot left off.
func (t *Terminal) Grid() Grid {
	t.mu.Lock()
	defer t.mu.Unlock()
	var g Grid
	for _, row := range append(append(Grid{}, t.scrollback...), t.screen...) {
		g = append(g, trimmed(row))
	}
	for len(g) > 0 && len(g[len(g)-1]) == 0 {
		g = g[:len(g)-1]
	}
	return g
}

// ANSI is Grid drawn as the ANSI that draws it, for Check to hold to a golden frame:
// Parse reads it back to the same cells.
func (t *Terminal) ANSI() string {
	var b strings.Builder
	pen := blank.Pen
	for i, row := range t.Grid() {
		if i > 0 {
			b.WriteString("\n")
		}
		for _, c := range row {
			if c.Glyph == "" {
				continue
			}
			if c.Pen != pen {
				b.WriteString(sgr(c.Pen))
				pen = c.Pen
			}
			b.WriteString(c.Glyph)
		}
	}
	if pen != blank.Pen {
		b.WriteString(sgr(blank.Pen))
	}
	return b.String()
}

func trimmed(row []Cell) []Cell {
	end := len(row)
	for end > 0 && row[end-1].Glyph == " " && row[end-1].BG == Default {
		end--
	}
	return append([]Cell(nil), row[:end]...)
}

// sgr is the SGR that sets the pen to p from any other.
func sgr(p Pen) string {
	codes := []string{"0"}
	if p.Bold {
		codes = append(codes, "1")
	}
	if p.Faint {
		codes = append(codes, "2")
	}
	if p.FG != Default {
		codes = append(codes, colourCode(p.FG, 30, 90, 38))
	}
	if p.BG != Default {
		codes = append(codes, colourCode(p.BG, 40, 100, 48))
	}
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

// colourCode is c as the SGR parameters that set it: base and bright the codes of the
// theme's first and second eight, extended the code that takes an xterm-256 or 24-bit
// colour after it.
func colourCode(c Colour, base, bright, extended int) string {
	s := string(c)
	switch {
	case strings.HasPrefix(s, "ansi:"):
		n, _ := strconv.Atoi(strings.TrimPrefix(s, "ansi:"))
		if n < 8 {
			return strconv.Itoa(base + n)
		}
		return strconv.Itoa(bright + n - 8)
	case strings.HasPrefix(s, "256:"):
		return fmt.Sprintf("%d;5;%s", extended, strings.TrimPrefix(s, "256:"))
	}
	var r, g, b int
	_, _ = fmt.Sscanf(s, "#%02X%02X%02X", &r, &g, &b)
	return fmt.Sprintf("%d;2;%d;%d;%d", extended, r, g, b)
}

func (t *Terminal) apply(s string) error {
	if !utf8.ValidString(s) {
		return errors.New("not UTF-8")
	}
	state := -1
	for rest := s; rest != ""; {
		var err error
		switch rest[0] {
		case '\x1b':
			var size int
			size, err = t.escape(rest)
			rest, state = rest[size:], -1
		case '\r':
			t.col = 0
			rest, state = rest[1:], -1
		case '\n':
			t.col = 0
			t.lineFeed()
			rest, state = rest[1:], -1
		default:
			var cluster string
			var width int
			cluster, rest, width, state = uniseg.FirstGraphemeClusterInString(rest, state)
			err = t.put(cluster, width)
		}
		if err != nil {
			return fmt.Errorf("at row %d, column %d of the screen: %w", t.row+1, t.col+1, err)
		}
	}
	return nil
}

// escape applies the CSI sequence s starts with, and returns its length in bytes:
// SGR, the cursor up (A) or forward (C), and the erase from the cursor to the end of
// the screen (J), the moves a live region makes. Any other is an error.
func (t *Terminal) escape(s string) (int, error) {
	if !strings.HasPrefix(s, "\x1b[") {
		return 0, fmt.Errorf("escape %q isn't CSI", s[:min(len(s), 2)])
	}
	end := strings.IndexFunc(s[2:], func(r rune) bool { return r >= 0x40 && r <= 0x7e })
	if end < 0 {
		return 0, fmt.Errorf("escape %q never ends", s)
	}
	params, final, size := s[2:2+end], s[2+end], 3+end
	if final == 'm' {
		if err := t.pen.set(params); err != nil {
			return 0, fmt.Errorf("SGR %q: %w", params, err)
		}
		return size, nil
	}
	return size, t.move(params, final, s[:size])
}

func (t *Terminal) move(params string, final byte, seq string) error {
	switch final {
	case 'A', 'C':
		n, err := count(params)
		if err != nil {
			return fmt.Errorf("escape %q: %w", seq, err)
		}
		if final == 'C' {
			t.col = min(t.col+n, t.width-1)
			return nil
		}
		if n > t.row {
			return fmt.Errorf("escape %q moves the cursor up past the screen's top, into rows scrolled away", seq)
		}
		t.row, t.col = t.row-n, min(t.col, t.width-1)
		return nil
	case 'J':
		if params != "" && params != "0" {
			return fmt.Errorf("escape %q erases more than from the cursor down", seq)
		}
		t.eraseBelow()
		return nil
	}
	return fmt.Errorf("escape %q isn't modelled", seq)
}

// count is a cursor move's count: its parameter, 1 where it has none, as a terminal
// reads it; 0, which a terminal reads as 1, is refused as the slip it would be.
func count(params string) (int, error) {
	if params == "" {
		return 1, nil
	}
	n, err := strconv.Atoi(params)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("count %q isn't a number of 1 or more", params)
	}
	return n, nil
}

func (t *Terminal) eraseBelow() {
	col := min(t.col, t.width-1)
	for c := col; c < t.width; c++ {
		t.screen[t.row][c] = blank
	}
	for r := t.row + 1; r < t.height; r++ {
		t.screen[r] = t.blankRow()
	}
}

// put draws cluster at the cursor in the pen, wrapping to the next row first where
// it doesn't fit on this one. The cursor may rest past the last column, as a
// terminal's waits there to wrap.
func (t *Terminal) put(cluster string, width int) error {
	r, _ := utf8.DecodeRuneInString(cluster)
	switch {
	case r < ' ' || r == 0x7f || r >= 0x80 && r <= 0x9f:
		return fmt.Errorf("control character %q", r)
	case width == 0:
		return join(t.screen[t.row][:t.col], cluster)
	case width > 2:
		return fmt.Errorf("%q takes %d cells", cluster, width)
	}
	if t.col+width > t.width {
		t.col = 0
		t.lineFeed()
	}
	t.screen[t.row][t.col] = Cell{Glyph: cluster, Pen: t.pen}
	if width == 2 {
		t.screen[t.row][t.col+1] = Cell{Pen: t.pen}
	}
	t.col += width
	return nil
}

func (t *Terminal) lineFeed() {
	if t.row < t.height-1 {
		t.row++
		return
	}
	t.scrollback = append(t.scrollback, t.screen[0])
	t.screen = append(t.screen[1:], t.blankRow())
}
