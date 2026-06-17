package setupguide_test

import (
	"strings"
	"testing"

	"mint/internal/config"
	"mint/internal/configtest"
	"mint/internal/setupguide"
)

// configReferenceBody returns the slice of the emitted guide at and after the
// config-reference marker — the section whose table this task renders. Driving
// the row assertions from this slice (rather than the whole guide) keeps them
// scoped to the rendered table.
func configReferenceBody(t *testing.T) string {
	t.Helper()

	body := setupguide.Guide()
	idx := strings.Index(body, setupguide.MarkerConfigReference)
	if idx < 0 {
		t.Fatalf("guide is missing the config-reference marker %q", setupguide.MarkerConfigReference)
	}
	return body[idx:]
}

// containsRowLine reports whether section carries a single line that holds the
// key, the level cell, the default cell, and the description together — the
// per-row presence the drift-resistant assertions key on. A markdown row
// "| key | level | default | description |" satisfies this; metadata split
// across lines does not.
//
// A blank default (defaultDefault == "") is the LOAD-BEARING special case: a
// plain strings.Contains(line, "") is unconditionally true and would assert
// nothing about the blank cell, so this routes a blank default through the exact
// single-space-cell shape check (blankDefaultCellRenders) — the row's parsed
// default cell must be empty AND the delimiter intact — instead of the vacuous
// Contains. Non-blank defaults stay driven by the verbatim row.Default token.
func containsRowLine(section, key, level, defaultDefault, description string) bool {
	for _, line := range strings.Split(section, "\n") {
		if !strings.Contains(line, key) ||
			!strings.Contains(line, level) ||
			!strings.Contains(line, description) {
			continue
		}
		if defaultDefault == "" {
			if blankDefaultCellRenders(line) {
				return true
			}
			continue
		}
		if strings.Contains(line, defaultDefault) {
			return true
		}
	}
	return false
}

// blankDefaultCellRenders reports whether line is a well-formed four-column
// markdown row whose DEFAULT cell (third column) is the single-space blank cell
// defaultCell emits for an empty SoT default — the `| | ` shape: a lone space
// padded by the surrounding `| ... | ` separators, so the RAW (untrimmed) default
// field is exactly three spaces. Inspecting the raw field (not the trimmed cell)
// is what makes this bite on all three blank-default regressions:
//   - "auto" (or any non-blank token): the raw field would carry that token.
//   - a truly-empty cell with no space (`||`): the raw field would be two spaces,
//     not three.
//   - a dropped/duplicated delimiter: the row would not split into the expected
//     six pipe-delimited fields.
//
// It does NOT collapse to Contains(line, "").
func blankDefaultCellRenders(line string) bool {
	// renderConfigReference joins cells with the fixed "| ... | ... |" delimiter
	// (a leading "| ", " | " between columns, a trailing " |"), so a four-column
	// row splits on "|" into six fields: a leading "", four padded cells, a
	// trailing "". The default cell is the third padded field (index 3).
	const blankDefaultField = "   " // " " separator + defaultCell("") space + " " separator
	fields := strings.Split(line, "|")
	if len(fields) != 6 {
		return false
	}
	return fields[3] == blankDefaultField
}

// sectionMarkers pairs each required section's exported marker constant with a
// human label used in failure messages. The structural test keys on these
// CONSTANTS, never on representative prose, so a wording change inside any
// section cannot break the test and removing a marker must break it.
var sectionMarkers = []struct {
	label  string
	marker string
}{
	{"pipeline", setupguide.MarkerPipeline},
	{"etiquette", setupguide.MarkerEtiquette},
	{"minimalism", setupguide.MarkerMinimalism},
	{"existing-config", setupguide.MarkerExistingConfig},
	{"config-reference", setupguide.MarkerConfigReference},
}

// TestGuide_EmitsEverySectionMarker proves Guide() carries each of the five
// required section markers. Detection keys on the marker CONSTANTS, decoupled
// from prose, so this is the structural presence proof the spec's "Stable
// section markers" mandate requires.
func TestGuide_EmitsEverySectionMarker(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	for _, sm := range sectionMarkers {
		if !strings.Contains(body, sm.marker) {
			t.Errorf("guide missing %s section marker %q", sm.label, sm.marker)
		}
	}
}

// TestGuide_EachMarkerAppearsExactlyOnce strengthens the presence proof: each marker
// must occur EXACTLY once in Guide(). Presence alone (TestGuide_EmitsEverySectionMarker)
// survives a duplicated section — e.g. a copy-pasted block emitting its marker twice —
// which the doc contract ("emitted on its own line immediately before its section")
// forbids. Counting per marker over the rendered body closes that gap.
func TestGuide_EachMarkerAppearsExactlyOnce(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	for _, sm := range sectionMarkers {
		if got := strings.Count(body, sm.marker); got != 1 {
			t.Errorf("guide contains %s section marker %q %d times, want exactly 1", sm.label, sm.marker, got)
		}
	}
}

// TestConfigReference_DescriptionsCarryNoTablePipe enforces the markdown-table contract
// at the SoT boundary: the config reference renders each row into a four-column
// `key | level | default | description` row with the Description written unescaped, so a
// Description containing a literal "|" would silently split the row into extra columns
// while the per-row strings.Contains assertions still passed. No description carries a
// pipe today; this fails loudly (naming the offending row) if one is ever added.
func TestConfigReference_DescriptionsCarryNoTablePipe(t *testing.T) {
	t.Parallel()

	for _, row := range config.MetadataRows() {
		if strings.Contains(row.Description, "|") {
			t.Errorf("SoT row (key %q, level %q) Description contains a '|' that breaks the markdown table column: %q", row.Key, row.Level, row.Description)
		}
	}
}

// TestGuide_MarkersAreUnique guards against two sections sharing a marker — a
// collision would let one section's presence mask another's absence, defeating
// the per-section structural proof.
func TestGuide_MarkersAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]string, len(sectionMarkers))
	for _, sm := range sectionMarkers {
		if prior, dup := seen[sm.marker]; dup {
			t.Errorf("marker %q is shared by %s and %s sections", sm.marker, prior, sm.label)
		}
		seen[sm.marker] = sm.label
	}
}

// TestGuide_MarkersAreCommentAnchorsNotProse proves the markers are HTML-comment
// anchors that will not collide incidentally with prose: each begins with the
// fixed `<!-- mint:section:` namespace and closes the comment. This is what
// keeps detection decoupled from the body text.
func TestGuide_MarkersAreCommentAnchorsNotProse(t *testing.T) {
	t.Parallel()

	for _, sm := range sectionMarkers {
		if !strings.HasPrefix(sm.marker, "<!-- mint:section:") {
			t.Errorf("%s marker %q must use the <!-- mint:section: anchor namespace", sm.label, sm.marker)
		}
		if !strings.HasSuffix(sm.marker, "-->") {
			t.Errorf("%s marker %q must close the HTML comment with -->", sm.label, sm.marker)
		}
	}
}

// TestGuide_EachMarkerSitsOnItsOwnLine proves every marker is emitted on its own
// line immediately preceding its section. This is the negative guard the spec
// demands: detection is line-anchored on the marker, so prose alone — a section
// body without its marker line — would NOT be detected. A marker buried mid-line
// inside prose would fail here.
func TestGuide_EachMarkerSitsOnItsOwnLine(t *testing.T) {
	t.Parallel()

	lines := strings.Split(setupguide.Guide(), "\n")
	for _, sm := range sectionMarkers {
		if !lineEquals(lines, sm.marker) {
			t.Errorf("%s marker %q must appear on its own line, not embedded in prose", sm.label, sm.marker)
		}
	}
}

// lineEquals reports whether marker appears as a whole line (ignoring leading or
// trailing whitespace) anywhere in lines.
func lineEquals(lines []string, marker string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) == marker {
			return true
		}
	}
	return false
}

// TestGuide_MentionsReleaseShimRole proves the pipeline section carries the
// one-line release-shim role mention: what ./release is and that mint init
// creates it, so the agent's picture of the release pipeline is complete.
func TestGuide_MentionsReleaseShimRole(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	if !strings.Contains(body, "./release") {
		t.Error("guide must mention the ./release shim by name")
	}
	if !strings.Contains(body, "mint init") {
		t.Error("guide must say mint init creates the release shim")
	}
}

// TestGuide_FirstProcedureStepIsCwdConfirm proves the cwd-confirm safety step is
// the FIRST ordered procedure step — the in-instructions safety net that
// replaces mint setup's missing cwd guard. The procedure numbers its steps, so
// step "1." must reference confirming the working directory / repo root before
// any other numbered step.
func TestGuide_FirstProcedureStepIsCwdConfirm(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	firstStep := firstNumberedStep(body)
	if firstStep == "" {
		t.Fatal("guide carries no numbered procedure steps")
	}
	lower := strings.ToLower(firstStep)
	if !strings.Contains(lower, "working directory") && !strings.Contains(lower, "repo root") {
		t.Errorf("first procedure step must confirm the working directory / repo root, got: %q", firstStep)
	}
}

// firstNumberedStep returns the first line that opens an ordered "1." procedure
// step, trimmed. Returns "" if none is found.
func firstNumberedStep(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "1.") {
			return trimmed
		}
	}
	return ""
}

// TestGuide_LearnMintStepNamesMintsOwnReadme proves procedure step 2 ("Learn
// mint") points the agent at MINT's OWN README — the human config-reference
// surface and the source of mint's commands/surface/philosophy — rather than the
// ambiguous "the project's README", which most naturally reads as the TARGET
// project's README (which would not document mint). The disambiguated wording is
// "mint's README". The minimalism-philosophy clause must survive the reword.
func TestGuide_LearnMintStepNamesMintsOwnReadme(t *testing.T) {
	t.Parallel()

	step := numberedStep(setupguide.Guide(), "2.")
	if step == "" {
		t.Fatal("guide carries no numbered procedure step 2")
	}
	if !strings.Contains(step, "mint's README") {
		t.Errorf("procedure step 2 must name mint's own README unambiguously (\"mint's README\"), got: %q", step)
	}
	if strings.Contains(step, "the project's README") {
		t.Errorf("procedure step 2 must not call it \"the project's README\" (reads as the target project's README), got: %q", step)
	}
	if !strings.Contains(strings.ToLower(step), "minimalist philosophy") {
		t.Errorf("procedure step 2 must retain the minimalism-philosophy clause, got: %q", step)
	}
}

// numberedStep returns the procedure step that opens with the given ordered
// prefix (e.g. "2."), joined across its continuation lines up to the next
// numbered step, and trimmed. Procedure steps wrap across multiple indented
// lines, so this gathers the whole step rather than just its opening line.
// Returns "" if no step opens with prefix.
func numberedStep(body, prefix string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), prefix) {
			continue
		}
		step := []string{strings.TrimSpace(line)}
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimSpace(next)
			if isNumberedStepOpener(trimmed) || trimmed == "" {
				break
			}
			step = append(step, trimmed)
		}
		return strings.Join(step, " ")
	}
	return ""
}

// isNumberedStepOpener reports whether a trimmed line opens an ordered procedure
// step ("1.", "2.", …) — the boundary numberedStep stops a step at.
func isNumberedStepOpener(trimmed string) bool {
	dot := strings.IndexByte(trimmed, '.')
	if dot <= 0 {
		return false
	}
	for _, r := range trimmed[:dot] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// TestGuide_CarriesAIModelPerVerbMapping proves the guide explains the
// AI-model-per-verb mapping: the same model -> the shared top-level ai_command;
// different models -> [release].ai_command + [commit].ai_command per-verb
// overrides.
func TestGuide_CarriesAIModelPerVerbMapping(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	for _, want := range []string{"ai_command", "[release].ai_command", "[commit].ai_command"} {
		if !strings.Contains(body, want) {
			t.Errorf("AI-model-per-verb mapping must reference %q", want)
		}
	}
}

// TestGuide_CarriesDiffExcludeScope proves the guide carries diff_exclude scope:
// it is release-notes noise (tracked process/meta/doc files such as .workflows/,
// .claude/, docs/), not generated code, and gitignore'd paths are already
// absent.
func TestGuide_CarriesDiffExcludeScope(t *testing.T) {
	t.Parallel()

	body := setupguide.Guide()

	if !strings.Contains(body, "diff_exclude") {
		t.Error("guide must reference diff_exclude")
	}
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "release-notes noise") && !strings.Contains(lower, "release notes noise") {
		t.Error("diff_exclude scope must frame it as release-notes noise, not generated code")
	}
	if !strings.Contains(lower, "gitignore") {
		t.Error("diff_exclude scope must note that gitignore'd paths are already absent from the diff")
	}
}

// TestGuide_ConfigReferenceHasLinePerSoTRow proves the config-reference section
// renders one table line per config.MetadataRows() row, each carrying the key,
// the level (via setupguide.LevelCell — MetadataLevel.String() with the
// shared-level placeholder), the verbatim default token, and the description —
// together on a single line. The expectations are DRIVEN by
// config.MetadataRows() (not a frozen literal table), so adding or removing a
// SoT row propagates here without a test edit, and a re-derived table that
// diverged from the SoT would fail this assertion. The level cell expectation
// is single-sourced through the production LevelCell seam, so it tracks
// production's placeholder rather than re-deriving it.
func TestGuide_ConfigReferenceHasLinePerSoTRow(t *testing.T) {
	t.Parallel()

	section := configReferenceBody(t)

	for _, row := range config.MetadataRows() {
		if !containsRowLine(section, row.Key, setupguide.LevelCell(row.Level), row.Default, row.Description) {
			t.Errorf("config reference missing a line for SoT row (level %q, key %q, default %q)",
				row.Level, row.Key, row.Default)
		}
	}
}

// TestGuide_ConfigReferenceRendersDualLevelKeysDistinctly proves ai_command and
// timeout each render as THREE distinct rows — shared, [release], [commit] —
// never collapsed. Each (level, key) pair must appear as its own line carrying
// that level's cell, mirroring the SoT's per-level model.
func TestGuide_ConfigReferenceRendersDualLevelKeysDistinctly(t *testing.T) {
	t.Parallel()

	section := configReferenceBody(t)
	levels := []config.MetadataLevel{config.LevelShared, config.LevelRelease, config.LevelCommit}

	for _, key := range []string{"ai_command", "timeout"} {
		for _, level := range levels {
			if !lineHasKeyAtLevel(section, key, setupguide.LevelCell(level)) {
				t.Errorf("config reference missing a distinct %q row at level %q", key, setupguide.LevelCell(level))
			}
		}
	}
}

// lineHasKeyAtLevel reports whether section carries a single line holding both
// the key and the level cell — the per-(level, key) row distinctness check the
// dual-level assertion needs.
func lineHasKeyAtLevel(section, key, levelCell string) bool {
	for _, line := range strings.Split(section, "\n") {
		if strings.Contains(line, key) && strings.Contains(line, levelCell) {
			return true
		}
	}
	return false
}

// TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim proves the default-column
// tokens are carried verbatim from the SoT — the empty-collection "[]"
// (diff_exclude), the sentinel "auto" (release_branch / provider), the per-verb
// inherit "shared" ([release]/[commit] ai_command/timeout), and a blank cell for
// empty-string-default keys ([release] context). The render performs zero
// metadata logic; it copies row.Default through.
func TestGuide_ConfigReferenceCarriesDefaultTokensVerbatim(t *testing.T) {
	t.Parallel()

	section := configReferenceBody(t)
	rows := configtest.MustByLevelKey(t)

	tests := []struct {
		name  string
		level config.MetadataLevel
		key   string
	}{
		{"empty-collection diff_exclude renders []", config.LevelShared, "diff_exclude"},
		{"sentinel-auto release_branch renders auto", config.LevelRelease, "release_branch"},
		{"sentinel-auto provider renders auto", config.LevelRelease, "provider"},
		{"per-verb [release].ai_command inherits shared", config.LevelRelease, "ai_command"},
		{"per-verb [commit].timeout inherits shared", config.LevelCommit, "timeout"},
		{"empty-string [release].context renders blank", config.LevelRelease, "context"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := rows[configtest.RowKey{Level: tt.level, Key: tt.key}]
			if !containsRowLine(section, row.Key, setupguide.LevelCell(row.Level), row.Default, row.Description) {
				t.Errorf("config reference must carry default %q verbatim for (level %q, key %q)",
					row.Default, tt.level, tt.key)
			}
		})
	}
}

// TestGuide_ConfigReferenceBlankDefaultRendersSingleSpaceCell pins the
// blank-default render at the AGENT-FACING seam: a key whose SoT default is the
// empty string (e.g. [release].context) must render the well-formed single-space
// default cell — the `| | ` shape — so the markdown row stays valid and the
// blank-vs-auto-vs-shared distinction the minimalism guidance rests on survives
// to the agent. This is the real assertion the blank case lacked: a regression
// that emitted "auto" (or any non-blank token), collapsed the cell to truly empty
// (`||`), or dropped a delimiter for a blank-default key turns this red. The row
// is located by its exact Key cell and the assertion runs through the production
// render, so it tracks defaultCell's actual output rather than re-deriving it.
func TestGuide_ConfigReferenceBlankDefaultRendersSingleSpaceCell(t *testing.T) {
	t.Parallel()

	section := configReferenceBody(t)
	rows := configtest.MustByLevelKey(t)

	// [release].context is a genuinely-no-value key: its SoT default is "" (not a
	// sentinel), so it is the canonical blank-default row to pin.
	const blankKey = "context"
	row := rows[configtest.RowKey{Level: config.LevelRelease, Key: blankKey}]
	if row.Default != "" {
		t.Fatalf("(level %q, key %q) Default = %q, want blank — pick a blank-default key for this seam test",
			config.LevelRelease, blankKey, row.Default)
	}

	line := rowLineFor(section, blankKey)
	if line == "" {
		t.Fatalf("config reference has no row line for blank-default key %q", blankKey)
	}

	// The line must carry this exact row (level + description) AND render the
	// single-space default cell — proven via the raw-field shape check, not a
	// trimmed-cell compare, so a truly-empty cell or a non-blank token bites.
	if !strings.Contains(line, setupguide.LevelCell(row.Level)) || !strings.Contains(line, row.Description) {
		t.Fatalf("row line for %q does not carry the expected level/description: %q", blankKey, line)
	}
	if !blankDefaultCellRenders(line) {
		t.Errorf("blank-default key %q must render the single-space `| | ` default cell, got line: %q", blankKey, line)
	}
}

// TestGuide_ConfigReferenceRendersLevelViaString proves the level column is
// rendered from MetadataLevel.String() (TOML form): each non-shared row carries
// its bracketed header verbatim on its line. The shared rows carry the empty
// String() form, surfaced as the "top-level" placeholder cell. The expectation
// flows through the production setupguide.LevelCell seam, so changing
// production's placeholder turns this red rather than silently tracking it.
func TestGuide_ConfigReferenceRendersLevelViaString(t *testing.T) {
	t.Parallel()

	section := configReferenceBody(t)

	for _, row := range config.MetadataRows() {
		if !lineHasKeyAtLevel(section, row.Key, setupguide.LevelCell(row.Level)) {
			t.Errorf("config reference must render level %q (via String()) on the %q row",
				setupguide.LevelCell(row.Level), row.Key)
		}
	}
}

// TestGuide_ConfigReferenceSharedLevelUsesNonCollidingToken proves the shared
// (top-level) rows render their LEVEL cell with a token that does NOT collide
// with the Default-column inherit "shared" token. A shared-level row's level
// cell must carry "top-level" and must NOT carry the bare "shared" word, so the
// reading agent never sees "shared" meaning two things in one table.
func TestGuide_ConfigReferenceSharedLevelUsesNonCollidingToken(t *testing.T) {
	t.Parallel()

	section := configReferenceBody(t)

	// max_diff_lines is a shared-level-only key (no dual-level twin), so its row
	// is unambiguously a shared row: its level cell is the placeholder under test.
	const sharedOnlyKey = "max_diff_lines"
	line := rowLineFor(section, sharedOnlyKey)
	if line == "" {
		t.Fatalf("config reference has no row line for shared-only key %q", sharedOnlyKey)
	}

	levelCell := levelCellOf(line)
	if levelCell != "top-level" {
		t.Errorf("shared-level row %q level cell = %q, want %q", sharedOnlyKey, levelCell, "top-level")
	}
	if strings.Contains(levelCell, "shared") {
		t.Errorf("shared-level row %q level cell %q must not collide with the Default inherit token %q",
			sharedOnlyKey, levelCell, "shared")
	}
}

// TestLevelCell_OutOfRangeLevelDoesNotRenderTopLevel proves the render seam ONLY maps
// the genuine LevelShared (whose String() is "") to the "top-level" placeholder: an
// out-of-range MetadataLevel now carries a non-empty Stringer sentinel, so LevelCell
// returns that sentinel verbatim rather than masquerading as a valid shared/top-level
// cell in the agent-facing config table. This is the render-path half of the closed-enum
// enforcement (the config package's String() distinguishability test is the other half).
func TestLevelCell_OutOfRangeLevelDoesNotRenderTopLevel(t *testing.T) {
	t.Parallel()

	// One past the last declared constant — a value the closed enum never produces.
	outOfRange := config.LevelCommit + 1

	got := setupguide.LevelCell(outOfRange)
	if got == "top-level" {
		t.Errorf("LevelCell(out-of-range MetadataLevel(%d)) = %q, must not render the shared/top-level cell", int(outOfRange), got)
	}
	if got != outOfRange.String() {
		t.Errorf("LevelCell(out-of-range MetadataLevel(%d)) = %q, want the level's sentinel %q rendered verbatim", int(outOfRange), got, outOfRange.String())
	}
}

// rowLineFor returns the markdown table row line in section whose first cell
// (the Key column) is exactly key, or "" if none matches. Matching on the key
// cell (not Contains) pins the right row even when several rows share a key.
func rowLineFor(section, key string) string {
	for _, line := range strings.Split(section, "\n") {
		cells := splitRow(line)
		if len(cells) >= 1 && cells[0] == key {
			return line
		}
	}
	return ""
}

// levelCellOf returns the LEVEL column (second cell) of a markdown table row.
func levelCellOf(line string) string {
	cells := splitRow(line)
	if len(cells) < 2 {
		return ""
	}
	return cells[1]
}

// splitRow splits a "| a | b | c |" markdown row into its trimmed cell values,
// dropping the empty leading/trailing fields the surrounding pipes produce.
func splitRow(line string) []string {
	if !strings.HasPrefix(strings.TrimSpace(line), "|") {
		return nil
	}
	parts := strings.Split(line, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts[1 : len(parts)-1] {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}
