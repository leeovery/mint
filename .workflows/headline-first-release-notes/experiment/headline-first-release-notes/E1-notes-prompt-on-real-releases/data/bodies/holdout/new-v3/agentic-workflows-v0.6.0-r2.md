Workflow state, status displays and menus are now computed by a deterministic engine instead of being re-derived by Claude, so dashboards render identically every time and state changes are validated and committed atomically.

## 💥 Breaking

- `/workflow-migrate` is no longer a skill and migrations now run as part of `/workflow-start` — stop invoking it directly, as the manifest CLI is also replaced by `engine manifest`.
- The `workflow-manifest` skill and its `init`, `init-phase`, `create-discovery-topic` and `project` commands are removed — use `engine workunit create`, `engine topic start` and `engine discovery-map add`, and read project data with `engine manifest get project.…`.
- Discovery map items no longer carry a `status` field, and writing one is refused — lifecycle is computed at render time, and a new migration strips the old field from existing manifests.
- `engine manifest set` now rejects any value that is invalid for a typed field, including numbers, booleans and `~`, where it previously let them through.

## ✨ Added

- Every dashboard, menu and tree is rendered by the engine and emitted verbatim, so the epic dashboard, start overview, inbox, manage and completed views no longer vary between sessions or break their layout on long summaries.
- Knowledge base setup now happens conversationally from `/workflow-start`, with non-interactive `knowledge setup --from-system`, `--keyword-only` and `--provider …` forms.
- `knowledge setup --key-only` stores the API key at a hidden terminal prompt, and keys can no longer be passed as a flag or through chat.
- The Discussion Map is now tracked in the manifest with a new `deferred` state, and is rendered on demand instead of living inside the discussion file.
- Off-topic concerns raised in a discussion or research session are rerouted into the target topic's Triage section, including a pivot-to-epic option for single-topic work.
- Marking a discovery topic as handled, and later unhandling it, is now supported.
- A `finalise` action completes a work unit whose phases are all done.
- Discovery briefs and map edits now record per-topic provenance, and re-adding a dismissed topic requires explicit confirmation.
- The knowledge base indexes the promoted inbox seeds and epic discovery session logs.
- A `docs/` folder covers every phase, the engine, the knowledge base and the agents.

## 🔧 Changed

- Epic, feature, bugfix, quick-fix and cross-cutting navigation share one engine-driven proceed/revisit flow, with revisit options computed from pipeline state.
- Moving a work unit between states, such as complete, cancel, reactivate, pivot, absorb, promote and topic cancel or reactivate, is now a single committed transaction that is validated before anything changes.
- Implementation tracking, including task start, fix attempts, completion and analysis cycles, is recorded by engine commands, and approval menus are rendered by the engine.
- Background review and perspective agents now write their results over a skeleton file, so in-flight agents are visible and can be waited on at conclusion.
- Task executor reports are shorter.
- Several agents gained or lost tool access, including Linear access for planning agents and git-write bans on review agents.
- Reopening a completed phase now goes through a dedicated transition rather than a manual status edit.
- Knowledge base query results strip terminal control characters.
- Migrations run through a single Node orchestrator that supports both `.sh` and `.cjs` migrations.

## 🐛 Fixed

- Spec-change detection no longer re-baselines the plan on every resume, so unreconciled spec changes keep being reported until the plan concludes.
- Quick-fix review no longer reports missing acceptance criteria as a finding, because quick-fix tasks are verified against their own completeness checks.
- Concluding a discussion or research session can no longer loop forever on a fresh gap review when nothing new was written since the last one.
- Restarting a discussion, research or specification session now also clears stale agent results and resets source and consult-reference tracking.
- The fix-attempt counter no longer leaks from one task into the next, and a resumed session keeps its fix history.
- Interrupted legacy research splits are detected and surfaced, and unsplittable names are reported up front instead of failing partway.
- Knowledge base commands now work from any subdirectory.
- Whitespace-free runs of 200,000 characters or more no longer crash indexing.
- A missing API key now shows a "provide the key" message instead of advising a destructive rebuild.
- A partial store without metadata is no longer reported as ready by `check`.
- A failed single-file index is queued for automatic retry.
- Error output can no longer leak bearer tokens or API keys.
- Migrations 001–005 no longer abort on files that lack a status line or a `##` section, nor drop their body content.
- Several other migrations that could abort the chain or duplicate or mis-order data, including 006, 019–021, 026, 029, 033, 034, 040, 042 and 044, are fixed.

## 🗑️ Removed

- The per-skill `discovery.cjs` scripts are replaced by `gateway.cjs` equivalents.
- The static epic dependency-map view is removed.
