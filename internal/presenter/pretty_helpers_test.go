package presenter_test

import (
	"strings"
	"testing"

	"mint/internal/presenter"
)

// TestRuleCapForTestMirrorsProductionRuleCap guards the single sourcing of the
// decorative rule width: RuleCapForTest is exported from production's ruleCap (via
// export_test.go) so the test-side rule helpers can't drift from the real cap. This
// asserts the value the rest of the package's layout tests bake into their expected
// rules is the spec's 50-column cap.
func TestRuleCapForTestMirrorsProductionRuleCap(t *testing.T) {
	if presenter.RuleCapForTest != 50 {
		t.Errorf("RuleCapForTest = %d, want 50 (the spec rule cap)", presenter.RuleCapForTest)
	}
}

// notesTitlePrefix is the SINGLE definition of the fixed title segment the pretty
// titled rule opens with — "── release notes · v{X} " — before the trailing run of
// U+2500 fills out to the capped width. Both the layout tests (which build the exact
// expected rule) and the width tests (which assert the rendered rule keeps this
// prefix) source the literal from here, so the prefix lives in exactly one place on
// the test side and mirrors the presenter's own notesTitledRule construction.
func notesTitlePrefix(version string) string {
	return "── release notes · v" + version + " "
}

// notesTitledRule builds the expected titled opener rule for a version: the
// notesTitlePrefix filled with U+2500 up to the cap width, with the SAME minimum-one
// fill clamp production applies (so a prefix longer than the cap keeps its full title
// and never produces a negative repeat count). The width is sourced once from
// presenter.RuleCapForTest — the default presenter (termWidth 0) renders at ruleCap.
func notesTitledRule(version string) string {
	prefix := notesTitlePrefix(version)
	fill := presenter.RuleCapForTest - len([]rune(prefix))
	if fill < 1 {
		fill = 1
	}
	return prefix + strings.Repeat("─", fill)
}

// notesClosingRule builds the expected closing rule: U+2500 repeated to the cap
// width, sourced once from presenter.RuleCapForTest.
func notesClosingRule() string {
	return strings.Repeat("─", presenter.RuleCapForTest)
}

// ruleDisplayWidth counts the display columns of a rendered rule line: the rune
// count of the line with any ANSI SGR escapes stripped. The decorative rule is built
// from single-column ASCII/box-drawing runes, so the rune count IS the column
// width — this lets a test assert "the rule is N columns wide" independent of colour
// styling.
func ruleDisplayWidth(line string) int {
	return len([]rune(stripANSI(line)))
}

// stripANSI removes CSI SGR escape sequences (ESC '[' … 'm') from s so a rendered
// line can be measured by display columns regardless of colour styling. It is a
// minimal stripper sufficient for the dim styling lipgloss emits on the rules, and
// is the single shared ANSI-strip primitive for the pretty tests.
func stripANSI(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == 0x1b && i+1 < len(runes) && runes[i+1] == '[' {
			i += 2
			for i < len(runes) && runes[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}
