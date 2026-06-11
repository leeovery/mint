package commit

// StagingMode selects WHAT goes into the commit — the resolved form of the bare /
// -a / -A flag surface (see the Staging Model in commit's spec). The cmd layer
// resolves the mutually-exclusive -a/-A flags into exactly one of these and threads
// it into the orchestrator; the deferred-staging machinery (Phase 2 tasks 2-2/2-3)
// consumes it to compute the would-be-committed diff read-only and to apply the
// staging only after the gate accepts.
//
// The zero value is StagedOnly so an unset mode is the Phase 1 default (commit the
// index exactly as staged), keeping a bare `mint commit` byte-identical to Phase 1.
type StagingMode int

const (
	// StagedOnly commits the index exactly as staged — no `git add`. It is the zero
	// value and the default when neither -a nor -A is given (the Phase 1 behaviour).
	StagedOnly StagingMode = iota
	// All is `-a`/`--all` = `git commit -a` semantics: tracked modifications +
	// deletions, NO untracked files. Muscle-memory faithful to git's `commit -a`.
	All
	// AddAll is `-A`/`--add-all` = `git add -A` then commit: everything including
	// untracked files — the user's `git add .` habit in one shot.
	AddAll
)
