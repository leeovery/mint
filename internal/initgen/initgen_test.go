package initgen_test

import (
	"strings"
	"testing"

	"mint/internal/initgen"
)

// TestMintTOML_BodyHasNoActiveOrCommentedKeys is the MINIMAL-SHAPE pin: the body
// carries NO config key — neither an active `key = value` / `[table]` line nor a
// commented `# key = value` / `# [table]` line. Only the header comment block (a run
// of pure-prose `#` lines, which may contain a URL or an `=` inside that prose)
// survives the strip. A header line is exempt from the no-key check via a strict
// TOML-bare-key predicate, so a header URL or prose `=` is not mis-flagged.
func TestMintTOML_BodyHasNoActiveOrCommentedKeys(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	for _, line := range strings.Split(tmpl, "\n") {
		if carriesConfigKey(line) {
			t.Errorf("minimal template body must carry no active or commented config key, got: %q", line)
		}
	}
}

// TestMintTOML_HeaderCarriesBothPointers pins the cold-arrival recovery net: the
// header must point to BOTH the GitHub docs (a github.com substring) for the human
// config reference AND the literal `mint setup` for AI-assisted setup. Both pointers
// are load-bearing — an agent or human opening the minimal file directly still finds
// where the config reference lives.
func TestMintTOML_HeaderCarriesBothPointers(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	if !strings.Contains(tmpl, "github.com") {
		t.Errorf("header must carry a GitHub-docs pointer (a github.com substring), got:\n%s", tmpl)
	}
	if !strings.Contains(tmpl, "mint setup") {
		t.Errorf("header must carry the literal `mint setup` pointer, got:\n%s", tmpl)
	}
}

// carriesConfigKey reports whether line is a TOML config key line — either an active
// or a commented `key = value` assignment or `[table]` header. A pure-prose comment
// (including one containing a URL or a prose `=`) is NOT a config key line: the
// strict bare-key predicate requires the text before `=` to be a single TOML bare
// key (letters, digits, underscore, hyphen), which prose never is.
func carriesConfigKey(line string) bool {
	body := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
	if body == "" {
		return false
	}
	if strings.HasPrefix(body, "[") && strings.HasSuffix(body, "]") {
		return true
	}
	idx := strings.Index(body, "=")
	if idx <= 0 {
		return false
	}
	key := strings.TrimSpace(body[:idx])
	if key == "" {
		return false
	}
	for _, r := range key {
		if !isBareKeyRune(r) {
			return false
		}
	}
	return true
}

// isBareKeyRune reports whether r is a valid character in a TOML bare key
// (letters, digits, underscore, or hyphen).
func isBareKeyRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_' || r == '-':
		return true
	default:
		return false
	}
}
