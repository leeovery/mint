package initgen_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"mint/internal/config"
	"mint/internal/initgen"
)

// commonKeyDefaults lists the active (uncommented) common keys the template must
// show at their out-of-the-box defaults, each as the exact TOML line the generator
// emits. These prove the template is a usable, defaults-documenting starting point.
var commonKeyDefaults = []string{
	"ai_command = 'claude -p --model sonnet'",
	"timeout = 60",
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

// TestMintTOML_AICommandIsPinnedSonnetDefault proves the scaffolded top-level
// ai_command shows the pinned default model command. This is the user-facing surface
// of the model pin (spec: "bump the scaffolded ai_command to claude -p --model
// sonnet").
func TestMintTOML_AICommandIsPinnedSonnetDefault(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	if !strings.Contains(tmpl, "ai_command = 'claude -p --model sonnet'") {
		t.Errorf("template missing active pinned ai_command line %q", "ai_command = 'claude -p --model sonnet'")
	}
}

// TestMintTOML_AICommandValueEqualsConfigConstant is the DRIFT GUARD tying the
// scaffolded ai_command value to config.DefaultAICommand: the spec mandates the
// pinned default VALUE be sourced from the canonical config constant, not re-typed.
// The template is kept a pure static string for readability, so this build-failing
// test pin is the constant-link guarantee — a drift between the template literal and
// the schema constant fails here.
func TestMintTOML_AICommandValueEqualsConfigConstant(t *testing.T) {
	t.Parallel()

	value := activeTopLevelValue(t, initgen.MintTOML(), "ai_command")
	if value != config.DefaultAICommand {
		t.Errorf("scaffolded ai_command value = %q, want config.DefaultAICommand %q", value, config.DefaultAICommand)
	}
}

// TestMintTOML_ScaffoldsActiveSharedTimeout proves the template adds the new active
// (uncommented) shared timeout key at its 60s default as integer seconds.
func TestMintTOML_ScaffoldsActiveSharedTimeout(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	if !strings.Contains(tmpl, "timeout = 60") {
		t.Errorf("template missing active shared timeout line %q", "timeout = 60")
	}
}

// TestMintTOML_TimeoutValueEqualsConfigConstant is the DRIFT GUARD tying the
// scaffolded timeout value to config.DefaultTimeout (in seconds), the same
// constant-link guarantee applied to ai_command. The template is pure static, so a
// drift between the scaffolded 60 and the schema floor fails here.
func TestMintTOML_TimeoutValueEqualsConfigConstant(t *testing.T) {
	t.Parallel()

	value := activeTopLevelValue(t, initgen.MintTOML(), "timeout")
	want := int(config.DefaultTimeout / time.Second)
	if value != strconv.Itoa(want) {
		t.Errorf("scaffolded timeout value = %q, want int(config.DefaultTimeout / time.Second) = %d", value, want)
	}
}

// TestMintTOML_PerVerbAICommandAndTimeoutOverridesShownCommented proves the per-verb
// override pattern is discoverable: under BOTH [release] and [commit] a commented
// ai_command and timeout config line appears, each with a trailing explanation (per
// the template's optional-key convention). uncommentTemplate strips one marker, so
// these must be single-# config lines the optional-key pins recognise.
func TestMintTOML_PerVerbAICommandAndTimeoutOverridesShownCommented(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	for _, table := range []string{"release", "commit"} {
		section := tableSection(tmpl, table)
		if section == "" {
			t.Fatalf("could not locate the [%s] table section in the template", table)
		}
		lines := strings.Split(section, "\n")
		for _, key := range []string{"ai_command", "timeout"} {
			if !hasExplainedCommentedKey(lines, key) {
				t.Errorf("[%s] must carry a commented per-verb %q override with a trailing explanation", table, key)
			}
		}
	}
}

// TestMintTOML_CommentsNameNoModelOrStrongerModelSteer scans every comment line's
// EXPLANATION (the prose after the config value) and asserts no model alias is named
// and no "stronger model" steer appears. The pinned default VALUE on a config line
// (`--model sonnet`) is allowed — only model names inside a # explanation are
// forbidden (spec: "Config comments stay model-agnostic").
func TestMintTOML_CommentsNameNoModelOrStrongerModelSteer(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	forbiddenTokens := []string{"sonnet", "opus", "haiku"}
	forbiddenPhrases := []string{"stronger model", "better model", "smarter model", "more capable model", "powerful model"}

	for _, line := range strings.Split(tmpl, "\n") {
		explanation := strings.ToLower(commentExplanation(line))
		if explanation == "" {
			continue
		}
		for _, token := range forbiddenTokens {
			if strings.Contains(explanation, token) {
				t.Errorf("comment explanation names a model %q (must stay model-agnostic): %q", token, line)
			}
		}
		for _, phrase := range forbiddenPhrases {
			if strings.Contains(explanation, phrase) {
				t.Errorf("comment explanation carries a stronger-model steer %q: %q", phrase, line)
			}
		}
	}
}

// TestMintTOML_TimeoutHintFramedAroundLatency asserts the timeout explanations are
// framed around COMMAND LATENCY (the ai_command running slowly), not a model, and
// carry no concrete model-tied timeout number — the only timeout value in the
// scaffold is the generic 60s default and the illustrative per-verb example.
func TestMintTOML_TimeoutHintFramedAroundLatency(t *testing.T) {
	t.Parallel()

	tmpl := initgen.MintTOML()

	var timeoutExplanations []string
	for _, line := range strings.Split(tmpl, "\n") {
		body := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if !isConfigLineForKey(body, "timeout") {
			continue
		}
		explanation := commentExplanation(line)
		if explanation == "" {
			t.Errorf("timeout config line missing its explanation: %q", line)
			continue
		}
		timeoutExplanations = append(timeoutExplanations, explanation)
	}

	if len(timeoutExplanations) == 0 {
		t.Fatal("no timeout config-line explanations found to assert latency framing")
	}

	for _, explanation := range timeoutExplanations {
		lower := strings.ToLower(explanation)
		if !strings.Contains(lower, "slow") && !strings.Contains(lower, "latency") {
			t.Errorf("timeout explanation must be framed around command latency (slow/latency), got: %q", explanation)
		}
	}
}

// activeTopLevelValue extracts the value of an ACTIVE (uncommented) top-level
// `key = value` line from the template, stripping surrounding TOML quotes so the
// returned value is the raw command/number. It fails the test if no active line for
// key exists.
func activeTopLevelValue(t *testing.T, tmpl, key string) string {
	t.Helper()

	for _, line := range strings.Split(tmpl, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !isConfigLineForKey(trimmed, key) {
			continue
		}
		value := valueAfterEquals(trimmed)
		// Drop a trailing `# ...` explanation if present, then strip TOML quotes.
		if idx := strings.Index(value, "#"); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		return strings.Trim(value, "'\"")
	}
	t.Fatalf("no active top-level %q line found in template", key)
	return ""
}

// tableSection returns the slice of the template from the active `[table]` header up
// to (but excluding) the next top-level `[...]` header. For the [commit] table, whose
// HEADER is itself commented (`# [commit]`), it matches the commented header too so
// the per-verb override lines beneath it are still scanned. Returns "" if the header
// is not found.
func tableSection(tmpl, table string) string {
	lines := strings.Split(tmpl, "\n")
	start := -1
	for i, line := range lines {
		body := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line), "#"))
		if body == "["+table+"]" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		body := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(lines[i]), "#"))
		if strings.HasPrefix(body, "[") && strings.HasSuffix(body, "]") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// commentExplanation returns the trailing prose explanation on a line — the text
// after the LAST `#` that introduces a comment following a config assignment. For a
// commented config line like `# key = value  # prose`, it returns "prose"; for a pure
// active config line with a trailing `# prose`, it returns "prose"; for a config line
// with no trailing explanation it returns "". A whole-line prose comment (no `=`
// before the explaining `#`) is also returned so the model-agnostic scan covers
// section headers and free prose.
func commentExplanation(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	// A config line (active or commented) carries its explanation after the value's
	// trailing `#`. Locate the value's `=`; the explanation is the comment after it.
	body := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
	if eq := strings.Index(body, "="); eq > 0 {
		if hash := strings.Index(body[eq:], "#"); hash >= 0 {
			return strings.TrimSpace(body[eq+hash+1:])
		}
		return ""
	}
	// No config assignment: the whole comment body is prose to scan.
	if strings.HasPrefix(trimmed, "#") {
		return body
	}
	return ""
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
	"[commit]",
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
	if cfg.AICommand != "claude -p --model sonnet" {
		t.Errorf("AICommand = %q, want %q", cfg.AICommand, "claude -p --model sonnet")
	}
	// Assert the ACTIVE shared timeout key (top-level) survived the round-trip at its
	// 60s default. The per-verb resolution is NOT asserted here because uncommenting
	// the template deliberately activates the per-verb `timeout = 120` override lines,
	// which would (correctly) win over the shared value for each verb.
	if cfg.Timeout == nil || *cfg.Timeout != config.DefaultTimeout {
		t.Errorf("shared timeout = %v, want %v", cfg.Timeout, config.DefaultTimeout)
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
