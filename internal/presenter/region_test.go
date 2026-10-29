package presenter_test

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter"
	"mint/internal/presenter/frametest"
)

// regionStep is one call on a live region, and the bytes it must write.
type regionStep struct {
	name string
	call func(r *presenter.LiveRegion)
	want string
}

func draw(settled, region []string) func(r *presenter.LiveRegion) {
	return func(r *presenter.LiveRegion) { r.Draw(settled, region, 24) }
}

func lines(n int, prefix string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("%s%d", prefix, i+1)
	}
	return out
}

func TestTheLiveRegionWritesExactlyTheseEscapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		steps []regionStep
	}{
		{
			name: "a first draw writes the region alone, a newline after each line",
			steps: []regionStep{
				{call: draw(nil, []string{"a", "b"}), want: "a\nb\n"},
			},
		},
		{
			name: "a redraw goes back up the height it wrote, erases below, and writes again",
			steps: []regionStep{
				{call: draw(nil, []string{"a", "b"}), want: "a\nb\n"},
				{name: "same height", call: draw(nil, []string{"c", "d"}), want: "\r\x1b[2A\x1b[Jc\nd\n"},
				{name: "taller", call: draw(nil, []string{"e", "f", "g"}), want: "\r\x1b[2A\x1b[Je\nf\ng\n"},
				{name: "shorter", call: draw(nil, []string{"h"}), want: "\r\x1b[3A\x1b[Jh\n"},
			},
		},
		{
			name: "settled lines print where the region was, the region under them",
			steps: []regionStep{
				{call: draw([]string{"s1"}, []string{"a", "b"}), want: "s1\na\nb\n"},
				{call: draw([]string{"s2", "s3"}, []string{"c"}), want: "\r\x1b[2A\x1b[Js2\ns3\nc\n"},
			},
		},
		{
			name: "erased for the editor, then drawn again from where it was",
			steps: []regionStep{
				{call: draw(nil, []string{"a", "b"}), want: "a\nb\n"},
				{name: "erase", call: func(r *presenter.LiveRegion) { r.Erase() }, want: "\r\x1b[2A\x1b[J"},
				{name: "erase again, nothing shown", call: func(r *presenter.LiveRegion) { r.Erase() }, want: ""},
				{name: "drawn again", call: draw(nil, []string{"a", "b"}), want: "a\nb\n"},
			},
		},
		{
			name: "a region of height 0: no move over it, and none down to it",
			steps: []regionStep{
				{call: draw(nil, []string{"a"}), want: "a\n"},
				{name: "to nothing", call: draw([]string{"s"}, nil), want: "\r\x1b[1A\x1b[Js\n"},
				{name: "nothing to go back over", call: draw([]string{"t"}, nil), want: "t\n"},
				{name: "nothing at all", call: draw(nil, nil), want: ""},
			},
		},
		{
			name: "a question for a typed line printed last, the region off, then the region drawn afresh under the line",
			steps: []regionStep{
				{call: draw(nil, []string{"a", "b"}), want: "a\nb\n"},
				{name: "asked", call: func(r *presenter.LiveRegion) { r.Ask([]string{"s", "? q "}) }, want: "\r\x1b[2A\x1b[Js\n? q "},
				{name: "the line ended in its Enter", call: func(r *presenter.LiveRegion) { r.Released(true) }, want: ""},
				{name: "drawn afresh, nothing to go back over", call: draw(nil, []string{"c"}), want: "c\n"},
			},
		},
		{
			name: "a line the input closed on before its Enter is ended",
			steps: []regionStep{
				{call: func(r *presenter.LiveRegion) { r.Ask([]string{"? q "}) }, want: "? q "},
				{name: "released", call: func(r *presenter.LiveRegion) { r.Released(false) }, want: "\n"},
			},
		},
		{
			name: "a terminal made shorter is gone back over only to the screen's top",
			steps: []regionStep{
				{call: func(r *presenter.LiveRegion) { r.Draw(nil, lines(10, "line "), 24) }, want: strings.Join(lines(10, "line "), "\n") + "\n"},
				{name: "redrawn 5 rows tall", call: func(r *presenter.LiveRegion) { r.Draw(nil, []string{"c"}, 5) }, want: "\r\x1b[4A\x1b[Jc\n"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			r := presenter.NewLiveRegion(&out)
			for i, step := range tt.steps {
				out.Reset()
				step.call(r)
				if got := out.String(); got != step.want {
					t.Errorf("step %d %s wrote %q, want %q", i+1, step.name, got, step.want)
				}
			}
		})
	}
}

func TestTheRegionKeepsToTheTerminalsHeightLessOneRow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		rows int
		want int
	}{
		{name: "a terminal 10 rows tall", rows: 10, want: 9},
		{name: "a terminal giving no height, taken as 24", rows: 0, want: 23},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			r := presenter.NewLiveRegion(&out)
			region := lines(30, "line ")
			r.Draw(nil, region, tt.rows)
			if want := strings.Join(region[:tt.want], "\n") + "\n"; out.String() != want {
				t.Errorf("drew %q, want the region's first %d lines, %q", out.String(), tt.want, want)
			}
			out.Reset()
			r.Draw(nil, []string{"x"}, tt.rows)
			if want := fmt.Sprintf("\r\x1b[%dA\x1b[Jx\n", tt.want); out.String() != want {
				t.Errorf("redrew %q, want the move up %d rows, %q", out.String(), tt.want, want)
			}
		})
	}
}

func TestRedrawsLeaveTheScreenAsTheLastDrawHasIt(t *testing.T) {
	t.Parallel()
	term := frametest.NewTerminal(20, 6)
	r := presenter.NewLiveRegion(term)
	r.Draw([]string{"header"}, lines(4, "plan "), 6)
	r.Draw([]string{"done 1"}, lines(5, "next "), 6)
	r.Erase()
	r.Draw(nil, []string{"? question", "plan 1"}, 6)
	r.Ask([]string{"? question "})
	if _, err := term.Write([]byte("typed\n")); err != nil {
		t.Fatal(err)
	}
	r.Released(true)
	r.Draw([]string{"answered"}, []string{"plan 1"}, 6)
	if err := term.Err(); err != nil {
		t.Fatal(err)
	}
	if got, want := term.Grid().Text(), []string{"header", "done 1", "? question typed", "answered", "plan 1"}; !slices.Equal(got, want) {
		t.Errorf("the screen reads\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
