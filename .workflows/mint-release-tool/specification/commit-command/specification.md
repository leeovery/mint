# Specification: Commit Command

## Specification

## Overview

`mint commit` is a sibling verb to `mint release` that produces an AI-generated commit message from the diff, built into the `mint` binary. It is a **thin standalone verb**: it does NOT ride the release lifecycle spine. Its core act is small and local: stage (optionally) → generate a message → optionally review → commit → optionally push.

The guiding design stance: **there is no code yet.** The shared AI machinery is designed up front to serve both verbs cleanly — `commit` is not retrofitted onto release-note generation, nor does it require reworking any settled release decision.

## Scope

**What `commit` reuses (shared primitives):**
- The `Presenter` seam — pretty/plain rendering, `-y` orthogonality, `--plain`, the `Continue?` review-gate rendering (defined in `cli-presentation`).
- The AI engine — transport `claude -p`, mint-owned prompt, fail-loud + retry, `--no-ai` skip (the three-layer engine, below).
- `git_safe` lock-resilient git.
- The TOML config model (verb-namespaced shape, below).
- `diff_exclude` globs and the `max_diff_lines` guard apply to commit's diff exactly as they apply to release's — we don't feed excluded files (bundles, lockfiles, minified output) into message generation.

**What `commit` does NOT touch:**
- Version detection, tags, changelog, publish/provider, the regenerate command.
- No point-of-no-return / atomic-push semantics — a commit is inherently local and reversible until pushed.

**Safety posture:** the inverse of release's. Release forces a known-good, clean, in-sync starting state because it is high-consequence; commit assumes a messy, in-progress working tree because operating on one is its entire reason to exist.

---

## Working Notes

[Optional - capture in-progress discussion if needed]
