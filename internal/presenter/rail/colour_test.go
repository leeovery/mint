package rail_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/presenter/rail"
)

// The accent at each level: #5FD7AF in 24 bits, xterm-256 colour 79, the theme's
// cyan.
var accents = map[rail.Level]string{
	rail.TrueColour: "38;2;95;215;175",
	rail.Colours256: "38;5;79",
	rail.Colours16:  "36",
}

// The escapes each element is drawn with, as agntc's are: bold 1, dim 2, both shut
// by 22; red 31, yellow 33 and the accent each shut by 39; never a full reset.
func bld(text string) string     { return "\x1b[1m" + text + "\x1b[22m" }
func dm(text string) string      { return "\x1b[2m" + text + "\x1b[22m" }
func rd(text string) string      { return "\x1b[31m" + text + "\x1b[39m" }
func ylw(text string) string     { return "\x1b[33m" + text + "\x1b[39m" }
func pad(n int) string           { return strings.Repeat(" ", n) }
func rule(n int) string          { return strings.Repeat("━", n) }
func redBold(text string) string { return "\x1b[1;31m" + text + "\x1b[22;39m" }

func TestEachElementIsDrawnInItsOwnEscapesAtEachLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item rail.Item
		// want is the lines drawn, given the accent's code.
		want func(a string) []string
	}{
		{
			name: "the header: mint bold in the accent, the rule and the right side dim",
			item: rail.Header{Command: "release", Where: "~/Code/pigeon", Version: "0.1.2"},
			want: func(a string) []string {
				return []string{" " + accentBold(a, "mint") + " " + dm("─") + " release" + pad(63) + dm("~/Code/pigeon · v0.1.2")}
			},
		},
		{
			name: "the start station: its mark in the accent, the name bold, the detail dim",
			item: rail.Station{Kind: rail.Start, Name: "v1.3.2", Detail: "main @ 7c1e0b4"},
			want: func(a string) []string {
				return []string{" " + acc(a, "◇") + "  " + bld("v1.3.2") + pad(8) + dm("main @ 7c1e0b4")}
			},
		},
		{
			name: "a stage done: its mark in the accent, the name bold, its answer and time dim",
			item: rail.Node{Name: "notes", Detail: "written", Tail: "accepted", Time: "52.3s"},
			want: func(a string) []string {
				return []string{" " + acc(a, "●") + "  " + bld("notes") + pad(9) + "written" + dm(" · accepted") + pad(59) + dm("52.3s")}
			},
		},
		{
			name: "a stage running: the spinner in the accent, its text dim",
			item: rail.Node{State: rail.Running, Name: "pre_tag", Detail: "running the pre_tag hook…", Frame: 1},
			want: func(a string) []string {
				return []string{" " + acc(a, "⠙") + "  " + bld("pre_tag") + pad(7) + dm("running the pre_tag hook…")}
			},
		},
		{
			name: "progress: the bar's done part in the accent, the rest dim",
			item: rail.Node{State: rail.Running, Name: "notes", Progress: &rail.Progress{Label: "part 3 of 4", Done: 3, Total: 4}},
			want: func(a string) []string {
				return []string{" " + acc(a, "⠋") + "  " + bld("notes") + pad(9) + dm("part 3 of 4  ") + acc(a, strings.Repeat("━", 18)) + dm(strings.Repeat("─", 6))}
			},
		},
		{
			name: "a stage to come: the whole row dim",
			item: rail.Node{State: rail.ToCome, Name: "push", Detail: "main → origin"},
			want: func(string) []string {
				return []string{" " + dm("○") + "  " + dm("push") + pad(10) + dm("main → origin")}
			},
		},
		{
			name: "a stage failed: its mark, name, detail and rail red, its output behind a dim gutter, what mint did and the path in the normal colour",
			item: rail.Node{State: rail.Failed, Name: "notes", Detail: "it failed", Lines: []rail.Hang{{Kind: rail.Output, Text: "usage limit"}, {Kind: rail.Did, Text: "saved to"}, {Kind: rail.Path, Text: "~/n.md"}}},
			want: func(string) []string {
				return []string{
					" " + rd("✗") + "  " + redBold("notes") + pad(9) + rd("it failed"),
					" " + rd("│") + pad(16) + dm("│ ") + "usage limit",
					" " + rd("│") + pad(16) + "saved to",
					" " + rd("│") + pad(16) + "~/n.md",
				}
			},
		},
		{
			name: "a failed stage's detail: red on every line it runs to, wrapped or broken",
			item: rail.Node{State: rail.Failed, Name: "pre_tag", Detail: strings.Repeat("failed ", 12) + "running sh:\ncontext canceled"},
			want: func(string) []string {
				return []string{
					" " + rd("✗") + "  " + redBold("pre_tag") + pad(7) + rd(strings.Repeat("failed ", 11)),
					" " + rd("│") + pad(16) + rd("failed running sh:"),
					" " + rd("│") + pad(16) + rd("context canceled"),
				}
			},
		},
		{
			name: "a warning: its mark yellow",
			item: rail.Node{State: rail.Warning, Name: "v1.2.0", Detail: "skipped"},
			want: func(string) []string {
				return []string{" " + ylw("!") + "  " + bld("v1.2.0") + pad(8) + "skipped"}
			},
		},
		{
			name: "the unwind: its mark in the accent, the rail under it solid, what it did in the normal colour",
			item: rail.Node{State: rail.Unwind, Name: "unwind", Detail: "reset", Lines: []rail.Hang{{Kind: rail.Did, Text: "kept the answers"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "↩") + "  " + bld("unwind") + pad(8) + "reset",
					" " + acc(a, "│") + pad(16) + "kept the answers",
				}
			},
		},
		{
			name: "a stage done: what mint did under it in the normal colour, metadata dim",
			item: rail.Node{Name: "preflight", Detail: "tree clean", Lines: []rail.Hang{{Kind: rail.Did, Text: "committed as chore: x"}, {Kind: rail.Note, Text: "skipping preflight hook"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "●") + "  " + bld("preflight") + pad(5) + "tree clean",
					" " + acc(a, "│") + pad(16) + "committed as chore: x",
					" " + acc(a, "│") + pad(16) + dm("skipping preflight hook"),
				}
			},
		},
		{
			name: "a question: the question bold, each key bold in the accent on a line of their own",
			item: rail.Question{Text: "Use these notes?", Choices: []rail.Choice{{Key: "y", Action: "accept"}, {Key: "n", Action: "abort"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "?") + "  " + bld("Use these notes?"),
					" " + dm("┆") + "  " + accentBold(a, "y") + " accept   " + accentBold(a, "n") + " abort",
				}
			},
		},
		{
			name: "a default word: its mark dim",
			item: rail.Question{Text: "Source?", Choices: []rail.Choice{{Key: "tag", Action: "tag body", Default: true}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "?") + "  " + bld("Source?"),
					" " + dm("┆") + "  " + accentBold(a, "tag") + " tag body" + dm(" (default)"),
				}
			},
		},
		{
			name: "a question under a node: its branch's ├ in the accent and ┄ dim, its ? in the accent, the question bold, what it's about dim",
			item: rail.Question{Under: true, Text: "Exclude docs/?", About: "design documents", Choices: []rail.Choice{{Key: "e", Action: "exclude"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "├") + dm(strings.Repeat("┄", 15)) + " " + acc(a, "? ") + bld("Exclude docs/?"),
					" " + acc(a, "│") + pad(18) + dm("design documents"),
					" " + acc(a, "│") + pad(18) + accentBold(a, "e") + " exclude",
				}
			},
		},
		{
			name: "an answer acted on: its ✓ in the accent, its text in the normal colour, what it's about dim",
			item: rail.Node{Name: "check", Lines: []rail.Hang{{Kind: rail.Acted, Text: "docs/ excluded", About: "design documents"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "●") + "  " + bld("check"),
					" " + acc(a, "│") + pad(16) + acc(a, "✓ ") + "docs/ excluded" + dm(" · design documents"),
				}
			},
		},
		{
			name: "an answer kept: its – and all of it dim",
			item: rail.Node{Name: "check", Lines: []rail.Hang{{Kind: rail.Kept, Text: "docs/ kept", About: "design documents"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "●") + "  " + bld("check"),
					" " + acc(a, "│") + pad(16) + dm("– docs/ kept · design documents"),
				}
			},
		},
		{
			name: "lines hanging off a node settled before them: the rail solid, each line as the node hangs it",
			item: rail.Hanging{Lines: []rail.Hang{{Kind: rail.Kept, Text: "docs/ kept", About: "design documents"}, {Kind: rail.Did, Text: "written to .mint.toml"}}},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "│") + pad(16) + dm("– docs/ kept · design documents"),
					" " + acc(a, "│") + pad(16) + "written to .mint.toml",
				}
			},
		},
		{
			name: "the point of no return ahead: all of it dim",
			item: rail.Rule{},
			want: func(string) []string { return []string{" " + dm("┿━━ point of no return "+rule(76))} },
		},
		{
			name: "the point of no return crossed: its ┿ in the accent",
			item: rail.Rule{Crossed: true},
			want: func(a string) []string {
				return []string{" " + acc(a, "┿") + dm("━━ point of no return "+rule(76))}
			},
		},
		{
			name: "the end station landed: its mark in the accent, its time dim, its URL in the accent",
			item: rail.End{Landed: true, Name: "v1.4.0", Detail: "released", Time: "1m 14s", URL: "https://x.test/v1.4.0"},
			want: func(a string) []string {
				return []string{
					" " + acc(a, "◆") + "  " + bld("v1.4.0") + pad(8) + "released" + pad(68) + dm("1m 14s"),
					pad(18) + acc(a, "https://x.test/v1.4.0"),
				}
			},
		},
		{
			name: "the end station waiting: all of it dim",
			item: rail.Station{Kind: rail.Pending, Name: "v1.4.0"},
			want: func(string) []string { return []string{" " + dm("◇") + "  " + dm("v1.4.0")} },
		},
		{
			name: "a body behind the solid rail",
			item: rail.Body{Text: "✨ Added"},
			want: func(a string) []string { return []string{" " + acc(a, "│") + "    ✨ Added"} },
		},
		{
			name: "next: its heading bold in the accent, each command in the accent",
			item: rail.Next{Moves: []rail.Move{{Lead: "Skip the AI:", Command: "mint release --no-ai"}}},
			want: func(a string) []string {
				return []string{" " + accentBold(a, "next"), "   Skip the AI:", "     " + acc(a, "mint release --no-ai")}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			for level, a := range accents {
				got := drawIn(100, level, tt.item)
				if want := tt.want(a); !slices.Equal(got, want) {
					t.Errorf("at level %d drawn\n%q\nwant\n%q", level, got, want)
				}
			}
			for _, ln := range drawIn(100, rail.NoColour, tt.item) {
				if strings.Contains(ln, "\x1b") {
					t.Errorf("with no colour drawn %q, want no escape", ln)
				}
			}
		})
	}
}

func TestTheDottedRailAndTheSolidRailInEscapes(t *testing.T) {
	t.Parallel()
	items := []rail.Item{
		rail.Node{Name: "record"},
		rail.Rule{Crossed: true},
		rail.Node{State: rail.Running, Name: "notes"},
		rail.Node{State: rail.ToCome, Name: "push"},
		rail.Station{Kind: rail.Pending, Name: "v1"},
	}
	for level, a := range accents {
		got := drawIn(100, level, items...)
		want := []string{
			" " + acc(a, "●") + "  " + bld("record"),
			" " + acc(a, "│"),
			" " + acc(a, "┿") + dm("━━ point of no return "+rule(76)),
			" " + acc(a, "⠋") + "  " + bld("notes"),
			" " + dm("┆"),
			" " + dm("○") + "  " + dm("push"),
			" " + dm("┆"),
			" " + dm("◇") + "  " + dm("v1"),
		}
		if !slices.Equal(got, want) {
			t.Errorf("at level %d drawn\n%q\nwant\n%q", level, got, want)
		}
	}
}

func acc(a, text string) string        { return "\x1b[" + a + "m" + text + "\x1b[39m" }
func accentBold(a, text string) string { return "\x1b[1;" + a + "m" + text + "\x1b[22;39m" }
