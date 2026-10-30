package rail_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter/rail"
)

// Every state has its own mark, so with no colour the drawing still says it.
func TestEachStateDrawsItsOwnMarkWithNoColour(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item rail.Item
		want []string
	}{
		{name: "the start station", item: rail.Station{Kind: rail.Start, Name: "v1.3.2", Detail: "main @ 7c1e0b4"}, want: []string{" ◇  v1.3.2        main @ 7c1e0b4"}},
		{name: "a stage done", item: rail.Node{Name: "version", Detail: "v1.4.0 (minor bump)"}, want: []string{" ●  version       v1.4.0 (minor bump)"}},
		{name: "a stage done, with its answer", item: rail.Node{Name: "notes", Detail: "written", Tail: "accepted (-y)"}, want: []string{" ●  notes         written · accepted (-y)"}},
		{name: "a stage done, its answer alone", item: rail.Node{Name: "review", Tail: "accepted"}, want: []string{" ●  review        accepted"}},
		{name: "a stage running, its first frame", item: rail.Node{State: rail.Running, Name: "pre_tag", Detail: "running the pre_tag hook…"}, want: []string{" ⠋  pre_tag       running the pre_tag hook…"}},
		{name: "a stage running, a later frame", item: rail.Node{State: rail.Running, Name: "pre_tag", Frame: 12}, want: []string{" ⠹  pre_tag"}},
		{name: "a stage to come", item: rail.Node{State: rail.ToCome, Name: "push", Detail: "main + v1.4.0 → origin, atomic"}, want: []string{" ○  push          main + v1.4.0 → origin, atomic"}},
		{name: "a stage failed", item: rail.Node{State: rail.Failed, Name: "notes", Detail: "the AI call failed"}, want: []string{" ✗  notes         the AI call failed"}},
		{name: "a warning", item: rail.Node{State: rail.Warning, Name: "publish", Detail: "gh failed"}, want: []string{" !  publish       gh failed"}},
		{name: "the unwind", item: rail.Node{State: rail.Unwind, Name: "unwind", Detail: "reset 1 commit"}, want: []string{" ↩  unwind        reset 1 commit"}},
		{name: "the end station waiting", item: rail.Station{Kind: rail.Pending, Name: "v1.4.0"}, want: []string{" ◇  v1.4.0"}},
		{
			name: "the end station landed",
			item: rail.End{Landed: true, Name: "v1.4.0", Detail: "released", URL: "https://github.com/acme/pigeon/releases/tag/v1.4.0"},
			want: []string{" ◆  v1.4.0        released", "                  https://github.com/acme/pigeon/releases/tag/v1.4.0"},
		},
		{name: "the end station stopped", item: rail.End{Name: "v1.4.0", Detail: "dry run · nothing changed"}, want: []string{" ◇  v1.4.0        dry run · nothing changed"}},
		{
			name: "a question",
			item: rail.Question{Text: "Commit this message?", Choices: []rail.Choice{{Key: "y", Action: "commit"}, {Key: "n", Action: "abort"}}},
			want: []string{" ?  Commit this message?", " ┆  y commit   n abort"},
		},
		{name: "a dry run's preview of a question", item: rail.Question{Text: "Release with these notes?"}, want: []string{" ?  Release with these notes?"}},
		{name: "the point of no return ahead", item: rail.Rule{}, want: []string{" ┿━━ point of no return " + strings.Repeat("━", 76)}},
		{name: "the point of no return crossed", item: rail.Rule{Crossed: true}, want: []string{" ┿━━ point of no return " + strings.Repeat("━", 76)}},
		{
			name: "progress",
			item: rail.Node{State: rail.Running, Name: "notes", Progress: &rail.Progress{Label: "part 1 of 4", Done: 1, Total: 4}},
			want: []string{" ⠋  notes         part 1 of 4  ━━━━━━──────────────────"},
		},
		{
			name: "progress with no parts done",
			item: rail.Node{State: rail.Running, Name: "notes", Progress: &rail.Progress{Label: "merging 4 parts", Total: 4}},
			want: []string{" ⠋  notes         merging 4 parts  ────────────────────────"},
		},
		{
			name: "lines hanging off a failed node",
			item: rail.Node{State: rail.Failed, Name: "pre_tag", Detail: "the pre_tag hook failed", Lines: []rail.Hang{
				{Kind: rail.Output, Text: "npm ERR! missing script: build\n\nnpm ERR! see the log\n"},
				{Kind: rail.Note, Text: "part notes saved to"},
				{Kind: rail.Path, Text: "~/notes.md"},
			}},
			want: []string{
				" ✗  pre_tag       the pre_tag hook failed",
				" │                │ npm ERR! missing script: build",
				" │                │",
				" │                │ npm ERR! see the log",
				" │                part notes saved to",
				" │                ~/notes.md",
			},
		},
		{
			name: "a body, verbatim",
			item: rail.Body{Text: "  indented  twice\n\n✨ Added\n"},
			want: []string{" │      indented  twice", " │", " │    ✨ Added"},
		},
		{
			name: "next",
			item: rail.Next{Moves: []rail.Move{{Lead: "Release with the commit-subject list:", Command: "mint release -m --no-ai"}, {Lead: "Pass --no-ai to skip the AI."}}},
			want: []string{" next", "   Release with the commit-subject list:", "     mint release -m --no-ai", "   Pass --no-ai to skip the AI."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := draw(t, 100, tt.item); !slices.Equal(got, tt.want) {
				t.Errorf("drawn\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestTheRailBetweenItemsSaysHowFarTheRunHasCome(t *testing.T) {
	t.Parallel()
	done := rail.Node{Name: "record", Detail: "CHANGELOG.md"}
	toCome := rail.Node{State: rail.ToCome, Name: "push"}
	tests := []struct {
		name  string
		items []rail.Item
		want  []string
	}{
		{name: "nodes done, one after another", items: []rail.Item{done, done}, want: []string{" ●  record        CHANGELOG.md", " ●  record        CHANGELOG.md"}},
		{
			name:  "a blank under the header, the rail under the start station",
			items: []rail.Item{rail.Header{Command: "commit"}, rail.Station{Kind: rail.Start, Name: "main"}, done},
			want:  []string{" mint ─ commit", "", " ◇  main", " │", " ●  record        CHANGELOG.md"},
		},
		{
			name:  "dotted below the live node, the plan hollow",
			items: []rail.Item{rail.Node{State: rail.Running, Name: "notes"}, toCome, toCome},
			want:  []string{" ⠋  notes", " ┆", " ○  push", " ○  push"},
		},
		{
			name:  "the point of no return ahead, dotted",
			items: []rail.Item{toCome, rail.Rule{}, toCome, rail.Station{Kind: rail.Pending, Name: "v1.4.0"}},
			want:  []string{" ○  push", " ┆", " ┿━━ point of no return " + strings.Repeat("━", 76), " ○  push", " ┆", " ◇  v1.4.0"},
		},
		{
			name:  "the point of no return crossed, solid",
			items: []rail.Item{done, rail.Rule{Crossed: true}, done, rail.End{Landed: true, Name: "v1.4.0"}},
			want:  []string{" ●  record        CHANGELOG.md", " │", " ┿━━ point of no return " + strings.Repeat("━", 76), " ●  record        CHANGELOG.md", " │", " ◆  v1.4.0"},
		},
		{
			name:  "the rail around a body, and the question under it",
			items: []rail.Item{done, rail.Body{Text: "notes"}, rail.Question{Text: "Use these notes?"}, toCome},
			want:  []string{" ●  record        CHANGELOG.md", " │", " │    notes", " │", " ?  Use these notes?", " ┆", " ○  push"},
		},
		{
			name: "solid down a question under a node, dotted below it",
			items: []rail.Item{
				rail.Node{Name: "check", Detail: "2 flagged", Lines: []rail.Hang{{Kind: rail.Note, Text: "demo/ excluded · demo tapes"}}},
				rail.Question{Under: true, Text: "Exclude design/ from the notes?", About: "design documents · 1 file, 26 kB", Choices: []rail.Choice{{Key: "e", Action: "exclude"}, {Key: "k", Action: "keep"}}},
				toCome,
			},
			want: []string{
				" ●  check         2 flagged",
				" │                demo/ excluded · demo tapes",
				" │                ? Exclude design/ from the notes?",
				" │                  design documents · 1 file, 26 kB",
				" │                  e exclude   k keep",
				" ┆",
				" ○  push",
			},
		},
		{
			name:  "a blank over next",
			items: []rail.Item{rail.End{Name: "v1.3.2"}, rail.Next{Moves: []rail.Move{{Lead: "Try again."}}}},
			want:  []string{" ◇  v1.3.2", "", " next", "   Try again."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := drawIn(100, rail.NoColour, tt.items...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("drawn\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(tt.want, "\n"))
			}
		})
	}
}

func TestLevelOfReadsTermenvsProfile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		profile termenv.Profile
		want    rail.Level
	}{
		{profile: termenv.TrueColor, want: rail.TrueColour},
		{profile: termenv.ANSI256, want: rail.Colours256},
		{profile: termenv.ANSI, want: rail.Colours16},
		{profile: termenv.Ascii, want: rail.NoColour},
	}
	for _, tt := range tests {
		if got := rail.LevelOf(tt.profile); got != tt.want {
			t.Errorf("LevelOf(%v) = %v, want %v", tt.profile, got, tt.want)
		}
	}
}
