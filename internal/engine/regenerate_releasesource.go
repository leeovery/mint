package engine

// This file is the regenerate provider-release SOURCE read (`--source release`):
// reading the EXISTING provider release body back as the source mint consumes — the
// provider-release sibling of the tag-annotation read (regenerate_tagsource.go).
//
// The read itself lives behind the publish.Publisher seam (ReadReleaseBody): the body
// was already presentation-format when the release was published, so the bytes flow
// straight to the changelog/provider write WHOLE — no parsing, no splitting, no
// validation transform. The release source runs NO AI and assembles NO diff: it
// touches only this one read.
//
// hasBody is the single branch point both regenerate modes consume, exactly mirroring
// the tag source's ReadTagBody:
//   - SINGLE mode (RequireReleaseBody) turns hasBody=false into a fail-loud error.
//   - --all mode (processOneVersion) calls Publisher.ReadReleaseBody directly and
//     skips-and-reports on hasBody=false, never writing an empty body downstream.

import (
	"context"
	"errors"
	"fmt"

	"mint/internal/publish"
)

// errReleaseSourceNoPublisher is the fail-loud precondition for the provider-release
// source on a DOWNGRADED run: reading the existing release requires a resolved provider,
// so a nil publisher (non-github / no-remote origin) aborts the run rather than
// nil-dereferencing the seam. It is surfaced after the source resolves (single:
// RegenerateRun; batch: RegenerateAllValidated), the only point the resolved source meets
// the publisher.
var errReleaseSourceNoPublisher = errors.New("reading the release body requires a resolvable provider; none was found")

// RequireReleaseBody is single-mode's fail-loud wrapper around the Publisher's
// ReadReleaseBody: it returns the published release body verbatim for the downstream
// write, or — when the release has no usable body (absent, empty, or whitespace-only) —
// the EXACT fail-loud error, so the release source stops rather than writing an
// empty body.
//
// Single mode does NOT fall back to a fresh re-diff; it directs the user there with the
// `use --source fresh` hint (the tag source's wording). (Batch --all mode calls
// Publisher.ReadReleaseBody directly and skips-and-reports instead of failing the whole
// run.)
func RequireReleaseBody(ctx context.Context, p publish.Publisher, tag string) (string, error) {
	body, hasBody, err := p.ReadReleaseBody(ctx, tag)
	if err != nil {
		return "", err
	}
	if !hasBody {
		return "", fmt.Errorf("release for tag %s has no body to reuse — use --source fresh", tag)
	}
	return body, nil
}
