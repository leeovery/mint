# Single-version regenerate prepends changelog entry at top instead of semver position

Observed in the tick project (github.com/leeovery/tick) on 2026-07-12 while backfilling release notes with mint.

A full `mint release regenerate --all --target both` ran first and built CHANGELOG.md from scratch correctly — all versions in proper order, oldest to newest. One version (v0.2.1) failed during that run because its diff was oversized, so it ended up absent from the changelog. It was then regenerated individually with `mint release regenerate 0.2.1 --target both -y`.

The GitHub release body for v0.2.1 was written correctly. The changelog write, however, prepended the new `## [0.2.1]` section at the very top of CHANGELOG.md — directly under the file's intro header, above the existing `## [0.2.8]` entry — rather than inserting it in semver order between `## [0.2.2]` and `## [0.2.0]` where it belongs.

The distinguishing condition appears to be that CHANGELOG.md had no existing `[0.2.1]` section at the time of the run (the backfill case): the version being regenerated was older than every entry already in the file, and the new section landed at the top regardless.

Impact: the changelog silently ends up mis-ordered — the command reports success, so nothing flags that the entry is in the wrong place. Anyone backfilling a single historical version (which is exactly the recovery path after one version fails during an `--all` run) has to notice the misplacement themselves and move the section by hand, then commit the fix separately.

A second, smaller formatting issue was noticed in the same changelog output: versions whose diff is empty get a "Maintenance release — no notable source changes" body, and that line is written with no trailing blank line before the next `## [x.y.z]` heading (observed between `[0.2.4]`/`[0.2.3]` and `[0.0.3]`/`[0.0.2]`). It still renders as a heading under CommonMark, but the spacing is inconsistent with every other entry in the file.
