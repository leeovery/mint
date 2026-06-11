package initgen_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/initgen"
)

// commonKeyDefaults lists the active (uncommented) common keys the template must
// show at their out-of-the-box defaults, each as the exact TOML line the generator
// emits. These prove the template is a usable, defaults-documenting starting point.
var commonKeyDefaults = []string{
	"ai_command = 'claude -p'",
	"max_diff_lines = 50000",
	"tag_prefix = 'v'",
	"commit_prefix = '🌿'",
	"changelog = true",
	"publish = true",
	"on_notes_failure = 'abort'",
}

func TestMintTOML_IncludesCommonKeysAtDefaults(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	for _, line := range commonKeyDefaults {
		if !strings.Contains(tmpl, line) {
			t.Errorf("template missing active common key line %q", line)
		}
	}
}

// optionalKeys lists every optional key that must be present-but-COMMENTED, each
// with a one-line explanation. The key is the TOML key/table name as it appears
// after the comment marker.
var optionalKeys = []string{
	"diff_exclude",
	"release_branch",
	"version_file",
	"version_pattern",
	"provider",
	"context",
	"[release.hooks]",
	"preflight",
	"pre_tag",
	"post_release",
}

func TestMintTOML_OptionalKeysPresentButCommentedWithExplanation(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()
	lines := strings.Split(tmpl, "\n")

	for _, key := range optionalKeys {
		commented := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "#") {
				continue
			}
			// Strip the comment marker(s) and check the key appears as a
			// config line, not buried in prose.
			body := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if isConfigLineForKey(body, key) {
				commented = true
				break
			}
		}
		if !commented {
			t.Errorf("optional key %q must appear as a commented config line", key)
		}
	}
}

// isConfigLineForKey reports whether body (a comment line with its marker already
// stripped) is the commented config line introducing key: either a `key = ...`
// assignment or the `[release.hooks]` table header.
func isConfigLineForKey(body, key string) bool {
	if strings.HasPrefix(key, "[") {
		return body == key
	}
	return strings.HasPrefix(body, key+" =") || strings.HasPrefix(body, key+"=")
}

func TestMintTOML_OptionalKeysEachHaveAComment(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()
	lines := strings.Split(tmpl, "\n")

	// Each non-table optional key's commented line must carry an explanation —
	// either a trailing `# ...` on the same line or it sits within a commented
	// block. We assert the explanatory trailing comment form the generator uses.
	keysNeedingInlineComment := []string{
		"diff_exclude",
		"release_branch",
		"version_file",
		"version_pattern",
		"provider",
		"context",
		"preflight",
		"pre_tag",
		"post_release",
	}

	for _, key := range keysNeedingInlineComment {
		if !hasExplainedCommentedKey(lines, key) {
			t.Errorf("optional key %q must have a one-line explanation on its commented line", key)
		}
	}
}

// hasExplainedCommentedKey reports whether key appears on a commented config line
// that also carries an explanatory trailing comment (a `# ...` after the value).
func hasExplainedCommentedKey(lines []string, key string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		body := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if !isConfigLineForKey(body, key) {
			continue
		}
		// After the leading marker is stripped, an explanation is a second `#`
		// segment carrying prose, i.e. the body still contains a `#` introducing
		// a trailing comment after the assignment.
		if idx := strings.Index(body, "#"); idx > 0 {
			explanation := strings.TrimSpace(body[idx+1:])
			if explanation != "" {
				return true
			}
		}
	}
	return false
}

func TestMintTOML_HooksOnlyUnderReleaseHooks(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()
	lines := strings.Split(tmpl, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		body := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if body == "[hooks]" {
			t.Errorf("template must not contain a top-level [hooks] table; hooks nest under [release.hooks]")
		}
	}

	if !strings.Contains(tmpl, "[release.hooks]") {
		t.Error("template must contain the [release.hooks] table")
	}
}

func TestMintTOML_PreTagShowsBothStringAndArrayForms(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	// pre_tag must demonstrate BOTH a single command string and an array of
	// commands, so the operator sees both shapes the schema accepts. Exactly one
	// form is an uncommentable `pre_tag = ...` config line (uncommenting both
	// would duplicate the key); the other is documented in prose. We therefore
	// assert: a string-valued config line for pre_tag exists, AND an array example
	// (`['npm ci', 'npm run build']`) is shown somewhere referencing pre_tag.
	hasConfigStringForm := false
	hasArrayExample := false
	for _, line := range strings.Split(tmpl, "\n") {
		body := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if isConfigLineForKey(body, "pre_tag") {
			value := valueAfterEquals(body)
			if strings.HasPrefix(value, "'") || strings.HasPrefix(value, "\"") {
				hasConfigStringForm = true
			}
		}
		if strings.Contains(line, "pre_tag") && strings.Contains(line, "['npm ci', 'npm run build']") {
			hasArrayExample = true
		}
	}

	if !hasConfigStringForm {
		t.Error("pre_tag must show a single command-string form as an uncommentable config line")
	}
	if !hasArrayExample {
		t.Error("pre_tag must document an array-of-commands form example")
	}
}

// valueAfterEquals returns the value portion of a `key = value` line (whatever
// follows the first `=`), trimmed of surrounding whitespace.
func valueAfterEquals(line string) string {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(line[idx+1:])
}

func TestMintTOML_MentionsPromptOverrideInCommentOnly(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	// The prompt-override file is ONLY mentioned in a comment. Every line that
	// references the prompt override must be a comment line — never an active
	// config assignment.
	found := false
	for _, line := range strings.Split(tmpl, "\n") {
		if !strings.Contains(line, ".mint/notes-prompt.md") && !strings.Contains(line, "prompt") {
			continue
		}
		if strings.Contains(line, "prompt") {
			found = true
		}
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			t.Errorf("prompt override must only be mentioned in a comment, got active line: %q", line)
		}
	}
	if !found {
		t.Error("template must mention the prompt override in a comment")
	}
}

// TestMintTOML_UncommentedLoadsCleanly is the VALIDITY GUARANTEE: programmatically
// uncomment every commented CONFIG line in the template, write the result to a
// temp .mint.toml, and feed it through the REAL config.Load. Every scaffolded key
// (active or optional) must then load with no unknown-key, bad-type, or enum error.
func TestMintTOML_UncommentedLoadsCleanly(t *testing.T) {
	t.Parallel()

	uncommented := uncommentTemplate(t, initgen.MintTOML())

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".mint.toml"), []byte(uncommented), 0o644); err != nil {
		t.Fatalf("writing uncommented .mint.toml: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("uncommented template failed config.Load: %v\n---\n%s", err, uncommented)
	}

	// Sanity: the active defaults survived the round-trip, proving Load actually
	// parsed the uncommented document rather than falling back to defaults on a
	// read error.
	if cfg.AICommand != "claude -p" {
		t.Errorf("AICommand = %q, want %q", cfg.AICommand, "claude -p")
	}
	if cfg.Release.OnNotesFailure != "abort" {
		t.Errorf("OnNotesFailure = %q, want %q", cfg.Release.OnNotesFailure, "abort")
	}
}

// uncommentTemplate strips a single leading comment marker from every commented
// line that is itself a config line (a `key = value` assignment or a `[table]`
// header once the marker is removed), leaving pure prose comments commented. This
// mirrors what a user does by hand when enabling a setting, so the result is a
// fully-active config the validity test feeds through config.Load.
func uncommentTemplate(t *testing.T, tmpl string) string {
	t.Helper()

	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(tmpl))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			body := strings.TrimPrefix(trimmed, "#")
			body = strings.TrimPrefix(body, " ")
			if looksLikeConfigLine(body) {
				b.WriteString(body)
				b.WriteByte('\n')
				continue
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning template: %v", err)
	}
	return b.String()
}

// looksLikeConfigLine reports whether body (a comment line with one marker already
// stripped) is a TOML config line: a `[table]` header or a `key = value`
// assignment. Pure prose comments fail this test and stay commented.
func looksLikeConfigLine(body string) bool {
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
	// A TOML key is a single bare token (letters, digits, underscores). A prose
	// comment containing an `=` (e.g. "default = auto") would have spaces in its
	// "key", so reject anything non-key-like.
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
