package engine

// This file is the regenerate provider create-or-update DISPATCH (task 5-7): the
// per-version decision that probes whether a provider release exists at the tag and
// routes to UpdateRelease (it does) or CreateRelease (it doesn't). The user is never
// asked to choose — "refresh existing text" and "mass-heal a missing release" are
// the same command, resolved per version, so an `--all` batch transparently mixes
// updates and creates by calling this once per version.
//
// Everything stays behind the publish.Publisher seam: the helper takes a Publisher
// (plus tag/title/body), so NO GitHub/driver specifics leak into the orchestration —
// the probe and the two writes are all interface methods. A genuine probe failure
// (not just "release not found") is SURFACED without dispatching either write, so
// the dispatch never silently defaults to create-or-update on a real failure.
//
// This task owns the helper only; wiring it into the single-version (5-9) and `--all`
// batch (5-11) execution is later work — both call DispatchRelease per version.

import (
	"context"
	"fmt"

	"mint/internal/publish"
)

// DispatchRelease resolves create-vs-update for ONE version and writes the provider
// release through the Publisher seam: it probes ReleaseExists(tag) and dispatches
// UpdateRelease when the release exists, CreateRelease when it is absent. The
// decision is per version, so an `--all` batch calls this once per tag and mixes
// updates and creates with no extra branching.
//
// A genuine probe failure (an error from ReleaseExists — gh missing, auth, network,
// anything that is NOT a clean "release absent") is surfaced WITHOUT dispatching
// either write, so the dispatch never silently falls back to create-or-update on a
// real failure. The user is never prompted to choose.
//
// It returns the created/updated release URL the routed write reported, so the
// forward release path can thread it into the success footer; on a probe failure the
// URL is empty alongside the error.
func DispatchRelease(ctx context.Context, p publish.Publisher, tag, title, body string) (string, error) {
	exists, err := p.ReleaseExists(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("probing provider release for tag %s: %w", tag, err)
	}
	if exists {
		return p.UpdateRelease(ctx, tag, title, body)
	}
	return p.CreateRelease(ctx, tag, title, body)
}
