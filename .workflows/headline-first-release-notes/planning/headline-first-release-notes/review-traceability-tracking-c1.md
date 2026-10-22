# Review Tracking: Headline First Release Notes - Traceability

## Findings

### 1. Acceptance and hold-check runs don't pin E1's model

**Type**: Missing from plan
**Spec Reference**: §10.2 Tuning and acceptance (acceptance: "The same model is E1's, `claude-sonnet-5-5` … Where the alias resolves to another model when a run is made, that run names E1's model in the alias's place and is otherwise the shipped command"; and every hold-check run "uses the acceptance pass's AI command and model"); Corrigendum 2026-10-22 (the `sonnet` alias)
**Plan Reference**: Phase 3 Acceptance; headline-first-release-notes-3-10 (Acceptance Criteria, Do); the AI-command line in the Do of headline-first-release-notes-3-1, 3-2, 3-3, 3-4 and 3-9
**Move**: settled
**Change Type**: update-task

**Problem**:
Whether the rewritten prompt ships rests on the acceptance pass and the hold checks, and as planned they run on whatever model the `sonnet` alias points to on the day they run. The bar is fixed at counts measured on `claude-sonnet-5-5`. If the alias has moved to another model by then, the pass compares that model's notes against a bar set on a different one, so a pass or a fail tells Lee nothing about the wording. Nobody would notice, because the counts look like those of any other run. The plan only says to use mint's shipped default `ai_command` "as E1 ran it", and E1's harness hard-codes `config.DefaultAICommand` (`--model sonnet`) at `harness/main.go:87` and `:158`. Following the plan's Do lines therefore picks up the alias. The chunked-path and nothing-users-meet checks run through mint's own default command, so they inherit the same drift.

**Proposal**:
Carry §10.2's model rule, which its corrigendum settled. The acceptance pass and every run that checks a failure §10.3 says holds shipping, first runs and frozen-wording re-runs alike, use mint's shipped default `ai_command` on E1's model, `claude-sonnet-5-5`. Where the command's `sonnet` alias resolves to another model when a run is made, that run names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command. The fix goes in three places: one rule-form criterion in Phase 3's Acceptance, task 3-10's harness criterion and its AI-command Do line, and the AI-command Do line every hold-check task shares. The record admits only this one answer: §10.1's bar column is fixed counts measured on that model, so only a run on the same model can be compared with it.

**Current**:

*Phase 3 Acceptance (planning.md), last criterion:*

- [ ] With the wording frozen, every mint gate passes, and mint's test suite still spawns no real AI command: the acceptance pass and the re-run checks run outside it (§10.2).

*headline-first-release-notes-3-10, Acceptance Criteria, first criterion:*

- [ ] Given the frozen wording, the rewritten default prompt closed by §7.1's reminder, when the acceptance pass runs it once over E1's 26 held-out releases with two repeats each, it does so with E1's harness, the same inputs, and the same AI command and model, and yields 52 bodies (§10.2)

*headline-first-release-notes-3-10, Do, second bullet:*

- The AI command is mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it (§10.2).

*headline-first-release-notes-3-1, Do, opening sentence of the third bullet (the rest of the bullet stays as it is):*

- A run that checks a failure §10.3 says holds shipping, this task's maintenance-line check included, uses mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own.

*headline-first-release-notes-3-2, headline-first-release-notes-3-3 and headline-first-release-notes-3-4, Do, opening sentence of the AI-command bullet in each (third bullet in 3-2 and 3-3, second in 3-4; the rest of each bullet stays as it is):*

- These checks use mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own.

*headline-first-release-notes-3-9, Do, opening sentence of the second bullet (the rest of the bullet stays as it is):*

- Every re-run check uses mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own.

**Proposed Text**:

*Phase 3 Acceptance (planning.md), replacing the last criterion with these two:*

- [ ] The acceptance pass, and every run that checks a failure §10.3 says holds shipping, first runs and re-runs on the frozen wording alike, use mint's shipped default `ai_command` on E1's model, `claude-sonnet-5-5`, never a release repo's own command. Where the command's `sonnet` alias resolves to another model when a run is made, that run names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command, so its results stay comparable with §10.1's fixed bar (§10.2).
- [ ] With the wording frozen, every mint gate passes, and mint's test suite still spawns no real AI command: the acceptance pass and the re-run checks run outside it (§10.2).

*headline-first-release-notes-3-10, Acceptance Criteria, first criterion:*

- [ ] Given the frozen wording, the rewritten default prompt closed by §7.1's reminder, when the acceptance pass runs it once over E1's 26 held-out releases with two repeats each, it does so with E1's harness, the same inputs, and mint's shipped default `ai_command` on E1's model, `claude-sonnet-5-5`, and yields 52 bodies. Where the command's `sonnet` alias resolves to another model when the pass runs, the pass names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command (§10.2)

*headline-first-release-notes-3-10, Do, second bullet:*

- The AI command is mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, on E1's model, `claude-sonnet-5-5`, which the command's `sonnet` alias resolved to when E1 ran (E1's `report.md`: the `modelUsage` key of a `--output-format json` call). E1's harness builds its calls on `config.DefaultAICommand` itself. Where the alias resolves to another model when the pass runs, the pass names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command (§10.2).

*headline-first-release-notes-3-1, Do, opening sentence of the third bullet:*

- A run that checks a failure §10.3 says holds shipping, this task's maintenance-line check included, uses mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own, on E1's model, `claude-sonnet-5-5`. Where the command's `sonnet` alias resolves to another model when the run is made, the run names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command.

*headline-first-release-notes-3-2, headline-first-release-notes-3-3 and headline-first-release-notes-3-4, Do, opening sentence of the AI-command bullet in each:*

- These checks use mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own, on E1's model, `claude-sonnet-5-5`. Where the command's `sonnet` alias resolves to another model when a run is made, the run names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command.

*headline-first-release-notes-3-9, Do, opening sentence of the second bullet:*

- Every re-run check uses mint's shipped default `ai_command` (`config.DefaultAICommand`, `internal/config/config.go`), as E1 ran it, never a release repo's own, on E1's model, `claude-sonnet-5-5`. Where the command's `sonnet` alias resolves to another model when a run is made, the run names `claude-sonnet-5-5` in the alias's place and is otherwise the shipped command.

**Resolution**: Pending
**Notes**:

---

## Observations

- Tasks 3-5, 3-6 and 3-7 each pass the wording on to the freeze only after tuning runs show its count "as §10.1 counts them". §10.2 leaves open which of E1's releases tuning runs on (the 5 `tune` rows or the 26 held out). Either way, the acceptance pass on the frozen wording is the gate that decides.
- Phase 3's Acceptance does not restate the known-weak-spot deferrals that tasks 1-3, 1-5 and 1-6 record. Task 3-7 owns them, none of them holds shipping, and the frozen-wording criterion already proves the rules are unchanged.
- The "salience needs" comment at `internal/engine/regenerate_fresh.go:140` falls outside §6.2's `internal/notes` scope and does not describe the Change Map, so nothing there needs to change.
