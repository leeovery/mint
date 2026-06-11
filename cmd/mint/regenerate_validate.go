package main

import (
	"fmt"

	"mint/internal/engine"
)

// validateRegenerateRequest applies the semantic source × target axis-contract
// to a parsed regenerateRequest (task 5-2). It runs AFTER the 5-1 structural
// parse and needs the loaded config's changelog toggle, so it lives here rather
// than in the parser. It returns the validated request — with Target possibly
// RESOLVED (--reuse with no --target becomes --target release) — or a fail-loud
// error with the exact spec message.
//
// The checks are ordered so the MOST SPECIFIC message wins:
//  1. --reuse ⇒ release-only: --reuse --target changelog/both is rejected before
//     anything else (its source IS the notes record, so targeting the changelog
//     would write a file from itself).
//  2. --reuse with no --target resolves Target to release — unconditionally, so
//     -y does not change the outcome.
//  3. changelog-disabled: a resolved changelog/both target with changelog=false
//     is rejected (mint never silently creates a CHANGELOG the project opted out
//     of). Delegated to validateTargetAgainstChangelog so task 5-12 can reuse it
//     for the batch up-front config check.
//  4. fresh + -y with no --target is rejected: -y skips the interactive target
//     prompt and the fresh path has no default surface, so there is nothing safe
//     to guess unattended.
//
// A fresh run WITHOUT -y and without --target is deliberately NOT an error here:
// the interactive prompt (task 5-10) resolves the target later, so Target is
// left targetUnset and the request proceeds.
func validateRegenerateRequest(req regenerateRequest, changelogEnabled bool) (regenerateRequest, error) {
	if req.Source == sourceReuse {
		if req.Target == targetChangelog || req.Target == targetBoth {
			return regenerateRequest{}, fmt.Errorf("--reuse writes the provider release only; it cannot target the changelog")
		}
		// --reuse implies --target release; resolve it regardless of -y.
		if req.Target == targetUnset {
			req.Target = targetRelease
		}
	}

	if err := validateTargetAgainstChangelog(req.Target, changelogEnabled); err != nil {
		return regenerateRequest{}, err
	}

	if req.Source == sourceFresh && req.Yes && req.Target == targetUnset {
		return regenerateRequest{}, fmt.Errorf("--target is required with --fresh -y")
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
