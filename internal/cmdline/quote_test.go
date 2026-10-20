package cmdline_test

import (
	"slices"
	"testing"

	"mint/internal/cmdline"
)

func TestQuote_SplitGivesBackTheWordWhole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		word string
	}{
		{name: "a space", word: "/Users/me/Code/My App/v1.2.4.md"},
		{name: "a single quote", word: "/Users/me/Code/Lee's App/v1.2.4.md"},
		{name: "only single quotes", word: "''"},
		{name: "a dollar", word: "/tmp/$HOME/notes.md"},
		{name: "a backtick", word: "/tmp/a`id`b.md"},
		{name: "a star", word: "/tmp/*.md"},
		{name: "a semicolon", word: "/tmp/a;rm -rf b.md"},
		{name: "an ampersand", word: "/tmp/a&b.md"},
		{name: "a pipe", word: "/tmp/a|b.md"},
		{name: "a tilde", word: "~/notes.md"},
		{name: "a double quote and backslash", word: `/tmp/a"b\c.md`},
		{name: "a tab and newline", word: "/tmp/a\tb\nc.md"},
		{name: "empty", word: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			quoted := cmdline.Quote(tt.word)
			got, err := cmdline.Split("mint release --notes-file " + quoted)
			if err != nil {
				t.Fatalf("Split of Quote(%q) = %q: %v", tt.word, quoted, err)
			}
			if want := []string{"mint", "release", "--notes-file", tt.word}; !slices.Equal(got, want) {
				t.Errorf("Split of Quote(%q) = %q, want %q", tt.word, got, want)
			}
		})
	}
}

func TestQuote_WritesQuotingOnlyWhereAByteNeedsIt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		word string
		want string
	}{
		{
			name: "every safe byte is left as it is",
			word: "/Users/me/Library/Caches/mint/chunk-notes/-Users-me-Code-mint/v1.2.4_@%+=:,.md",
			want: "/Users/me/Library/Caches/mint/chunk-notes/-Users-me-Code-mint/v1.2.4_@%+=:,.md",
		},
		{name: "a space wraps the word in single quotes", word: "/a/My App.md", want: "'/a/My App.md'"},
		{name: "a single quote is written as '\\''", word: "/a/Lee's.md", want: `'/a/Lee'\''s.md'`},
		{name: "empty is an empty quoted word", word: "", want: "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := cmdline.Quote(tt.word); got != tt.want {
				t.Errorf("Quote(%q) = %q, want %q", tt.word, got, tt.want)
			}
		})
	}
}
