---
status: in-progress
created: 2026-06-09
cycle: 2
phase: Gap Analysis
topic: commit-command
---

# Review Tracking: commit-command - Gap Analysis

## Findings

### 1. Editor resolution for the `e` (edit) gate action is unspecified

**Source**: Specification analysis
**Category**: Gap/Ambiguity
**Affects**: Interactive Review Gate (`e` choice); `$EDITOR` Fallback — Path Semantics (Editor resolution)

**Details**:
The spec defines editor resolution precisely, but only for the *fallback* path. The "Editor resolution" subsection lives under `$EDITOR` Fallback — Path Semantics and is framed fallback-specifically: it says mint uses git's resolution order (`GIT_EDITOR` → `core.editor` → `$VISUAL` → `$EDITOR` → git's built-in default) "so the fallback opens whatever `git commit` would open," and justifies mint opening the editor itself with a fallback-specific reason ("because staging must be deferred until the save-as-accept event").

The `e` / edit gate action is a **separate path** (loop-back, explicitly *not* save-as-accept). It says only "open the message in `$EDITOR` pre-filled." Two things are left open for `e`:

1. **Which editor `e` launches.** Does `e` reuse the same git resolution chain as the fallback, or launch the raw `$EDITOR` environment variable? These diverge on real machines (unset `$EDITOR` but `core.editor` set; or `GIT_EDITOR`/`$VISUAL` set). An implementer must guess, and could reasonably build `e` differently from the fallback — producing inconsistent editor behaviour between two paths the user perceives as "the same editor."
2. **What `e` does when no editor resolves to a launchable program.** On the fallback path this is a defined fail-loud case. On the `e` path the message already exists, so the alternatives are plausible and unstated: fail loud, or quietly re-render the gate with the unedited message (treat `e` as a no-op). The spec's "`e` can never produce an empty commit / is a refinement step" reasoning suggests a graceful degrade, but it is not stated.

This is a clarity/consistency gap rather than a missing decision — both editor paths almost certainly want the same resolution chain — but as written an implementer has to decide it, and could get it wrong.

**Proposed Addition**:
[leave blank until discussed]

**Resolution**: Pending
**Notes**:

---
