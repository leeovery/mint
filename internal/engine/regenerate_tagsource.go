package engine

// This file is the regenerate tag SOURCE read (`--source tag`, task 5-5): reading a
// tag's annotation body back as the source mint consumes on the tag-source path.
//
// The forward path writes the annotated tag as `git tag -a … -F -` with the message
// subject `{commit_prefix} Release {tag}`, a blank line, then the FULL notes body
// verbatim (see internal/release). `git for-each-ref --format=%(contents:body)
// refs/tags/<tag>` returns exactly that body part (git splits off the subject and
// the blank separator), so the read is a single deterministic git call and the body
// is used WHOLE — no parsing, no splitting, no validation transform. It was already
// presentation-format when written, so the bytes flow straight to the downstream write.
//
// The tag source runs NO AI and assembles NO diff: it touches only this one read.

import (
	"context"
	"fmt"
	"strings"

	"mint/internal/runner"
)

// ReadTagBody reads tag's annotation body via ONE deterministic git call —
// `git for-each-ref --format=%(contents:body) refs/tags/<tag>` through the runner
// seam — and returns it WHOLE alongside whether the tag carries an annotation body.
//
// hasBody is the single branch point both regenerate modes consume:
//   - SINGLE mode (RequireTagBody) turns hasBody=false into a fail-loud error.
//   - --all mode (task 5-12) skips-and-reports on hasBody=false, never writing an
//     empty provider release body.
//
// "No annotation body" covers BOTH a lightweight tag (no annotation object, so
// for-each-ref emits nothing) AND an annotated tag with an empty or whitespace-only
// body — both surface as an empty/whitespace contents:body, so the detection is a
// single trim-and-check-empty. The returned body is the RAW for-each-ref stdout
// (verbatim, never trimmed); only the hasBody decision trims.
//
// A genuine git failure (missing binary, non-zero exit) is surfaced as an error and
// is NOT masked as "no body".
func ReadTagBody(ctx context.Context, r runner.CommandRunner, tag string) (body string, hasBody bool, err error) {
	res, err := r.Run(ctx, "git", "for-each-ref", "--format=%(contents:body)", "refs/tags/"+tag)
	if err != nil {
		return "", false, fmt.Errorf("reading annotation body for tag %s: %w", tag, err)
	}
	return res.Stdout, strings.TrimSpace(res.Stdout) != "", nil
}

// RequireTagBody is single-mode's fail-loud wrapper around ReadTagBody: it returns
// the annotation body verbatim for the downstream write, or — when the tag has no
// annotation body (lightweight, empty, or whitespace-only) — the EXACT fail-loud
// error the spec pins, so the tag-source path stops rather than writing an empty body.
//
// Single mode does NOT fall back to a fresh re-diff; it directs the user there with
// the `use --source fresh` hint. (Batch --all mode, task 5-12, calls ReadTagBody
// directly and skips-and-reports instead of failing the whole run.)
func RequireTagBody(ctx context.Context, r runner.CommandRunner, tag string) (string, error) {
	body, hasBody, err := ReadTagBody(ctx, r, tag)
	if err != nil {
		return "", err
	}
	if !hasBody {
		return "", fmt.Errorf("tag %s has no annotation body — use --source fresh", tag)
	}
	return body, nil
}
