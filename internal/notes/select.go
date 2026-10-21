package notes

import (
	"context"
	"fmt"

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
	// KindSupplied: the operator supplied the finished body (--notes-file), no AI.
	KindSupplied
	// KindFloor: the normal AI path failed and the operator chose the body at the
	// notes floor's choice.
	KindFloor
)

// SelectState describes the run inputs the precedence needs: any supplied body,
// whether this is a FIRST RELEASE (no prior tag), the LastTag the diff base
// and git-log fallbacks range from, and whether --no-ai was set. FirstRelease and
// LastTag are complementary: when FirstRelease is true there is no diff base and
// LastTag is unused; otherwise LastTag names the prior tag every git range
// (`{LastTag}..HEAD`) is computed against.
type SelectState struct {
	// Supplied is the finished body the operator handed mint, nil when none was. It
	// wins over everything, first release included: no diff is assembled and no AI,
	// cache or on_notes_failure is consulted.
	Supplied *string

	// FirstRelease is true when there is no prior tag. It wins over everything but a
	// supplied body: mint records FirstReleaseBody and never assembles a diff, runs the
	// degenerate check, consults --no-ai, or reaches on_notes_failure.
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
	generator      *Generator
	assembler      *Assembler
	runner         runner.CommandRunner
	root           string
	fallbackNotice func(failure error)
	check          *ExclusionCheck
}

// NewSelector builds a Selector over the composed providers. The same Assembler
// the Generator wraps is passed in directly so the degenerate-check diff and the
// AI path share one git seam; the runner is the same seam, used for the no-AI and
// fallback commit-subject bodies.
func NewSelector(generator *Generator, assembler *Assembler, r runner.CommandRunner, root string) *Selector {
	return &Selector{generator: generator, assembler: assembler, runner: r, root: root}
}

// WithFallbackNotice returns a copy of s that hands notice the failure each
// KindFallback body ships in place of; s itself is unchanged.
func (s *Selector) WithFallbackNotice(notice func(failure error)) *Selector {
	c := *s
	c.fallbackNotice = notice
	return &c
}

// WithExclusionCheck returns a copy of s whose AI path makes check's call first, before
// cache reuse and the notes; s itself is unchanged.
func (s *Selector) WithExclusionCheck(check *ExclusionCheck) *Selector {
	c := *s
	c.check = check
	return &c
}

// CacheInputs carries the dry-run note-cache key components the NORMAL-AI path
// produced: the post-diff_exclude diff fed to the AI and the resolved
// prompt/context instructions. Together with the caller's computed version these
// are exactly the three fields the cache key hashes (notescache.Key). Cacheable is
// true ONLY for the normal-AI path — the one path that produced a stochastic AI
// body worth caching; every other Kind leaves it false and the dry-run write is
// skipped (nothing to cache).
type CacheInputs struct {
	// Cacheable is true only when an AI body was produced (KindNormalAI). When false
	// Diff and Instructions are zero and the caller must not write a cache entry.
	Cacheable bool
	// Diff is the post-diff_exclude diff handed to the AI — the SAME filtered diff
	// the prompt carried, NOT the HEAD sha. It is a cache-key component.
	Diff string
	// Instructions is the resolved prompt/context (the [release].prompt override
	// contents OR the default prompt plus the injected [release].context). A prompt
	// or context change changes it, so the cache key correctly invalidates.
	Instructions string
}

// ReuseFunc is the real-run note-cache reuse hook (task 4-8) the selector consults
// on the NORMAL-AI path AFTER the cache-key inputs (the post-diff_exclude diff and
// the resolved prompt/context instructions) are known but BEFORE the notes call.
// Given those inputs it returns a cached body to reuse INSTEAD of generating, with
// reused=true; reused=false means no live cache match (regenerate via the AI). A
// non-nil error aborts. The selector knows nothing of the cache, the version, or the
// clock — the engine closes over those and the selector merely offers the pre-AI
// interception point so the precedence (and the single diff/instructions resolution)
// is not duplicated. Returning false is byte-identical to having no hook at all.
type ReuseFunc func(diff, instructions string) (body string, reused bool, err error)

// SelectBody resolves the notes body via the STRICT precedence (each branch
// returns immediately; on_notes_failure is consulted ONLY in branch 4):
//
//  0. SUPPLIED (--notes-file) → the supplied body verbatim, KindSupplied. Wins over
//     everything, first release included.
//  1. FIRST RELEASE (no prior tag) → record.FirstReleaseBody, KindFirstRelease.
//     Wins over everything below: no diff is assembled, no degenerate check, no
//     --no-ai, no on_notes_failure.
//  2. DEGENERATE (prior tag, empty/whitespace post-exclusion diff) → StubBody,
//     KindDegenerate, no AI. Wins over --no-ai.
//  3. --no-ai → NoAIBody, KindNoAI, no AI, never aborts.
//  4. NORMAL AI PATH → the exclusion check when one is wired, then the reuse hook,
//     then the Generator over the already-assembled diff. Exclude answers at the
//     check re-assemble the diff without their globs; one they leave empty gives
//     StubBody, KindDegenerate, no AI. On success → body, KindNormalAI. On failure →
//     ResolveFailure: an abort error propagates (no body, KindNormalAI), a fallback
//     body returns as KindFallback.
//
// The diff is assembled once (branch 2), and again only after Exclude answers, and
// reused by branch 4 via GenerateFromDiff.
func (s *Selector) SelectBody(ctx context.Context, state SelectState, cfg config.Config) (string, Kind, error) {
	body, kind, _, err := s.SelectBodyWithCacheInputs(ctx, state, cfg)
	return body, kind, err
}

// SelectBodyWithCacheInputs is SelectBody plus the dry-run note-cache key inputs.
// It runs the identical precedence and additionally surfaces, for the NORMAL-AI
// path only, the post-diff_exclude diff and the resolved prompt/context
// instructions (CacheInputs) so the caller can compute the cache key WITHOUT
// re-assembling the diff or re-resolving the prompt. Every non-AI path returns a
// zero, non-Cacheable CacheInputs (there is no AI body to cache). SelectBody is the
// thin wrapper that discards the inputs, so its behaviour is byte-identical. It is
// SelectBodyWithReuse with NO reuse hook (always generate) — the dry-run WRITE path,
// which generates the preview to cache.
func (s *Selector) SelectBodyWithCacheInputs(ctx context.Context, state SelectState, cfg config.Config) (string, Kind, CacheInputs, error) {
	return s.SelectBodyWithReuse(ctx, state, cfg, nil)
}

// SelectBodyWithReuse runs the identical precedence as SelectBodyWithCacheInputs and,
// on the NORMAL-AI path ONLY, consults the optional reuse hook AFTER the cache-key
// inputs (the post-diff_exclude diff + resolved prompt/context instructions) are
// resolved but BEFORE the notes call. When the hook reports a live cache match the
// cached body is returned as KindNormalAI and no notes call is made; otherwise (a
// miss, or a nil hook) the AI generates as before. Either way the returned CacheInputs
// carry the resolved diff + instructions, so a reused body still surfaces a coherent
// (Cacheable) key — the real run does not re-write the cache, so this is informational.
//
// A nil reuse hook makes this byte-identical to the always-generate path, so the
// dry-run write path passes nil and gets unchanged behaviour.
func (s *Selector) SelectBodyWithReuse(ctx context.Context, state SelectState, cfg config.Config, reuse ReuseFunc) (string, Kind, CacheInputs, error) {
	if state.Supplied != nil {
		return *state.Supplied, KindSupplied, CacheInputs{}, nil
	}

	if state.FirstRelease {
		return record.FirstReleaseBody, KindFirstRelease, CacheInputs{}, nil
	}

	diff, err := s.assembler.AssembleDiff(ctx, state.LastTag)
	if err != nil {
		return "", KindNormalAI, CacheInputs{}, err
	}

	if IsDegenerate(diff) {
		return StubBody(), KindDegenerate, CacheInputs{}, nil
	}

	if state.NoAI {
		body, err := NoAIBody(ctx, s.runner, state.LastTag)
		if err != nil {
			return "", KindNoAI, CacheInputs{}, err
		}
		return body, KindNoAI, CacheInputs{}, nil
	}

	instructions, err := ResolveInstructions(s.root, cfg.Release)
	if err != nil {
		return "", KindNormalAI, CacheInputs{}, fmt.Errorf("resolving notes instructions for cache key: %w", err)
	}

	diff, err = s.checkExclusions(ctx, state.LastTag, diff, cfg.MaxPromptBytes)
	if err != nil {
		return "", KindNormalAI, CacheInputs{}, err
	}
	if IsDegenerate(diff) {
		return StubBody(), KindDegenerate, CacheInputs{}, nil
	}

	cacheInputs := CacheInputs{Cacheable: true, Diff: diff, Instructions: instructions}

	// Real-run reuse: with a live cache match the cached body is reused and no notes call
	// is made. The hook is consulted ONLY here (the one cacheable path) and
	// only when wired (the dry-run write path passes nil).
	if reuse != nil {
		body, reused, err := reuse(diff, instructions)
		if err != nil {
			return "", KindNormalAI, CacheInputs{}, err
		}
		if reused {
			return body, KindNormalAI, cacheInputs, nil
		}
	}

	body, err := s.generator.GenerateFromDiff(ctx, state.LastTag, diff, cfg)
	if err != nil {
		body, kind, ferr := s.resolveNormalPathFailure(ctx, err, state.LastTag, cfg.Release)
		return body, kind, CacheInputs{}, ferr
	}
	return body, KindNormalAI, cacheInputs, nil
}

// checkExclusions runs the exclusion check over diff when one is wired and returns the
// diff its Exclude answers leave. The answered globs join the shared Assembler's
// excludes, so the Change Map and every later generation, an `r` included, leave them
// out too.
func (s *Selector) checkExclusions(ctx context.Context, lastTag, diff string, maxPromptBytes int) (string, error) {
	if s.check == nil {
		return diff, nil
	}
	excluded, err := s.check.Check(ctx, diff, maxPromptBytes)
	if err != nil || len(excluded) == 0 {
		return diff, err
	}
	s.assembler.Exclude(excluded...)
	return s.assembler.AssembleDiff(ctx, lastTag)
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
	if s.fallbackNotice != nil {
		s.fallbackNotice(failure)
	}
	return body, KindFallback, nil
}
