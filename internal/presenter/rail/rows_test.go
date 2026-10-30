package rail_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter/frametest"
	"mint/internal/presenter/rail"
)

// heldToRows holds each of lines to one terminal row at width: it reads as cells
// with no control character and no escape but SGR, and is no wider than width.
func heldToRows(t *testing.T, width int, lines []string) {
	t.Helper()
	for _, ln := range lines {
		g, err := frametest.Parse(ln)
		if err != nil {
			t.Errorf("at %d the line %q isn't cells alone: %v", width, ln, err)
			continue
		}
		if len(g) != 1 || len(g[0]) > width {
			t.Errorf("at %d the line %q takes %d rows, the first %d cells wide, want one no wider than %d", width, ln, len(g), len(g[0]), width)
		}
	}
}

// drawAll is items drawn at width in truecolor, every line of them.
func drawAll(width int, items ...rail.Item) []string {
	return drawIn(width, rail.TrueColour, items...)
}

func TestTextIsDrawnSoNothingInItCanMoveTheCursorOrChangeThePen(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, text, want string
	}{
		{name: "a tab, to the next stop from the text's start", text: "a\tb", want: "a       b"},
		{name: "a tab on a stop, to the one after", text: "abcdefgh\tx", want: "abcdefgh        x"},
		{name: "a carriage return mid-line", text: "50%\r100%", want: "50%␍100%"},
		{name: "a carriage return closing the text", text: "done\r", want: "done␍"},
		{name: "a colour, closed", text: "\x1b[1;31mred\x1b[0m!", want: "red!"},
		{name: "a colour never closed", text: "\x1b[31mred", want: "red"},
		{name: "a reset alone", text: "a\x1b[mb", want: "ab"},
		{name: "the cursor moved up", text: "\x1b[2Aup", want: "␛[2Aup"},
		{name: "a lone escape", text: "a\x1bb", want: "a␛b"},
		{name: "a C1 control", text: "\u009b31mx", want: "␛[31mx"},
		{name: "a bell and a delete", text: "\abell\x7f", want: "␇bell␡"},
		{name: "a NUL", text: "a\x00b", want: "a␀b"},
		{name: "a byte that isn't UTF-8", text: "a\xffb", want: "a�b"},
		{name: "a zero-width space, as it is", text: "zero\u200bwidth", want: "zero\u200bwidth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := draw(t, 100, rail.Body{Text: tt.text})
			if want := []string{" │    " + tt.want}; !slices.Equal(got, want) {
				t.Errorf("the body %q draws %q, want %q", tt.text, got, want)
			}
			heldToRows(t, 100, drawAll(100, rail.Body{Text: tt.text}))
		})
	}
}

func TestACRLFBreaksALineAsANewlineDoes(t *testing.T) {
	t.Parallel()
	got := draw(t, 100, rail.Node{State: rail.Failed, Name: "push", Lines: []rail.Hang{{Kind: rail.Output, Text: "a\r\nb\r\n"}}})
	want := []string{" ✗  push", " │                │ a", " │                │ b"}
	if !slices.Equal(got, want) {
		t.Errorf("the output draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// hostile is text with everything in it a terminal would act on.
const hostile = "tab\there\r\nCR\rthen \x1b[31mred \x1b[0mreset \x1b[2Kerase \u009b1A up \a bell\x7f\x00 \xff\u200b\nnext line"

func TestEveryFieldIsDrawnAsOneRowEach(t *testing.T) {
	t.Parallel()
	items := []rail.Item{
		rail.Header{Command: "release " + hostile, Where: "~/" + hostile, Version: hostile},
		rail.Station{Kind: rail.Start, Name: hostile, Detail: hostile},
		rail.Node{Name: hostile, Detail: hostile, Tail: hostile, Time: hostile, Lines: []rail.Hang{
			{Kind: rail.Note, Text: hostile}, {Kind: rail.Output, Text: hostile}, {Kind: rail.Path, Text: hostile},
		}},
		rail.Node{State: rail.Running, Name: "notes", Detail: hostile, Time: "1s"},
		rail.Node{State: rail.Running, Name: "notes", Progress: &rail.Progress{Label: hostile, Done: 1, Total: 2}},
		rail.Question{Text: hostile, Choices: []rail.Choice{{Key: "\x1b[2J", Action: hostile}}, Typing: true},
		rail.Question{Under: true, Text: hostile, About: hostile, Choices: []rail.Choice{{Key: "\x1b[2J", Action: hostile}}, Typing: true},
		rail.Body{Text: hostile},
		rail.Body{Text: hostile, Markdown: true},
		rail.End{Name: hostile, Detail: hostile, Time: hostile},
		rail.Next{Moves: []rail.Move{{Lead: hostile, Command: "a\x1b[2Kb\tc\nd"}}},
		rail.Version{Value: hostile},
	}
	for _, width := range []int{60, 70, 80, 100} {
		heldToRows(t, width, drawAll(width, items...))
	}
}

func TestADetailsLinesAfterItsFirstHangUnderTheNode(t *testing.T) {
	t.Parallel()
	got := draw(t, 100, rail.Node{State: rail.Failed, Name: "push", Detail: "git push failed:\nremote: rejected\n", Time: "1.2s", Lines: []rail.Hang{{Kind: rail.Note, Text: "nothing pushed"}}})
	want := []string{
		" ✗  push          git push failed:" + strings.Repeat(" ", 62) + "1.2s",
		" │                remote: rejected",
		" │                nothing pushed",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the node draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	heldToRows(t, 100, drawAll(100, rail.Node{State: rail.Running, Name: "push", Detail: "a\nb\nc"}))
}

func TestALineBreakInAOneRowFieldDrawsAsASpace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item rail.Item
		want []string
	}{
		{name: "a tail", item: rail.Node{Name: "notes", Detail: "written", Tail: "accepted\n(-y)"}, want: []string{" ●  notes         written · accepted (-y)"}},
		{name: "a station's detail", item: rail.Station{Kind: rail.Start, Name: "v1", Detail: "main\r\n@ 7c1e0b4"}, want: []string{" ◇  v1            main @ 7c1e0b4"}},
		{name: "an end's detail", item: rail.End{Name: "v1", Detail: "back\nhome"}, want: []string{" ◇  v1            back home"}},
		{name: "a question", item: rail.Question{Text: "Use these\nnotes?"}, want: []string{" ?  Use these notes?"}},
		{name: "a lead", item: rail.Next{Moves: []rail.Move{{Lead: "Try\nagain."}}}, want: []string{" next", "   Try again."}},
		{name: "the header's command", item: rail.Header{Command: "release\nregenerate"}, want: []string{" mint ─ release regenerate"}},
		{name: "a time", item: rail.Node{Name: "push", Time: "1m\n2s"}, want: []string{" ●  push" + strings.Repeat(" ", 87) + "1m 2s"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := draw(t, 100, tt.item); !slices.Equal(got, tt.want) {
				t.Errorf("drawn\n%q\nwant\n%q", got, tt.want)
			}
			heldToRows(t, 100, drawAll(100, tt.item))
		})
	}
}

func TestOnlyWhatFitsARowMayBeLive(t *testing.T) {
	t.Parallel()
	live := reflect.TypeFor[rail.Live]()
	tests := []struct {
		item rail.Item
		want bool
	}{
		{item: rail.Station{}, want: true},
		{item: rail.Node{}, want: true},
		{item: rail.Question{}, want: true},
		{item: rail.Rule{}, want: true},
		{item: rail.Body{}, want: true},
		{item: rail.Header{}, want: false},
		{item: rail.End{}, want: false},
		{item: rail.Next{}, want: false},
		{item: rail.Version{}, want: false},
	}
	for _, tt := range tests {
		if got := reflect.TypeOf(tt.item).Implements(live); got != tt.want {
			t.Errorf("%T may be live: %t, want %t", tt.item, got, tt.want)
		}
	}
}

func TestARegionIsDrawnOnlyWhereTheTerminalIsAsWideAsTheLayout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		term, width int
		fits        bool
	}{
		{term: 0, width: 80, fits: true},
		{term: 40, width: 60, fits: false},
		{term: 59, width: 60, fits: false},
		{term: 60, width: 60, fits: true},
		{term: 160, width: 100, fits: true},
	}
	for _, tt := range tests {
		l := rail.NewLayout(tt.term, rail.TrueColour)
		if l.Width() != tt.width || l.RegionFits() != tt.fits {
			t.Errorf("a terminal %d wide lays out %d wide, region fitting %t, want %d and %t", tt.term, l.Width(), l.RegionFits(), tt.width, tt.fits)
		}
		heldToRows(t, l.Width(), l.Region(running()))
	}
}

// retrying is the bar's text as a part is re-cut.
const retrying = "part 2 was too big for the model, retrying it as 2 smaller parts"

func TestABarsLabelIsCutBeforeItsBar(t *testing.T) {
	t.Parallel()
	node := rail.Node{State: rail.Running, Name: "notes", Time: "38s", Progress: &rail.Progress{Label: retrying, Done: 1, Total: 2}}
	for _, width := range []int{60, 80, 100} {
		lines := drawIn(width, rail.NoColour, node)
		heldToRows(t, width, drawAll(width, node))
		detail := lines[len(lines)-1]
		bar := strings.Count(detail, "━") + strings.Count(detail, "─")
		label, _, _ := strings.Cut(detail[strings.Index(detail, "part 2"):], "  ━")
		if bar < 12 || !strings.HasSuffix(label, "…") || !strings.HasPrefix(retrying, strings.TrimSuffix(label, "…")) {
			t.Errorf("at %d the bar draws %q: a bar of %d cells after the label %q, want 12 at least, the label cut", width, detail, bar, label)
		}
	}
	short := draw(t, 100, rail.Node{State: rail.Running, Name: "notes", Progress: &rail.Progress{Label: "merging 4 parts", Total: 4}})
	if want := " ⠋  notes         merging 4 parts  " + strings.Repeat("─", 24); !slices.Equal(short, []string{want}) {
		t.Errorf("a short label draws %q, want the whole bar %q", short, want)
	}
}

func TestInitDrawsRowsWithNoRail(t *testing.T) {
	t.Parallel()
	run := rail.Run{Settled: []rail.Item{
		rail.Header{Command: "init", Where: "~/Code/pigeon"},
		rail.Node{Name: ".mint.toml", Detail: "created"},
		rail.Node{State: rail.LeftAlone, Name: "release", Detail: "already there · --force overwrites it"},
		rail.Node{Name: ".github/workflows/release.yml", Detail: "created"},
	}}
	tests := []struct {
		width int
		want  []string
	}{
		{width: 100, want: []string{
			" mint ─ init" + strings.Repeat(" ", 75) + "~/Code/pigeon",
			"",
			" ●  .mint.toml    created",
			" ○  release       already there · --force overwrites it",
			" ●  .github/workflows/release.yml",
			"                  created",
		}},
		{width: 60, want: []string{
			" mint ─ init" + strings.Repeat(" ", 35) + "~/Code/pigeon",
			"",
			" ●  .mint.toml",
			"    created",
			" ○  release",
			"    already there · --force overwrites it",
			" ●  .github/workflows/release.yml",
			"    created",
		}},
	}
	for _, tt := range tests {
		got := rail.NewLayout(tt.width, rail.NoColour).Draw(run)
		if !slices.Equal(got, tt.want) {
			t.Errorf("at %d init draws\n%s\nwant\n%s", tt.width, strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
		}
		heldToRows(t, tt.width, rail.NewLayout(tt.width, rail.TrueColour).Draw(run))
	}
	colour := drawAll(100, rail.Node{State: rail.LeftAlone, Name: "release", Detail: "already there"})
	if want := " " + dm("○") + "  " + dm("release") + pad(7) + dm("already there"); !slices.Equal(colour, []string{want}) {
		t.Errorf("a file left alone draws %q, want the whole row dim %q", colour, want)
	}
}

func TestTheVersionLineIsTheWordmarkAndTheVersion(t *testing.T) {
	t.Parallel()
	for level, a := range accents {
		got := drawIn(100, level, rail.Version{Value: "0.1.2"})
		if want := []string{" " + acc(a, "mint") + " v0.1.2"}; !slices.Equal(got, want) {
			t.Errorf("at level %d the version draws %q, want %q", level, got, want)
		}
	}
	if got := draw(t, 100, rail.Version{Value: "0.1.2"}); !slices.Equal(got, []string{" mint v0.1.2"}) {
		t.Errorf("with no colour the version draws %q", got)
	}
	heldToRows(t, 60, drawAll(60, rail.Version{Value: strings.Repeat("9", 80)}))
}

func TestADryRunsRailIntoItsEndIsDimAfterTheStagesItSkipped(t *testing.T) {
	t.Parallel()
	run := rail.Run{Settled: []rail.Item{
		rail.Node{Name: "notes", Detail: "written"},
		rail.Node{State: rail.ToCome, Name: "record"},
		rail.Rule{},
		rail.Node{State: rail.ToCome, Name: "push"},
		rail.End{Name: "v1.4.0", Detail: "dry run · nothing changed", Time: "9s"},
	}}
	for level, a := range accents {
		got := rail.NewLayout(100, level).Draw(run)
		want := []string{
			" " + acc(a, "●") + "  " + bld("notes") + pad(9) + "written",
			" " + dm("┆"),
			" " + dm("○") + "  " + dm("record"),
			" " + dm("┆"),
			" " + dm("┿━━ point of no return "+rule(76)),
			" " + dm("○") + "  " + dm("push"),
			" " + dm("┆"),
			" " + acc(a, "◇") + "  " + bld("v1.4.0") + pad(8) + "dry run · nothing changed" + pad(55) + dm("9s"),
		}
		if !slices.Equal(got, want) {
			t.Errorf("at level %d the dry run draws\n%q\nwant\n%q", level, got, want)
		}
		heldToRows(t, 100, got)
	}
}

func TestANegativeSpinnerFrameCountsBack(t *testing.T) {
	t.Parallel()
	if got := draw(t, 100, rail.Node{State: rail.Running, Name: "notes", Frame: -1}); !slices.Equal(got, []string{" ⠏  notes"}) {
		t.Errorf("frame -1 draws %q, want the last frame", got)
	}
}

func TestALongCommandAndALongTimeAreCut(t *testing.T) {
	t.Parallel()
	header := draw(t, 60, rail.Header{Command: "release " + strings.Repeat("regenerate ", 10), Version: "0.1.2"})
	if len(header) != 1 || cellsOf(t, header[0]) != 60 || !strings.HasSuffix(header[0], "…") {
		t.Errorf("a long command draws %q, want it cut to the width", header)
	}
	node := draw(t, 100, rail.Node{Name: "push", Detail: "main → origin", Time: strings.Repeat("9", 40)})
	if len(node) != 1 || cellsOf(t, node[0]) != 100 || !strings.HasSuffix(node[0], " "+strings.Repeat("9", 11)+"…") {
		t.Errorf("a long time draws %q, want it cut to 12 cells", node)
	}
	heldToRows(t, 60, drawAll(60, rail.Header{Command: strings.Repeat("x", 200)}))
}
