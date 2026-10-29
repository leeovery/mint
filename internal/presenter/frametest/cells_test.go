package frametest_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter/frametest"
)

var (
	none    = frametest.Pen{FG: frametest.Default, BG: frametest.Default}
	mint    = frametest.Colour("#5FD7AF")
	cyan    = frametest.Colour("ansi:6")
	colour  = frametest.Colour("256:79")
	brightR = frametest.Colour("ansi:9")
)

func pen(fg frametest.Colour, bold, faint bool) frametest.Pen {
	return frametest.Pen{FG: fg, BG: frametest.Default, Bold: bold, Faint: faint}
}

// inked is a row of text's cells, each drawn in p, a wide glyph's second cell
// empty.
func inked(text string, p frametest.Pen) []frametest.Cell {
	var row []frametest.Cell
	for _, r := range text {
		row = append(row, frametest.Cell{Glyph: string(r), Pen: p})
	}
	return row
}

func TestParseReadsSGRAsATerminalDoes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, frame string
		want        frametest.Grid
	}{
		{
			name:  "bold and faint, each set alone, both shut by 22",
			frame: "\x1b[1mA\x1b[2mB\x1b[22mC",
			want:  frametest.Grid{slices.Concat(inked("A", pen(frametest.Default, true, false)), inked("B", pen(frametest.Default, true, true)), inked("C", none))},
		},
		{
			name:  "a colour at each depth, each shut by 39",
			frame: "\x1b[38;2;95;215;175mA\x1b[38;5;79mB\x1b[36mC\x1b[91mD\x1b[39mE",
			want:  frametest.Grid{slices.Concat(inked("A", pen(mint, false, false)), inked("B", pen(colour, false, false)), inked("C", pen(cyan, false, false)), inked("D", pen(brightR, false, false)), inked("E", none))},
		},
		{
			name:  "a background, shut by 49",
			frame: "\x1b[44mA\x1b[49mB\x1b[48;5;79mC\x1b[0mD",
			want: frametest.Grid{slices.Concat(
				inked("A", frametest.Pen{FG: frametest.Default, BG: "ansi:4"}), inked("B", none),
				inked("C", frametest.Pen{FG: frametest.Default, BG: colour}), inked("D", none),
			)},
		},
		{
			name:  "parameters together, reset by an empty SGR",
			frame: "\x1b[1;38;5;79mAb\x1b[m c",
			want:  frametest.Grid{slices.Concat(inked("Ab", pen(colour, true, false)), inked(" c", none))},
		},
		{
			name:  "carried across lines",
			frame: "\x1b[2mA\nB\x1b[0m\nC",
			want:  frametest.Grid{inked("A", pen(frametest.Default, false, true)), inked("B", pen(frametest.Default, false, true)), inked("C", none)},
		},
		{
			name:  "a wide glyph, two cells",
			frame: "✨A",
			want:  frametest.Grid{{{Glyph: "✨", Pen: none}, {Pen: none}, {Glyph: "A", Pen: none}}},
		},
		{
			name:  "a last newline, ending the last row",
			frame: "A\n\nB\n",
			want:  frametest.Grid{inked("A", none), nil, inked("B", none)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := frametest.Parse(tt.frame)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(got, tt.want, slices.Equal) {
				t.Errorf("Parse(%q) = %v, want %v", tt.frame, got, tt.want)
			}
		})
	}
}

func TestParseRefusesWhatItWouldReadOtherwiseThanATerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, frame, want string
	}{
		{name: "underlined", frame: "AB\x1b[4mC", want: `row 1, column 3: SGR "4": attribute 4 isn't read`},
		{name: "reversed", frame: "\x1b[7mA", want: "attribute 7 isn't read"},
		{name: "a colour of no depth it knows", frame: "\x1b[38;3;1mA", want: "neither 5;n nor 2;r;g;b"},
		{name: "a colour cut short", frame: "\x1b[48;2;1;2mA", want: "neither 5;n nor 2;r;g;b"},
		{name: "a channel past 255", frame: "\x1b[38;2;1;2;256mA", want: "neither 5;n nor 2;r;g;b"},
		{name: "a colour's parts apart by colons", frame: "\x1b[38:2::1:2:3mA", want: `parameter "38:2::1:2:3" isn't a number`},
		{name: "an escape that isn't SGR", frame: "A\n\x1b[2JB", want: `row 2, column 1: escape "\x1b[2J" isn't SGR`},
		{name: "the cursor moved up", frame: "\x1b[3A", want: `escape "\x1b[3A" isn't SGR`},
		{name: "a hyperlink", frame: "\x1b]8;;x\x1b\\A", want: `escape "\x1b]" isn't SGR`},
		{name: "an escape never ended", frame: "A\x1b[1", want: "never ends"},
		{name: "a tab", frame: "A\tB", want: `row 1, column 2: control character '\t'`},
		{name: "a carriage return", frame: "A\r\nB", want: `control character '\r'`},
		{name: "a mark that takes no cell", frame: "́", want: "takes 0 cells"},
		{name: "bytes that aren't UTF-8", frame: "A\xff", want: "not UTF-8"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g, err := frametest.Parse(tt.frame)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse(%q) = %v, error %v, want an error with %q", tt.frame, g, err, tt.want)
			}
		})
	}
}

func TestTextIsTheGlyphsTrimmed(t *testing.T) {
	t.Parallel()
	g, err := frametest.Parse(" \x1b[1m✨ A\x1b[22m  \n\x1b[2m  \x1b[22m")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := g.Text(), []string{" ✨ A", ""}; !slices.Equal(got, want) {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestCompareHoldsEachCellToItsFrame(t *testing.T) {
	t.Parallel()
	frame, err := frametest.Parse("\x1b[1;36mAb\x1b[22;39m c\n\x1b[2md\x1b[22m✨")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(g frametest.Grid) frametest.Grid
		// want are the cells found to differ, each as its row and column.
		want [][2]int
	}{
		{name: "the frame itself", change: func(g frametest.Grid) frametest.Grid { return g }},
		{name: "a glyph changed", change: func(g frametest.Grid) frametest.Grid { g[0][1].Glyph = "B"; return g }, want: [][2]int{{1, 2}}},
		{name: "a foreground changed", change: func(g frametest.Grid) frametest.Grid { g[0][0].FG = mint; return g }, want: [][2]int{{1, 1}}},
		{name: "bold dropped", change: func(g frametest.Grid) frametest.Grid { g[0][1].Bold = false; return g }, want: [][2]int{{1, 2}}},
		{name: "faint dropped", change: func(g frametest.Grid) frametest.Grid { g[1][0].Faint = false; return g }, want: [][2]int{{2, 1}}},
		{name: "a background painted", change: func(g frametest.Grid) frametest.Grid { g[0][3].BG = cyan; return g }, want: [][2]int{{1, 4}}},
		{name: "a blank's foreground and weight", change: func(g frametest.Grid) frametest.Grid { g[0][2].FG = mint; g[0][2].Bold = true; return g }},
		{name: "a row cut short of a glyph", change: func(g frametest.Grid) frametest.Grid { g[0] = g[0][:3]; return g }, want: [][2]int{{1, 4}}},
		{name: "a row longer by blanks only", change: func(g frametest.Grid) frametest.Grid {
			g[0] = append(g[0], frametest.Cell{Glyph: " ", Pen: none})
			return g
		}},
		{name: "a wide glyph made narrow", change: func(g frametest.Grid) frametest.Grid { g[1] = g[1][:2]; g[1][1].Glyph = "*"; return g }, want: [][2]int{{2, 2}, {2, 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			drawn := tt.change(clone(frame))
			diffs, err := frametest.Compare(frame, drawn)
			if err != nil {
				t.Fatal(err)
			}
			var got [][2]int
			for _, d := range diffs {
				got = append(got, [2]int{d.Row, d.Col})
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Compare found the cells %v differing, want %v", got, tt.want)
			}
		})
	}
}

func TestCompareRefusesAGridOfOtherRows(t *testing.T) {
	t.Parallel()
	frame := frametest.Grid{inked("ab", none), inked("cd", none)}
	if _, err := frametest.Compare(frame, frame[:1]); err == nil || err.Error() != "drawn 1 rows, where the frame has 2" {
		t.Errorf("Compare error = %v, want the rows counted", err)
	}
}

func TestDescribeNamesTheFirstTwentyCellsThenCountsTheRest(t *testing.T) {
	t.Parallel()
	frame := frametest.Cell{Glyph: "b", Pen: pen(mint, true, false)}
	drawn := frametest.Cell{Glyph: "x", Pen: pen(frametest.Default, false, true)}
	diffs := make([]frametest.Difference, 23)
	for i := range diffs {
		diffs[i] = frametest.Difference{Row: 2, Col: i + 1, Frame: frame, Drawn: drawn}
	}
	got := strings.Split(frametest.Describe("failed-100", diffs), "\n")
	want := []string{
		"failed-100 draws 23 cells otherwise than its frame:",
		`  row 2, column 1: the frame's "b" #5FD7AF on default bold, drawn "x" default on default faint`,
	}
	if !slices.Equal(got[:2], want) {
		t.Errorf("Describe starts\n%s\nwant\n%s", strings.Join(got[:2], "\n"), strings.Join(want, "\n"))
	}
	if last := got[len(got)-1]; len(got) != 22 || last != "  and 3 more" {
		t.Errorf("Describe is %d lines, the last %q, want 22, the last %q", len(got), last, "  and 3 more")
	}
	if one := frametest.Describe("x", diffs[:1]); strings.Contains(one, "more") {
		t.Errorf("Describe of one cell says %q, want no count of more", one)
	}
}

func clone(g frametest.Grid) frametest.Grid {
	c := make(frametest.Grid, len(g))
	for i, row := range g {
		c[i] = slices.Clone(row)
	}
	return c
}
