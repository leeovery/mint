package notes

import (
	"context"

	"mint/internal/config"
	"mint/internal/record"
	"mint/internal/runner"
)

// Kind names which notes path the precedence resolver took. It is reported back to
// the caller so downstream stages can branch on the path WITHOUT re-deriving it —
// notably the interactive review gate, which offers regenerate (r) ONLY for
// KindNormalAI (the one path where the AI actually produced the body, so
// regenerating is meaningful), and run reporting, which describes how the body was
// produced.
type Kind int

const (
	// KindFirstRelease: no prior tag, fixed "Initial release." body, no AI.
	KindFirstRelease Kind = iota
	// KindDegenerate: prior tag but an empty/whitespace post-exclusion diff, StubBody, no AI.
	KindDegenerate
	// KindNoAI: deliberate --no-ai skip, fallback body, no AI, never aborts.
	KindNoAI
	// KindNormalAI: the AI produced the body. The only path the review gate offers r for.
	KindNormalAI
	// KindFallback: the normal AI path failed and on_notes_failure=fallback produced the body.
	KindFallback
)

// SelectState describes the run inputs the precedence needs: whether this is a
// FIRST RELEASE (no prior tag — the strongest guard), the LastTag the diff base
// and git-log fallbacks range from, and whether --no-ai was set. FirstRelease and
// LastTag are complementary: when FirstRelease is true there is no diff base and
// LastTag is unused; otherwise LastTag names the prior tag every git range
// (`{LastTag}..HEAD`) is computed against.
type SelectState struct {
	// FirstRelease is true when there is no prior tag. It wins over everything: mint
	// records FirstReleaseBody and never assembles a diff, runs the degenerate check,
	// consults --no-ai, or reaches on_notes_failure.
	FirstRelease bool

	// LastTag is the prior tag the diff and commit-subject fallback range from. It is
	// only meaningful when FirstRelease is false.
	LastTag string

	// NoAI is the --no-ai flag: a deliberate skip of the AI path (after first-release
	// and degenerate). It never aborts.
	NoAI bool
}

// Selector is the SINGLE notes-path precedence decision point. It holds the
// composed providers — the normal-path Generator, the Assembler (for the
// degenerate-check diff), the runner (for the no-AI and fallback bodies), and the
// repo root — and COMPOSES them; it reimplements none of them. The interactive
// review gate and the orchestrator invoke SelectBody as the one place that decides
// which body a release gets.
type Selector struct {
	generator *Generator
	assembler *Assembler
	runner    runner.CommandRunner
	root      string
}

// NewSelector builds a Selector over the composed providers. The same Assembler
// the Generator wraps is passed in directly so the degenerate-check diff and the
// AI path share one git seam; the runner is the same seam, used for the no-AI and
// fallback commit-subject bodies.
func NewSelector(generator *Generator, assembler *Assembler, r runner.CommandRunner, root string) *Selector {
	return &Selector{generator: generator, assembler: assembler, runner: r, root: root}
}

// SelectBody resolves the notes body via the STRICT precedence (each branch
// returns immediately; on_notes_failure is consulted ONLY in branch 4):
//
//  1. FIRST RELEASE (no prior tag) → record.FirstReleaseBody, KindFirstRelease.
//     Wins over everything: no diff is assembled, no degenerate check, no --no-ai,
//     no on_notes_failure.
//  2. DEGENERATE (prior tag, empty/whitespace post-exclusion diff) → StubBody,
//     KindDegenerate, no AI. Wins over --no-ai.
//  3. --no-ai → NoAIBody, KindNoAI, no AI, never aborts.
//  4. NORMAL AI PATH → Generator over the already-assembled diff. On success →
//     body, KindNormalAI. On failure → ResolveFailure: an abort error propagates
//     (no body, KindNormalAI), a fallback body returns as KindFallback.
//
// The diff is assembled ONCE (branch 2) and reused by branch 4 via
// GenerateFromDiff, so branches 2-4 share a single AssembleDiff call.
func (s *Selector) SelectBody(ctx context.Context, state SelectState, cfg config.Config) (string, Kind, error) {
	if state.FirstRelease {
		return record.FirstReleaseBody, KindFirstRelease, nil
	}

	diff, err := s.assembler.AssembleDiff(ctx, state.LastTag)
	if err != nil {
		return "", KindNormalAI, err
	}

	if IsDegenerate(diff) {
		return StubBody(), KindDegenerate, nil
	}

	if state.NoAI {
		body, err := NoAIBody(ctx, s.runner, state.LastTag, cfg.Release)
		if err != nil {
			return "", KindNoAI, err
		}
		return body, KindNoAI, nil
	}

	body, err := s.generator.GenerateFromDiff(ctx, state.LastTag, diff, cfg)
	if err != nil {
		return s.resolveNormalPathFailure(ctx, err, state.LastTag, cfg.Release)
	}
	return body, KindNormalAI, nil
}

// resolveNormalPathFailure routes a branch-4 generator failure through the
// on_notes_failure policy: an abort error propagates as KindNormalAI (the AI path
// was taken, then aborted — no body), while a fallback body returns as
// KindFallback (the AI failed but on_notes_failure=fallback produced a body).
func (s *Selector) resolveNormalPathFailure(ctx context.Context, failure error, lastTag string, rel config.Release) (string, Kind, error) {
	body, err := ResolveFailure(ctx, s.runner, failure, lastTag, rel)
	if err != nil {
		return "", KindNormalAI, err
	}
	return body, KindFallback, nil
}
