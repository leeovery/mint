AGENT: architecture
FINDINGS:
- FINDING: RunLocalGates now carries two adjacent boolean parameters
  SEVERITY: low
  FILES: internal/preflight/preflight.go:81, internal/engine/release.go:413, internal/engine/release.go:1106
  DESCRIPTION: RunLocalGates(ctx, r, releaseBranch, tag, anyBranch, skipCleanTree bool) reaches six
    parameters with two trailing booleans, and runPreflight mirrors the same pair. code-quality.md
    explicitly lists "Boolean parameters" and "Long parameter lists (4+)" as anti-patterns, and the
    two adjacent bools create a positional call-site readability hazard — e.g.
    RunLocalGates(t.Context(), r, "main", "v1.2.3", false, true) requires the reader to remember which
    flag is which (false=anyBranch, true=skipCleanTree). This is NOT a defect this work unit invented
    in isolation: anyBranch was already a bool param, and the spec deliberately mandates mirroring it
    exactly for consistency. So the new bool is a faithful, locally-correct extension of a pre-existing
    pattern, and changing it would mean reworking the established anyBranch shape rather than this
    quick-fix. Noted only as the single composition observation — the trend of accumulating gate-skip
    booleans is the thing to watch if a third skip is ever added.
  RECOMMENDATION: No change required for this quick-fix — the mirror-anyBranch decision is the right
    call here and the bool is concrete and tested at both levels. If a future change adds a third
    gate-skip flag, consider collapsing the skip flags into a single typed gate-skip set (e.g. a small
    options struct or bit set) so the call sites stop relying on positional booleans; until then the
    pattern stays as-is for consistency.
SUMMARY: The implementation is a clean, surgical, spec-faithful change: the dry-run+autostash bypass
  predicate is computed once at the single Release call site and threaded cleanly through
  runPreflight to RunLocalGates with no duplicated logic, no new types or flags, and coverage at both
  the engine end-to-end seam (zero stash mutations + bypassed porcelain probe + completed preview,
  plus the negative non-autostash-still-aborts guard) and the preflight unit level. The only
  observation is the now-paired gate-skip booleans on RunLocalGates, which deliberately mirror the
  existing anyBranch parameter per the spec and are low severity.
