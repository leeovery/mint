package config_test

// This is the OPTIONAL README key-presence TRIPWIRE (spec: "README — config reference
// verification", "Definition of done" → "README tripwire (optional)"). The README's
// per-key reference tables are the HUMAN config reference surface, but — unlike the
// machine surfaces (mint setup's SoT table) — nothing mechanically couples them to the
// schema. The Phase 1 drift test guards the SoT↔schema relationship on the FULL
// (level, key) pair; this tripwire is deliberately COARSER: it asserts every distinct
// schema key NAME appears somewhere in the README text, so a removed/renamed/added key
// that the README hasn't tracked fails the build.
//
// Why MetadataRows() is the key source (not the decode-shape toml tags directly): it
// keeps the tripwire, the SoT, and the schema ONE authoritative chain — MetadataRows()
// IS the SoT, and the Phase 1 bijection drift test pins the SoT to the real schema
// leaf-key set. So deriving the tripwire's key names from MetadataRows() means the
// tripwire transitively tracks the schema without re-reflecting the tags here. The
// container tags release/commit/hooks are EXCLUDED by construction: MetadataRows()
// emits no row for a sub-table container (its children carry their own level), so
// collecting distinct row .Key values never yields a container name — no explicit
// filter is needed (a guard test below asserts this absence-by-construction holds).
//
// The match is a plain CASE-SENSITIVE SUBSTRING of the snake_case key name. The current
// schema names have no problematic substring overlap (no key name is a substring of
// another), so substring presence is safe and sufficient. NOTE: a future SHORT key name
// that is a substring of unrelated README prose (or of another key) would need this
// match tightened to whole-word — flagging here so such an addition gets reviewed.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mint/internal/config"
)

// distinctSchemaKeyNames returns the deduped set of schema key NAMES from the SoT,
// collapsing the dual-level ai_command/timeout to ONE name each. This is the deliberate
// difference from the Phase 1 drift test, which matches on the (level, key) PAIR: here
// we want distinct NAMES (name-presence), so a name seen at multiple levels contributes
// once. Order follows first appearance in MetadataRows() so a failure message lists
// names in schema order.
func distinctSchemaKeyNames() []string {
	seen := map[string]bool{}
	var names []string
	for _, row := range config.MetadataRows() {
		if seen[row.Key] {
			continue
		}
		seen[row.Key] = true
		names = append(names, row.Key)
	}
	return names
}

// missingKeysInREADME is the PURE comparison helper at the heart of the tripwire: given
// the README text and the set of distinct schema key names, it returns every name that
// does NOT appear as a substring of the README, in the input order. Keeping it pure
// (text + names in, missing names out) lets the negative test drive it against a
// SYNTHETIC README body missing one known key WITHOUT editing the real README or schema
// — mirroring the Phase 1 bijection helper's pure-comparison pattern. The match is a
// plain case-sensitive substring (see file header for the whole-word caveat).
func missingKeysInREADME(readme string, keys []string) []string {
	var missing []string
	for _, key := range keys {
		if !strings.Contains(readme, key) {
			missing = append(missing, key)
		}
	}
	return missing
}

// readmePath resolves the repo-root README.md from the test file's own location via
// runtime.Caller — the config test package lives at internal/config/, so the README is
// two directories up. Anchoring on the test file (not the process CWD) keeps the path
// robust regardless of where `go test` is invoked from.
func readmePath(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate the tripwire test file to anchor the README path")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "README.md")
}

// readFileNamingPath is the PURE read: it reads path and, on failure, wraps the error so
// it NAMES the attempted path. Pure (path in, text-or-error out) so the loud-fail
// contract — "a moved/renamed README fails, naming the path" — can be driven against a
// deliberately bad path WITHOUT a *testing.T proxy. The thin readREADME helper turns its
// error into the loud t.Fatalf the real tripwire needs.
func readFileNamingPath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading README at %q: %w", path, err)
	}
	return string(data), nil
}

// readREADME reads the README at path, failing LOUDLY (naming the attempted path) if it
// cannot be read — a moved/renamed README must fail the build, never produce a vacuous
// pass against empty text.
func readREADME(t *testing.T, path string) string {
	t.Helper()

	text, err := readFileNamingPath(path)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return text
}

// TestREADME_DeclaresEverySchemaKeyName is the TRIPWIRE: every distinct schema key name
// must appear as a substring of the real README. A removed/renamed key that no longer
// appears, or an added key the README hasn't documented, fails the build. ALL missing
// names are reported in one message (not just the first).
func TestREADME_DeclaresEverySchemaKeyName(t *testing.T) {
	t.Parallel()

	readme := readREADME(t, readmePath(t))
	keys := distinctSchemaKeyNames()

	if missing := missingKeysInREADME(readme, keys); len(missing) != 0 {
		t.Errorf("README is missing %d schema key name(s) — the human config reference has drifted from the schema: %v", len(missing), missing)
	}
}

// TestDistinctSchemaKeyNames_DedupesDualLevelKeys proves the dual-level ai_command and
// timeout keys are deduped to ONE name each — the deliberate difference from the Phase 1
// drift test's (level, key) pair model. Each appears exactly once in the distinct-name
// set even though MetadataRows() carries three rows for each.
func TestDistinctSchemaKeyNames_DedupesDualLevelKeys(t *testing.T) {
	t.Parallel()

	counts := map[string]int{}
	for _, name := range distinctSchemaKeyNames() {
		counts[name]++
	}

	for _, dual := range []string{"ai_command", "timeout"} {
		if counts[dual] != 1 {
			t.Errorf("distinct key name %q appears %d time(s), want exactly 1 (dual-level keys dedupe to one name)", dual, counts[dual])
		}
	}
}

// TestDistinctSchemaKeyNames_ExcludesContainerTags proves the TOML table-container tags
// release/commit/hooks are NOT in the distinct key set. They are table headers, not
// config keys — excluded by construction because MetadataRows() emits no row for a
// container. This asserts the absence-by-construction holds (no explicit filter needed).
func TestDistinctSchemaKeyNames_ExcludesContainerTags(t *testing.T) {
	t.Parallel()

	containers := map[string]bool{"release": true, "commit": true, "hooks": true}
	for _, name := range distinctSchemaKeyNames() {
		if containers[name] {
			t.Errorf("distinct key set must not contain table-container tag %q (it is a TOML header, not a config key)", name)
		}
	}
}

// TestMissingKeysInREADME_FlagsAbsentKey proves the pure comparison helper reports a
// schema key name that is ABSENT from the README — driven against a synthetic README
// body that omits tag_prefix (every other key present), WITHOUT touching the real
// README or schema. This is the negative half of the tripwire: the guard mechanism
// itself is unit-tested.
func TestMissingKeysInREADME_FlagsAbsentKey(t *testing.T) {
	t.Parallel()

	keys := []string{"ai_command", "tag_prefix", "commit_prefix"}
	// Synthetic README body that mentions every key EXCEPT tag_prefix.
	body := "config reference: ai_command sets the AI invocation; commit_prefix brands the bookkeeping commit."

	missing := missingKeysInREADME(body, keys)

	if len(missing) != 1 || missing[0] != "tag_prefix" {
		t.Errorf("missingKeysInREADME must report exactly the absent key [tag_prefix], got %v", missing)
	}
}

// TestMissingKeysInREADME_ReportsAllMissing proves the helper reports ALL missing names
// (not just the first) so a failure message names every offender at once.
func TestMissingKeysInREADME_ReportsAllMissing(t *testing.T) {
	t.Parallel()

	keys := []string{"ai_command", "tag_prefix", "version_file"}
	// Body mentions only ai_command; tag_prefix and version_file are both absent.
	body := "only ai_command is documented here."

	missing := missingKeysInREADME(body, keys)

	if len(missing) != 2 || missing[0] != "tag_prefix" || missing[1] != "version_file" {
		t.Errorf("missingKeysInREADME must report ALL absent keys in order [tag_prefix version_file], got %v", missing)
	}
}

// TestReadFileNamingPath_FailsLoudlyOnMissingFile proves the README read fails LOUDLY
// NAMING the attempted path when the file cannot be read — a moved/renamed README must
// fail the build, never vacuously pass against empty text. Driven through the pure
// readFileNamingPath against a deliberately non-existent path; readREADME surfaces this
// same error via t.Fatalf in the real tripwire.
func TestReadFileNamingPath_FailsLoudlyOnMissingFile(t *testing.T) {
	t.Parallel()

	badPath := filepath.Join(t.TempDir(), "no-such-README.md")

	text, err := readFileNamingPath(badPath)
	if err == nil {
		t.Fatalf("reading a non-existent README at %q must fail, got text %q", badPath, text)
	}
	if !strings.Contains(err.Error(), badPath) {
		t.Errorf("read error must NAME the attempted path %q, got %v", badPath, err)
	}
}
