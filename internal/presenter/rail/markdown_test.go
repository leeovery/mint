package rail_test

import (
	"slices"
	"testing"

	"mint/internal/presenter/rail"
)

func TestANotesBodyDrawsAsItsMarkdownReads(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, line, want string
	}{
		{name: "a heading loses its markers", line: "## ✨ Added", want: "✨ Added"},
		{name: "the deepest heading", line: "###### Six", want: "Six"},
		{name: "past the deepest, as written", line: "####### Seven", want: "####### Seven"},
		{name: "a heading with no space, as written", line: "#hashtag", want: "#hashtag"},
		{name: "a heading of markers alone", line: "##", want: ""},
		{name: "a heading three in", line: "   # In", want: "In"},
		{name: "four in, as written", line: "    # Code", want: "    # Code"},
		{name: "a list item's marker a bullet", line: "- Retries", want: "• Retries"},
		{name: "a starred item", line: "* Retries", want: "• Retries"},
		{name: "a plussed item", line: "+ Retries", want: "• Retries"},
		{name: "a nested item keeps its indent", line: "  - Nested", want: "  • Nested"},
		{name: "a dash with no space, as written", line: "-1 retries", want: "-1 retries"},
		{name: "a rule, as written", line: "---", want: "---"},
		{name: "inline code loses its ticks", line: "- `pigeon replay <id>` resends", want: "• pigeon replay <id> resends"},
		{name: "bold loses its stars", line: "**Breaking:** the API moved", want: "Breaking: the API moved"},
		{name: "a lone tick, as written", line: "it's a ` tick", want: "it's a ` tick"},
		{name: "empty code, as written", line: "an `` empty", want: "an `` empty"},
		{name: "stars with no partner, as written", line: "**not closed", want: "**not closed"},
		{name: "a link, as written", line: "See [the docs](https://x.test/docs).", want: "See [the docs](https://x.test/docs)."},
		{name: "plain text, as written", line: "Webhooks retry on their own now.", want: "Webhooks retry on their own now."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			want := []string{" │    " + tt.want}
			if tt.want == "" {
				want = []string{" │"}
			}
			if got := draw(t, 100, rail.Body{Text: tt.line, Markdown: true}); !slices.Equal(got, want) {
				t.Errorf("drawn %q, want %q", got, want)
			}
		})
	}
}

func TestABodyNotMarkdownDrawsAsWritten(t *testing.T) {
	t.Parallel()
	text := "## feat: `retry` **now**\n- item"
	want := []string{" │    ## feat: `retry` **now**", " │    - item"}
	if got := draw(t, 100, rail.Body{Text: text}); !slices.Equal(got, want) {
		t.Errorf("drawn %q, want %q", got, want)
	}
}

func TestAMarkdownBodyWrapsAsRendered(t *testing.T) {
	t.Parallel()
	got := draw(t, 60, rail.Body{Text: "- `pigeon replay <id>` resends any delivery from the last thirty days.", Markdown: true})
	want := []string{
		" │    • pigeon replay <id> resends any delivery from the ",
		" │    last thirty days.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("drawn\n%q\nwant\n%q", got, want)
	}
}

func TestAMarkdownBodysStylesAtEachLevel(t *testing.T) {
	t.Parallel()
	text := "## Use `mint`\n- **Breaking:** `x` moved"
	for level, a := range accents {
		got := drawIn(100, level, rail.Body{Text: text, Markdown: true})
		want := []string{
			" " + acc(a, "│") + "    " + bld("Use ") + accentBold(a, "mint"),
			" " + acc(a, "│") + "    • " + bld("Breaking:") + " " + acc(a, "x") + " moved",
		}
		if !slices.Equal(got, want) {
			t.Errorf("at level %d drawn\n%q\nwant\n%q", level, got, want)
		}
	}
}
