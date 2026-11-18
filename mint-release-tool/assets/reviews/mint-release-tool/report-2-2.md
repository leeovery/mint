TASK: mint-release-tool-2-2 — Diff context assembly (last_tag..HEAD, CHANGELOG.md always-excluded)

STATUS: Complete

IMPLEMENTATION:
- Status: Implemented (and since extended by later phases in same file). internal/notes/assemble.go: AssembleDiff, forwardRange, AssembleRange, changelogExcludePathspec, excludePathspecs. argv `git diff {lastTag}..HEAD -- . :(exclude)CHANGELOG.md` via injected runner; returns git stdout verbatim (no Go stripping); empty diff not an error; missing git → ErrCommandNotFound via errors.Is, other non-zero wrapped. Phase-2 baseline reproduced exactly by zero ExcludeConfig.

TESTS:
- Status: Adequate. Covers DiffsLastTagToHEAD, ExcludesChangelogViaPathspec (byte-exact argv), ChangelogOnlyChange→empty, ForceAddedGitignoredFile passthrough, ReturnsPostExclusionDiffText, AbsentDiffExclude baseline, CommandNotFound + GitFails classification. argv assertion proves git (not Go) filters.

CODE QUALITY:
- Followed conventions (runner seam, FakeRunner, %w, errors.Is, behavioural tests, t.Parallel, doc comments). SOLID good — single assembly responsibility, DI. Low complexity, modern idioms, verbose-but-accurate comments.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/notes/assemble.go:112 — comment says make([]string, 0, 2+len(a.exclude.Globs)) capacity covers the tiers but the adjacent comment block (101-110) never states the cap is deliberate +1 headroom (CHANGELOG + worst-case version_file). Add a half-sentence so a future reader doesn't "tighten" it to 1+len(...) and clip the plain-mode entry.
