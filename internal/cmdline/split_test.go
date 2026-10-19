package cmdline_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"mint/internal/cmdline"
)

func TestSplit_QuotingRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		want    []string
	}{
		{
			name:    "joined, single-quoted, escaped and empty parts",
			command: `tool --system-prompt="Be terse" '$HOME' a\ b "" ''`,
			want:    []string{"tool", "--system-prompt=Be terse", "$HOME", "a b", "", ""},
		},
		{
			name:    "single quotes keep everything literally",
			command: `tool 'a "b" \c $d'`,
			want:    []string{"tool", `a "b" \c $d`},
		},
		{
			name:    "double-quote backslash escapes only the four specials",
			command: `tool "a\"b" "c\\d" "e\$f" "g\x" "h` + "\\`" + `i"`,
			want:    []string{"tool", `a"b`, `c\d`, `e$f`, `g\x`, "h`i"},
		},
		{
			name:    "backslash-newline outside quotes is a line continuation",
			command: "claude -p \\\n--model opus",
			want:    []string{"claude", "-p", "--model", "opus"},
		},
		{
			name:    "backslash-newline inside a word joins it",
			command: "tool ab\\\ncd",
			want:    []string{"tool", "abcd"},
		},
		{
			name:    "backslash-newline inside double quotes is removed",
			command: "tool \"ab\\\ncd\"",
			want:    []string{"tool", "abcd"},
		},
		{
			name:    "backslash-newline inside single quotes is literal",
			command: "tool 'ab\\\ncd'",
			want:    []string{"tool", "ab\\\ncd"},
		},
		{
			name:    "shell metacharacters are ordinary word characters",
			command: "tool $HOME *.go ~/x a|b c;d e&f <in >out `x`",
			want:    []string{"tool", "$HOME", "*.go", "~/x", "a|b", "c;d", "e&f", "<in", ">out", "`x`"},
		},
		{
			name:    "outside quotes a backslash makes a quote literal",
			command: `tool \"a\' \\`,
			want:    []string{"tool", `"a'`, `\`},
		},
		{
			name:    "quoted whitespace does not separate words",
			command: "tool \"a\tb\" 'c  d'",
			want:    []string{"tool", "a\tb", "c  d"},
		},
		{
			name:    "the lean default command",
			command: `claude -p --model sonnet --safe-mode --tools "" --strict-mcp-config --no-session-persistence --system-prompt "Follow the instructions you are given and output only what they ask for."`,
			want: []string{
				"claude", "-p", "--model", "sonnet", "--safe-mode", "--tools", "",
				"--strict-mcp-config", "--no-session-persistence", "--system-prompt",
				"Follow the instructions you are given and output only what they ask for.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := cmdline.Split(tt.command)
			if err != nil {
				t.Fatalf("Split(%q) returned unexpected error: %v", tt.command, err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Split(%q) = %q, want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestSplit_UnquotedCommandMatchesWhitespaceSplit(t *testing.T) {
	t.Parallel()

	commands := []string{
		"claude -p --model opus",
		"  claude \t -p\t\t--model   opus  ",
		"claude\n-p\r\n--model\vopus\f",
		"llm -m\u0085gpt-4o",
		"",
		" \t\n ",
	}

	for _, command := range commands {
		got, err := cmdline.Split(command)
		if err != nil {
			t.Fatalf("Split(%q) returned unexpected error: %v", command, err)
		}
		want := strings.Fields(command)
		if !slices.Equal(got, want) {
			t.Errorf("Split(%q) = %q, want the whitespace split %q", command, got, want)
		}
	}
}

func TestSplit_RejectsUnsplittableCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		want    error
	}{
		{name: "unterminated double quote", command: `tool "abc`, want: cmdline.ErrUnterminatedQuote},
		{name: "unterminated single quote", command: `tool 'abc`, want: cmdline.ErrUnterminatedQuote},
		{name: "escaped closing double quote leaves it open", command: `tool "abc\"`, want: cmdline.ErrUnterminatedQuote},
		{name: "trailing lone backslash", command: `tool abc\`, want: cmdline.ErrTrailingBackslash},
		{name: "lone backslash", command: `\`, want: cmdline.ErrTrailingBackslash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := cmdline.Split(tt.command)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Split(%q) error = %v, want %v", tt.command, err, tt.want)
			}
			if got != nil {
				t.Errorf("Split(%q) words = %q, want nil on error", tt.command, got)
			}
		})
	}
}
