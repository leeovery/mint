---
status: in-progress
created: 2026-06-08
cycle: 1
phase: Input Review
topic: mint-release-tool
---

# Review Tracking: mint-release-tool - Input Review

## Findings

### 1. Repo-root anchoring behaviour in submodules / worktrees flagged for spec but dropped

**Source**: `discussion/mint-release-tool.md` — "Safety & preflight gates" › Notes / deferred (Preflight), line 192
**Category**: Gap/Ambiguity
**Affects**: Stage 2 — Preflight & Safety Gates (gate #1, "Git repo present, anchored at the repo root"); secondarily Config Format & Schema › Format & location (root resolution)

**Details**:
The discussion explicitly parked one item *for the spec to resolve*: "Repo-root anchoring with the global-binary + shim model (where mint sets its working dir; behaviour in submodules/worktrees) is an implementation detail flagged for spec, not re-litigated here." The specification resolves root via `git rev-parse --show-toplevel` and states "mint runs from root" / "runs from root" (Stage 2 gate #1 and the config location section), but it never addresses the flagged edge: what `--show-toplevel` resolves to when mint is invoked from inside a git submodule or a linked worktree, and where mint then sets its working directory. This was a conscious "carry into the spec" item, not a closed decision, and it fell out. It matters because the global-binary-plus-shim model means mint can be launched from an arbitrary subdirectory, and submodule/worktree resolution changes which `.mint.toml` and which tag set mint sees.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---

### 2. Gitignored-but-force-added file is a deliberate non-special-cased edge — acknowledgment dropped

**Source**: `discussion/mint-release-tool.md` — "AI release notes — quality" › Diff-exclude globs, line 665
**Category**: Enhancement to existing topic
**Affects**: Stage 4 — AI Release Notes › Diff exclusion (the `diff_exclude` / `.gitignore`-inherent paragraph)

**Details**:
The discussion closes the diff-exclusion reasoning with an explicit edge-case ruling: a release diff is commit-to-commit so gitignored files never appear — *"Edge case (gitignored-but-force-added) is deliberate and not special-cased."* The spec captures the first half ("A release diff is commit-to-commit so it can only contain tracked files; gitignored files never appear") but drops the explicit acknowledgment that a file which is gitignored yet force-added (so it *is* tracked) will legitimately appear in the diff, and that this is a conscious non-special-cased decision rather than an oversight. Recording it prevents a future reader from treating it as a missed case.

**Current**:
`diff_exclude` (project artifacts) — configurable array of globs, on top of the above (knowledge bundle, minified output, lockfiles, generated code). These are *tracked, committed* generated files (deliberately not in `.gitignore`), which is why explicit exclusion is needed. A release diff is commit-to-commit so it can only contain tracked files; gitignored files never appear. Kept in config (not a `.mintignore` file) per the "one config, one place to look" principle; `.mintignore` is YAGNI, addable later if exclude sets grow large.

**Proposed Addition**:
{leave blank until discussed}

**Resolution**: Pending
**Notes**:

---
