package rail_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter/frametest"
	"mint/internal/presenter/rail"
)

func TestNewLayoutClampsTheTerminalsWidth(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		term, want int
	}{
		{name: "none given", term: 0, want: 80},
		{name: "unreadable", term: -1, want: 80},
		{name: "below the least", term: 40, want: 60},
		{name: "the least", term: 60, want: 60},
		{name: "between", term: 87, want: 87},
		{name: "the most", term: 100, want: 100},
		{name: "past the most", term: 220, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := rail.NewLayout(tt.term, rail.NoColour).Width(); got != tt.want {
				t.Errorf("NewLayout(%d).Width() = %d, want %d", tt.term, got, tt.want)
			}
		})
	}
}

func TestATimeEndsOnTheLayoutsLastColumn(t *testing.T) {
	t.Parallel()
	items := []rail.Item{
		rail.Node{Name: "push", Detail: "main + v1.4.0 → origin, atomic", Time: "1.9s"},
		rail.Node{State: rail.Running, Name: "notes", Detail: "writing…", Time: "38s"},
		rail.Node{State: rail.Failed, Name: "notes", Detail: "it failed", Time: "41s"},
		rail.End{Landed: true, Name: "v1.4.0", Detail: "released", Time: "1m 14s"},
	}
	for _, width := range []int{70, 80, 100} {
		for _, it := range items {
			first := draw(t, width, it)[0]
			if got := cellsOf(t, first); got != width || !strings.HasSuffix(first, timeOf(it)) {
				t.Errorf("at %d the line %q is %d cells, want %d ending in its time", width, first, got, width)
			}
		}
	}
}

func timeOf(it rail.Item) string {
	switch v := it.(type) {
	case rail.Node:
		return v.Time
	case rail.End:
		return v.Time
	}
	return ""
}

func TestBelowSeventyColumnsTimesDropAndTheDetailGoesUnderItsName(t *testing.T) {
	t.Parallel()
	node := rail.Node{Name: "pre_tag", Detail: "npm ci && npm run build", Time: "4.1s"}
	tests := []struct {
		width int
		want  []string
	}{
		{width: 70, want: []string{" ●  pre_tag       npm ci && npm run build" + strings.Repeat(" ", 25) + "4.1s"}},
		{width: 69, want: []string{" ●  pre_tag", " │  npm ci && npm run build"}},
	}
	for _, tt := range tests {
		if got := draw(t, tt.width, node); !slices.Equal(got, tt.want) {
			t.Errorf("at %d the node draws\n%s\nwant\n%s", tt.width, strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
		}
	}
}

func TestANameTooLongForItsColumnPutsTheDetailUnderIt(t *testing.T) {
	t.Parallel()
	got := draw(t, 100, rail.Node{State: rail.Warning, Name: "docs/specs/archive/**", Detail: "flagged, not asked", Time: "1s"})
	want := []string{
		" !  docs/specs/archive/**" + strings.Repeat(" ", 73) + "1s",
		" │                flagged, not asked",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the node draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// long is text no line holds: words, a run of spaces, emoji two cells wide, and a
// path longer than any line.
const long = "Deliveries that fail now retry  on an exponential schedule ✨ for up to an hour " +
	"before they're marked dead 🐛, so a short outage no longer loses events: " +
	"/very/long/path/that/no/line/of/the/layout/holds/whole/at/any/width/it/draws/at/all/v1.4.0.md end"

func TestHangingTextWrapsWithTheRailOnEveryLineAndNoCharacterChanged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item rail.Item
		// prefix is what each line of the text starts with at the width given.
		prefix func(width int) string
	}{
		{
			name:   "a note",
			item:   rail.Node{Name: "pre_tag", Lines: []rail.Hang{{Kind: rail.Note, Text: long}}},
			prefix: func(width int) string { return " │" + strings.Repeat(" ", detailCol(width)-2) },
		},
		{
			name:   "a path",
			item:   rail.Node{Name: "notes", State: rail.Failed, Lines: []rail.Hang{{Kind: rail.Path, Text: long}}},
			prefix: func(width int) string { return " │" + strings.Repeat(" ", detailCol(width)-2) },
		},
		{
			name:   "output, behind its gutter",
			item:   rail.Node{Name: "notes", State: rail.Failed, Lines: []rail.Hang{{Kind: rail.Output, Text: long}}},
			prefix: func(width int) string { return " │" + strings.Repeat(" ", detailCol(width)-2) + "│ " },
		},
		{
			name:   "a body",
			item:   rail.Body{Text: long},
			prefix: func(int) string { return " │    " },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for _, width := range []int{60, 80, 100} {
				lines := draw(t, width, tt.item)
				if _, ok := tt.item.(rail.Node); ok {
					lines = lines[1:]
				}
				heldToWrap(t, width, lines, tt.prefix(width), long)
			}
		})
	}
}

func TestASettledDetailWrapsInsideTheDetailColumn(t *testing.T) {
	t.Parallel()
	for _, width := range []int{70, 80, 100} {
		lines := draw(t, width, rail.Node{Name: "record", Detail: long, Time: "2.4s"})
		head, rest := lines[0], lines[1:]
		const name = " ●  record        "
		text := strings.TrimRight(strings.TrimSuffix(head, "2.4s"), " ")
		if !strings.HasPrefix(head, name) || cellsOf(t, head) != width {
			t.Fatalf("at %d the first line %q isn't the name, the detail and the time on the last column", width, head)
		}
		// The spaces a line breaks after are kept, but on the first line they can't be
		// told from the padding before the time.
		first := strings.TrimPrefix(text, name)
		if !strings.HasPrefix(long, first) {
			t.Fatalf("at %d the first line holds %q, want the detail's start", width, first)
		}
		heldToWrap(t, width, rest, " │"+strings.Repeat(" ", 16), strings.TrimLeft(long[len(first):], " "))
	}
}

// heldToWrap holds lines to text wrapped at width: each fits, each starts with
// prefix, and with the prefixes taken off they join to text again.
func heldToWrap(t *testing.T, width int, lines []string, prefix, text string) {
	t.Helper()
	var joined strings.Builder
	for _, ln := range lines {
		if got := cellsOf(t, ln); got > width {
			t.Errorf("at %d the line %q is %d cells", width, ln, got)
		}
		if !strings.HasPrefix(ln, prefix) {
			t.Fatalf("at %d the line %q doesn't start %q", width, ln, prefix)
		}
		joined.WriteString(strings.TrimPrefix(ln, prefix))
	}
	if joined.String() != text {
		t.Errorf("at %d the lines join to\n%q\nwant\n%q", width, joined.String(), text)
	}
	if len(lines) < 2 {
		t.Errorf("at %d the text took %d line, want it wrapped", width, len(lines))
	}
}

func TestALongWordBreaksAfterASlashWhereItCan(t *testing.T) {
	t.Parallel()
	got := draw(t, 80, rail.Node{Name: "notes", Lines: []rail.Hang{
		{Kind: rail.Path, Text: "~/Library/Caches/mint/chunk-notes/Users-lee-Code-pigeon/v1.4.0.md"},
	}})
	want := []string{
		" ●  notes",
		" │                ~/Library/Caches/mint/chunk-notes/Users-lee-Code-pigeon/",
		" │                v1.4.0.md",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the path draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestALiveNodesLineIsCutNotWrapped(t *testing.T) {
	t.Parallel()
	node := rail.Node{State: rail.Running, Name: "notes", Detail: long, Time: "38s"}
	for _, width := range []int{70, 80, 100} {
		lines := draw(t, width, node)
		if len(lines) != 1 {
			t.Fatalf("at %d the live node draws %d lines, want 1", width, len(lines))
		}
		ln := lines[0]
		text := strings.TrimRight(strings.TrimSuffix(ln, "38s"), " ")
		if cellsOf(t, ln) != width || !strings.HasSuffix(text, "…") {
			t.Errorf("at %d the line %q isn't cut with an ellipsis, its time on the last column", width, ln)
		}
	}
	narrow := draw(t, 60, node)
	if len(narrow) != 2 || cellsOf(t, narrow[1]) != 60 || !strings.HasSuffix(narrow[1], "…") {
		t.Errorf("at 60 the live node draws %q, want its name, then its detail cut to the width", narrow)
	}
}

func TestAQuestionsKeysGoOnALineOfTheirOwnAndWrapWhole(t *testing.T) {
	t.Parallel()
	q := rail.Question{Text: "Release with these notes?", Choices: []rail.Choice{
		{Key: "y", Action: "release with these notes"}, {Key: "n", Action: "abort the release"},
		{Key: "e", Action: "edit them"}, {Key: "r", Action: "regenerate them"},
	}}
	got := draw(t, 60, q)
	want := []string{
		" ?  Release with these notes?",
		" ┆  y release with these notes   n abort the release",
		" ┆  e edit them   r regenerate them",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the question draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestATypedLineIsReadAfterTheKeysOnTheLastLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		q    rail.Question
		want []string
	}{
		{
			name: "a question with keys",
			q:    rail.Question{Text: "Source?", Typing: true, Choices: []rail.Choice{{Key: "fresh", Action: "re-diff + AI"}, {Key: "tag", Action: "tag annotation body"}}},
			want: []string{" ?  Source?", " ┆  fresh re-diff + AI   tag tag annotation body ›  "},
		},
		{
			name: "a question with none",
			q:    rail.Question{Text: "Add one-time context:", Typing: true},
			want: []string{" ?  Add one-time context: ›  "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := draw(t, 100, tt.q); !slices.Equal(got, tt.want) {
				t.Errorf("the question draws\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestAQuestionUnderANodeWrapsIndentedUnderItsMark(t *testing.T) {
	t.Parallel()
	q := rail.Question{
		Under: true, Text: "Exclude docs/specs/archive/** from the release notes?",
		About:   "design documents and the plans written before them · 6 files, 48 kB",
		Choices: []rail.Choice{{Key: "e", Action: "exclude"}, {Key: "k", Action: "keep"}},
	}
	got := draw(t, 70, q)
	want := []string{
		" │                ? Exclude docs/specs/archive/** from the release ",
		" │                  notes?",
		" │                  design documents and the plans written before ",
		" │                  them · 6 files, 48 kB",
		" │                  e exclude   k keep",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the question draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNextsCommandsAreNeverCutOrWrapped(t *testing.T) {
	t.Parallel()
	command := "mint release -m --notes-file " + strings.Repeat("very-long-path/", 8) + "v1.4.0.md"
	lead := "Tidy the saved part notes, then release with them, as the AI's own words may need a look first:"
	got := draw(t, 60, rail.Next{Moves: []rail.Move{{Lead: lead, Command: command}, {Lead: "Or run it again."}}})
	want := []string{
		" next",
		"   Tidy the saved part notes, then release with them, as ",
		"   the AI's own words may need a look first:",
		"     " + command,
		"   Or run it again.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("next draws\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTheRegionsHeightIsItsLineCount(t *testing.T) {
	t.Parallel()
	for _, width := range []int{60, 64, 70, 80, 100} {
		for name, r := range map[string]rail.Run{"running": running(), "review": review(), "long": longRegion()} {
			l := rail.NewLayout(width, rail.TrueColour)
			region := l.Region(r)
			heldToRows(t, width, region)
			if all, settled := l.Draw(r), l.Settled(r, 0); len(all) != len(settled)+len(region) {
				t.Errorf("%s at %d: the drawing is %d lines, want the settled %d and the region's %d", name, width, len(all), len(settled), len(region))
			}
		}
	}
}

func longRegion() rail.Run {
	return rail.Run{
		Settled: []rail.Item{releaseHeader(), releaseStart()},
		Region: []rail.Live{
			rail.Node{Name: "check", Detail: long, Time: "6.2s", Lines: []rail.Hang{{Kind: rail.Note, Text: long}}},
			rail.Body{Text: long + "\n\n" + notesBody},
			rail.Question{Under: true, Text: "Exclude docs/specs/** from the notes? " + long, About: long, Choices: []rail.Choice{{Key: "e", Action: "exclude"}, {Key: "k", Action: "keep"}}},
			rail.Question{Text: "Release with these notes? " + long, Choices: []rail.Choice{{Key: "y", Action: "release"}, {Key: "n", Action: "abort"}}},
			rail.Node{State: rail.ToCome, Name: "push", Detail: long},
			rail.Station{Kind: rail.Pending, Name: "v1.4.0"},
		},
	}
}

func TestSettledDrawsTheItemsFromTheIndexGiven(t *testing.T) {
	t.Parallel()
	r := failed()
	l := rail.NewLayout(100, rail.TrueColour)
	all := l.Settled(r, 0)
	for from := range r.Settled {
		head := len(l.Settled(rail.Run{Settled: r.Settled[:from]}, 0))
		if got := l.Settled(r, from); !slices.Equal(got, all[head:]) {
			t.Errorf("Settled from %d draws\n%s\nwant the lines after the first %d\n%s", from, strings.Join(got, "\n"), head, strings.Join(all[head:], "\n"))
		}
	}
}

func TestTheHeadersRightSideGivesWay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header rail.Header
		want   string
	}{
		{
			name:   "whole",
			header: rail.Header{Command: "release", Where: "~/Code/pigeon", Version: "0.1.2"},
			want:   " mint ─ release" + strings.Repeat(" ", 23) + "~/Code/pigeon · v0.1.2",
		},
		{
			name:   "where cut from the left",
			header: rail.Header{Command: "release regenerate", Where: "~/Code/clients/acme/pigeon-webhooks", Version: "0.1.2"},
			want:   " mint ─ release regenerate  …s/acme/pigeon-webhooks · v0.1.2",
		},
		{
			name:   "where dropped, the version kept",
			header: rail.Header{Command: "release regenerate --all --target both", Where: "~/Code/clients/acme/pigeon-webhooks", Version: "0.1.2"},
			want:   " mint ─ release regenerate --all --target both" + strings.Repeat(" ", 8) + "v0.1.2",
		},
		{
			name:   "a dev build, with no version",
			header: rail.Header{Command: "commit", Where: "~/Code/pigeon"},
			want:   " mint ─ commit" + strings.Repeat(" ", 33) + "~/Code/pigeon",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := draw(t, 60, tt.header); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("the header draws\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

// draw is it alone, drawn at width with no colour.
func draw(t *testing.T, width int, it rail.Item) []string {
	t.Helper()
	return drawIn(width, rail.NoColour, it)
}

// drawIn is items drawn at width and level, settled in a run whose end station
// waits in the region undrawn: a run with a station, so the rail shows.
func drawIn(width int, level rail.Level, items ...rail.Item) []string {
	r := rail.Run{Settled: items, Region: []rail.Live{rail.Station{Kind: rail.Pending}}}
	return rail.NewLayout(width, level).Settled(r, 0)
}

// cellsOf is how many cells line takes on a terminal.
func cellsOf(t *testing.T, line string) int {
	t.Helper()
	g, err := frametest.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(g) == 0 {
		return 0
	}
	return len(g[0])
}

func detailCol(width int) int {
	if width < 70 {
		return 4
	}
	return 18
}
