package config_test

import (
	"testing"

	"mint/internal/config"
)

// The TOML bodies below spell each ai_command so that its DECODED string is the value
// under test: literal strings ('…') carry backslashes and double quotes verbatim, and a
// basic string ("\\\n") decodes to a backslash followed by a newline.

func TestLoad_MalformedAICommand_RejectedNamingLevelAndCause(t *testing.T) {
	t.Parallel()

	levels := []struct {
		key    string
		prefix string
	}{
		{key: "ai_command", prefix: ""},
		{key: "release.ai_command", prefix: "[release]\n"},
		{key: "commit.ai_command", prefix: "[commit]\n"},
	}
	causes := []struct {
		name  string
		value string
		cause string
	}{
		{name: "unterminated double quote", value: `'claude --system-prompt "Be terse'`, cause: "has an unterminated quote"},
		{name: "unterminated single quote", value: `"claude --system-prompt 'Be terse"`, cause: "has an unterminated quote"},
		{name: "trailing lone backslash", value: `'claude -p \'`, cause: "ends in a lone backslash"},
		{name: "empty first word", value: `"'' --flag"`, cause: "names no program"},
		{name: "lone empty quoted word", value: `'""'`, cause: "names no program"},
		{name: "backslash-newline yields no words", value: `"\\` + "\\n" + `"`, cause: "names no program"},
	}

	for _, level := range levels {
		for _, c := range causes {
			t.Run(level.key+"/"+c.name, func(t *testing.T) {
				t.Parallel()

				dir := t.TempDir()
				writeConfig(t, dir, level.prefix+"ai_command = "+c.value+"\n")

				_, err := config.Load(dir)
				if err == nil {
					t.Fatal("Load returned nil error for a malformed ai_command, want non-nil")
				}
				want := "invalid .mint.toml: " + level.key + " " + c.cause
				if err.Error() != want {
					t.Errorf("error = %q, want %q", err.Error(), want)
				}
			})
		}
	}
}

func TestLoad_BackslashNewlineFixture_DecodesToBackslashNewline(t *testing.T) {
	t.Parallel()

	// Guards the fixture above: the TOML basic string "\\\n" must decode to exactly a
	// backslash then a newline, or the "no words" case would be testing something else.
	// A continuation inside a word is well-formed, so the value loads and can be read back.
	dir := t.TempDir()
	writeConfig(t, dir, "[commit]\nai_command = \"a\\\\\\nb\"\n")
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if got := *cfg.Commit.AICommand; got != "a\\\nb" {
		t.Fatalf("decoded commit.ai_command = %q, want %q", got, "a\\\nb")
	}
}

func TestLoad_WellFormedAICommandWithLaterEmptyArgument_Loads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "ai_command = 'claude --tools \"\"'\n[release]\nai_command = \"claude --tools ''\"\n[commit]\nai_command = 'claude --tools \"\"'\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if got, want := cfg.AICommandFor(config.VerbCommit), `claude --tools ""`; got != want {
		t.Errorf("AICommandFor(VerbCommit) = %q, want %q", got, want)
	}
}

func TestLoad_BlankAICommandAtEveryLevel_LoadsAndFallsToDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "ai_command = ''\n[release]\nai_command = '   '\n[commit]\nai_command = \"\\t\\n \"\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	for _, verb := range []config.Verb{config.VerbRelease, config.VerbCommit} {
		if got := cfg.AICommandFor(verb); got != config.DefaultAICommand {
			t.Errorf("AICommandFor(%v) = %q, want the shipped default %q", verb, got, config.DefaultAICommand)
		}
	}
}

func TestLoad_BlankCommitAICommandUnderShared_LoadsAndFallsToShared(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeConfig(t, dir, "ai_command = 'llm -m gpt-4o'\n[commit]\nai_command = ''\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if got, want := cfg.AICommandFor(config.VerbCommit), "llm -m gpt-4o"; got != want {
		t.Errorf("AICommandFor(VerbCommit) = %q, want the shared %q", got, want)
	}
}
