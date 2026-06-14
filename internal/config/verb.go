package config

// Verb is the typed, CLOSED enum identifying which verb table a layered config
// accessor resolves against. It is the verb parameter the layered accessors
// (AICommandFor / TimeoutFor) accept — a named int type
// rather than a raw string so callers MUST pass one of the two named constants and a
// typo cannot silently fall through to the shared baseline.
//
// There are EXACTLY TWO values, one per verb table — VerbRelease ([release]) and
// VerbCommit ([commit]) — and deliberately NO `regenerate` value: regenerate is not a
// separate verb, it re-runs the release-notes task and so rides on [release]. With no
// `regenerate` member the regenerate routing is un-missable —
// internal/engine/regenerate_fresh.go can only pass VerbRelease.
//
// The closed two-value set makes the accessor's domain EXHAUSTIVE BY CONSTRUCTION:
// there is no unknown/third/zero member distinct from the two real verbs, so there is
// no "unrecognized verb" branch to handle. The zero value is VerbRelease (iota-0), a
// REAL verb table — never a silent unknown that could fall through to the shared
// baseline. Adding a third constant would reintroduce exactly that fall-through, so
// the set stays closed at two.
type Verb int

const (
	// VerbRelease selects the [release] table. It is iota-0, so the zero value of Verb
	// resolves to a real verb (release) — there is no unknown/empty member. regenerate
	// routes here too: it has no table of its own.
	VerbRelease Verb = iota
	// VerbCommit selects the [commit] table, the commit verb's per-verb overrides.
	VerbCommit
)
