package notes

import "strings"

// stubBody is the fixed body written for a DEGENERATE release: a single, honest
// line recorded under the version header (added by the Phase 1 changelog writer,
// not here). It is kept a package-level constant so the wording is tunable in one
// place. The em-dash matches the spec's example wording verbatim.
//
// This is the small-end mirror of the max_diff_lines guard: where a too-large
// diff routes to abort/fallback, a too-small (empty/whitespace-only) diff routes
// to this stub — a truthful record for a no-op release, never a hallucination,
// never a hard error, never a skipped entry.
const stubBody = "Maintenance release — no notable source changes"

// IsDegenerate reports whether the POST-exclusion diff carries nothing notable:
// it is EMPTY or WHITESPACE-ONLY (spaces, tabs, newlines, CR, in any combination).
// When true, the caller (the notes-path precedence, a later task) writes the
// StubBody entry and NEVER calls the AI — an empty diff is the one input the AI
// will reliably hallucinate on.
//
// The input is ALREADY post-exclusion (the output of AssembleDiff, where git has
// dropped CHANGELOG.md and any configured diff_exclude paths). So three distinct
// situations collapse to the SAME degenerate signal and are handled uniformly:
//   - a re-tag with no source change → empty diff;
//   - a release where every changed file fell under exclusion (Phase 2: only
//     CHANGELOG.md changed) → empty diff straight from git;
//   - pure churn that leaves only whitespace → whitespace-only diff.
//
// Whitespace AROUND real content does NOT make a diff degenerate — only an
// entirely-blank diff does. strings.TrimSpace removes the leading/trailing
// whitespace before testing for emptiness, so a diff padded with blank lines but
// carrying a real hunk is correctly NOT degenerate.
//
// This is a pure function of the diff: no git, no AI, no I/O. Its structural
// guarantee — that the degenerate path never invokes the AI — is that this file
// imports nothing AI-related (only strings); there is no transport to reach.
func IsDegenerate(diff string) bool {
	return strings.TrimSpace(diff) == ""
}

// StubBody returns the minimal, honest body for the degenerate path: a single
// non-empty line (see stubBody), NOT an error and NOT a skipped/empty entry. It
// is a normal body string — the caller flows it to the sinks under the version
// header, exactly like any other notes body. Like IsDegenerate, it touches no AI.
func StubBody() string {
	return stubBody
}
