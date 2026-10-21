TASK: Corrections (token-aware-diff-sizing-9-3, tick-49df08): CLAUDE.md's "Surgical unwind, then PONR" invariant names both ways the exclusion check's answers survive an unwind

ACCEPTANCE CRITERIA:
- CLAUDE.md's "Surgical unwind, then PONR" bullet carries the Solution's replacement sentence verbatim in place of the old exception sentence (§18.3, §6.6)
- The bullet's other sentences and the rest of CLAUDE.md are unchanged, and no code, test or other doc changes; every gate stays green

STATUS: issues_found

SPEC CONTEXT: §6.6 (with its 2026-10-21 corrigendum) has an untracked `.mint.toml` that git ignores receive the answers' write without staging or committing it, and an abort or failure after answering keeps that write, as it keeps an answers commit. §18.3 (spec line 894), as corrected by the 2026-10-21 corrigendum on §18.1-§18.3 (spec line 1008), rewrites CLAUDE.md's "Surgical unwind, then PONR" invariant to name the answers as the one exception. It covers both forms: the commit, re-committed on the starting commit, and the uncommitted write to an ignored untracked `.mint.toml`, which the unwind leaves in place.

IMPLEMENTATION:
- Status: Implemented
- Location: CLAUDE.md:42 (commit 888df23, the only file in that commit, a 1-line replacement; CLAUDE.md has not changed since, per `git diff 888df23 HEAD -- CLAUDE.md`)
- Notes: The new sentence matches the Solution's text character for character. The surrounding sentences ("every pre-push mutation is tracked ... mint removes only what it created", "The point of no return is the single `git push --atomic origin HEAD <tag>`. After it: warn-only, never unwind.") are unchanged, and the diff changes nothing else in the file. The claim holds against the code. internal/engine/release.go:1105-1115 commits only when `record.IsIgnored` returns false and sets `a.edit = nil` on both routes. internal/engine/release.go:557 hands that nil edit to `made.answersEdit`, so the revert at internal/engine/unwind.go:127 has nothing to undo. `record.IsIgnored` (internal/record/commit.go:120-130) runs `git check-ignore -q` without `--no-index`, so a tracked file never reports as ignored. That makes "untracked" in the sentence accurate. The new wording agrees with README.md:30, README.md:353, README.md:117, the setup guide (internal/setupguide/setupguide.go:267-270) and the unwind.go header (internal/engine/unwind.go:11-13).

TESTS:
- Status: Adequate
- Coverage: No test applies to a contract-document wording change. No Go source reads the project's CLAUDE.md: the five Go files that mention "CLAUDE.md" (internal/configtest/configtest.go, internal/config/config.go, internal/config/readme_ai_command_test.go, internal/setupguide/setupguide.go, internal/aitransport/aitransport.go) do so in comments or string literals and never open the file. So no gate can observe this change, and the "every gate stays green" criterion is settled for this task's delta by reading.
- Notes: None

CODE QUALITY:
- Project conventions: Followed. The edit applies CLAUDE.md's own rule that comments stay true to the as-built code.
- SOLID principles: N/A (documentation only)
- Complexity: Low
- Modern idioms: N/A
- Readability: Good
- Issues: The same stale claim the task removed from CLAUDE.md survives in the release.go spine header (see FINDINGS).

BLOCKING ISSUES:
- None

FINDINGS:
- [in-scope] [contained] internal/engine/release.go:22-23 — The AUTO-UNWIND header says the unwind "re-commits the exclusion answers on the starting commit when their commit had landed, the one thing it keeps". Task 8-9's ignored route falsified this, and the header was written in this feature (e0e2e63, task 8-5). The unwind also keeps the answers' uncommitted write to an untracked `.mint.toml` that git ignores: release.go:1115 clears `a.edit`, so `made.answersEdit` is nil and unwind.go:127 reverts nothing. Fix by replacing "then re-commits the exclusion answers on the starting commit when their commit had landed, the one thing it keeps." with "then re-commits the exclusion answers on the starting commit when their commit had landed, and leaves their uncommitted write to a .mint.toml git ignores in place: the answers are the one thing it keeps." This is comment text only and does not block. — FAILS: a maintainer working in the release spine reads the header's "the one thing it keeps" and takes the surviving ignored-file write for a leak. Adding it to the unwind brings back the dead end the §6.6 corrigendum closed: a repo that ignores `.mint.toml` is asked the same questions again after every abort. This is the same failure task 9-3 names for CLAUDE.md, at the comment that sits on the code itself.

UNSETTLED:
- None
