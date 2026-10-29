package frametest_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter/frametest"
)

// written is what a terminal width by height shows once writes are applied, a line a
// row with the scrollback first, failing on the first write it can't apply.
func written(t *testing.T, width, height int, writes ...string) *frametest.Terminal {
	t.Helper()
	term := frametest.NewTerminal(width, height)
	for _, w := range writes {
		if _, err := term.Write([]byte(w)); err != nil {
			t.Fatalf("writing %q: %v", w, err)
		}
	}
	return term
}

func TestTheTerminalAppliesWhatALiveRegionWrites(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		writes []string
		want   []string
	}{
		{
			name:   "lines, a newline returning to the first column",
			writes: []string{"ab\ncd\n"},
			want:   []string{"ab", "cd"},
		},
		{
			name:   "a carriage return writing over its row",
			writes: []string{"abcd\rxy"},
			want:   []string{"xycd"},
		},
		{
			name:   "a region redrawn: up its height, erased below, drawn again",
			writes: []string{"settle\nold 1\nold 2\n", "\r\x1b[2A\x1b[Jnew\n"},
			want:   []string{"settle", "new"},
		},
		{
			name:   "erased from mid-row to the screen's end",
			writes: []string{"keep\nabcdef\nmore\n", "\x1b[2A\x1b[3C\x1b[J"},
			want:   []string{"keep", "abc"},
		},
		{
			name:   "a row filled to its last column then ended takes one row, not two",
			writes: []string{"abcdef\nx\n"},
			want:   []string{"abcdef", "x"},
		},
		{
			name:   "a row too long wrapping onto the next",
			writes: []string{"abcdefgh\n"},
			want:   []string{"abcdef", "gh"},
		},
		{
			name:   "a wide glyph that won't fit on the row wrapping whole",
			writes: []string{"abcde✨\n"},
			want:   []string{"abcde", "✨"},
		},
		{
			name:   "rows scrolled off the top kept above the screen",
			writes: []string{"1\n2\n3\n4\n5\n"},
			want:   []string{"1", "2", "3", "4", "5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := written(t, 6, 3, tt.writes...)
			if got := term.Grid().Text(); !slices.Equal(got, tt.want) {
				t.Errorf("the terminal shows\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestTheCursorIsWhereTheWritesLeftIt(t *testing.T) {
	t.Parallel()
	term := written(t, 20, 5, "one\ntwo\nthree\n", "\r\x1b[2A\x1b[4C")
	if row, col := term.Cursor(); row != 1 || col != 4 {
		t.Errorf("Cursor() = %d, %d, want row 1, column 4", row, col)
	}
}

func TestTheScreenIsTheRowsShownWithoutTheScrollback(t *testing.T) {
	t.Parallel()
	term := written(t, 20, 3, "one\ntwo\nthree\nfour")
	if got, want := term.Screen(), []string{"two", "three", "four"}; !slices.Equal(got, want) {
		t.Errorf("Screen() = %q, want %q", got, want)
	}
}

func TestTheTerminalRefusesWhatALiveRegionMustNeverWrite(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, write, want string
	}{
		{name: "the cursor up into rows scrolled away", write: "1\n2\n3\n4\n\x1b[4A", want: "past the screen's top"},
		{name: "the screen cleared", write: "\x1b[2J", want: `escape "\x1b[2J" erases more than from the cursor down`},
		{name: "the cursor sent home", write: "\x1b[H", want: `escape "\x1b[H" isn't modelled`},
		{name: "the alternate screen", write: "\x1b[?1049h", want: `escape "\x1b[?1049h" isn't modelled`},
		{name: "the cursor hidden", write: "\x1b[?25l", want: `escape "\x1b[?25l" isn't modelled`},
		{name: "a move of no rows", write: "a\n\x1b[0A", want: `count "0" isn't a number of 1 or more`},
		{name: "a tab", write: "a\tb", want: `control character '\t'`},
		{name: "an escape that isn't CSI", write: "\x1b]8;;x\x1b\\", want: "isn't CSI"},
		{name: "an SGR attribute it doesn't read", write: "\x1b[4mA", want: "attribute 4 isn't read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			term := frametest.NewTerminal(10, 3)
			_, _ = term.Write([]byte(tt.write))
			if err := term.Err(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("writing %q: error %v, want one with %q", tt.write, err, tt.want)
			}
		})
	}
}

func TestATerminalGoneWrongStaysWrong(t *testing.T) {
	t.Parallel()
	term := frametest.NewTerminal(10, 3)
	_, _ = term.Write([]byte("\x1b[H"))
	if _, err := term.Write([]byte("fine\n")); err == nil {
		t.Error("a write after a bad one reports no error, want the first error kept")
	}
}

func TestTheTerminalsANSIReadsBackToItsCells(t *testing.T) {
	t.Parallel()
	term := written(t, 30, 4,
		" \x1b[38;2;95;215;175m●\x1b[39m  \x1b[1mpush\x1b[22m  \x1b[2m1.9s\x1b[22m\n",
		"\x1b[31m✗\x1b[39m \x1b[1;31mnotes\x1b[22;39m ✨ \x1b[38;5;79mx\x1b[39m\x1b[36my\x1b[39m\n",
		"\n plain\n")
	drawn := term.ANSI()
	got, err := frametest.Parse(drawn)
	if err != nil {
		t.Fatalf("parsing %q: %v", drawn, err)
	}
	diffs, err := frametest.Compare(term.Grid(), got)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) > 0 {
		t.Error(frametest.Describe("the terminal's ANSI", diffs))
	}
	if want := []string{" ●  push  1.9s", "✗ notes ✨ xy", "", " plain"}; !slices.Equal(got.Text(), want) {
		t.Errorf("the ANSI reads %q, want %q", got.Text(), want)
	}
}
