package config_test

import (
	"slices"
	"strings"
	"testing"

	"mint/internal/config"
)

// readmeSection returns the README text from the line opening with heading up to the
// next heading of the same or a higher level.
func readmeSection(t *testing.T, readme, heading string) string {
	t.Helper()

	lines := strings.Split(readme, "\n")
	start := slices.Index(lines, heading)
	if start < 0 {
		t.Fatalf("README has no %q heading", heading)
	}
	level := headingLevel(heading)
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if l := headingLevel(lines[i]); l > 0 && l <= level {
			end = i
			break
		}
	}
	return strings.Join(lines[start+1:end], "\n")
}

// headingLevel returns the markdown ATX heading level of line, or 0 when it is not a
// heading.
func headingLevel(line string) int {
	n := len(line) - len(strings.TrimLeft(line, "#"))
	if n == 0 || !strings.HasPrefix(line[n:], " ") {
		return 0
	}
	return n
}

func TestREADME_SharedAICommandRowShowsShippedDefault(t *testing.T) {
	t.Parallel()

	table := readmeSection(t, readREADME(t, readmePath(t)), "### Shared engine keys")
	want := "| `ai_command` | `" + config.DefaultAICommand + "` |"
	if !strings.Contains(table, want) {
		t.Errorf("README shared engine keys table has no row opening %q", want)
	}
}

func TestREADME_NoLineCallsPlainClaudeTheShippedDefault(t *testing.T) {
	t.Parallel()

	for i, line := range strings.Split(readREADME(t, readmePath(t)), "\n") {
		if !strings.Contains(strings.ToLower(line), "default") {
			continue
		}
		for _, plain := range []string{"`claude -p --model sonnet`", "'claude -p --model sonnet'"} {
			if strings.Contains(line, plain) {
				t.Errorf("README line %d still presents %s as the default: %q", i+1, plain, line)
			}
		}
	}
}

func TestREADME_AITransportDocumentsWhatTheLeanDefaultDrops(t *testing.T) {
	t.Parallel()

	section := readmeSection(t, readREADME(t, readmePath(t)), "## The AI Transport")
	for _, want := range []string{
		config.DefaultAICommand,
		"CLAUDE.md",
		"system prompt",
		"`[release]`",
		"`[commit]`",
		"`context`",
		"`prompt`",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("README \"The AI Transport\" section must carry %q", want)
		}
	}
}
