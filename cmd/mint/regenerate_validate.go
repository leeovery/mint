package main

import (
	"fmt"

	"mint/internal/engine"
)

// validateRegenerateRequest applies the semantic axis checks to a parsed
// regenerateRequest. It runs AFTER the structural parse and needs the loaded config's
// changelog toggle, so it lives here rather than in the parser. It returns the
// validated request or a fail-loud error with the exact message.
//
// The source and target axes are ORTHOGONAL — every source (fresh, reuse,
// from-release) can write every target (release, changelog, both) — so there is no
// source⇒target constraint and no implied target. Two checks remain:
//  1. changelog-disabled: a changelog/both target with changelog=false is rejected
//     (mint never silently creates a CHANGELOG the project opted out of). Delegated to
//     validateTargetAgainstChangelog so the batch path reuses it for its up-front
//     config check.
//  2. -y with no --target is rejected for EVERY source: -y skips the interactive target
//     prompt and no source has a safe default surface to guess unattended (mint fails
//     loud rather than picking a live surface to rewrite for the user).
//
// A run WITHOUT -y and without --target is deliberately NOT an error here: the
// interactive prompt resolves the target later, so Target is left targetUnset and the
// request proceeds.
func validateRegenerateRequest(req regenerateRequest, changelogEnabled bool) (regenerateRequest, error) {
	if err := validateTargetAgainstChangelog(req.Target, changelogEnabled); err != nil {
		return regenerateRequest{}, err
	}

	if req.Yes && req.Target == targetUnset {
		return regenerateRequest{}, fmt.Errorf("--target is required with -y")
	}

	return req, nil
}

// validateTargetAgainstChangelog rejects a changelog-touching target when the
// changelog is disabled in config. It operates on an already-RESOLVED target so
// it is reusable: task 5-2 calls it on a single resolved request, and task 5-12
// reuses it to validate a batch's target up front (a static config fact aborts
// the whole batch before it starts, never skipped per version). A release/unset
// target, or an enabled changelog, is a no-op.
func validateTargetAgainstChangelog(target regenerateTarget, changelogEnabled bool) error {
	if !changelogEnabled && (target == targetChangelog || target == targetBoth) {
		return engine.ErrChangelogDisabled
	}
	return nil
}
