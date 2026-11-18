✨ Added
- Deterministic workflow engine (`engine.cjs`) that owns manifest state, transitions, and byte-stable dashboards and menus — status displays are computed from disk instead of re-derived by the model.
- Single-call `engine boot` for startup — runs migrations, checks the knowledge base, and compacts it when ready.
- One-transaction lifecycle commands — create, complete, cancel, reactivate, pivot, absorb, and promote a work unit, each validated before anything changes and committed once.
- Conversational knowledge-base setup with non-interactive `knowledge setup` forms (`--from-system`, `--keyword-only`, `--provider`, `--key-only`) — API keys never pass through chat or command arguments.
- Full documentation set under `docs/` covering every phase, the engine, the knowledge base, agents, configuration, and project history.
- Discussion Map subtopics now live as typed manifest state with a new `deferred` state, rendered on demand.
- Off-topic concerns raised mid-session can be rerouted into the right topic's Triage queue, including closed or superseded topics.
- Engine-rendered gate menus for the implementation loop, with a fix-attempt escalation callout at three attempts.
- Cross-cutting work units now have a specification-entry path.
- Typecheck (`npm run typecheck`) and separate test gates for engine, CLI contracts, and migrations.

🔧 Changed
- All skills now call the engine instead of the old manifest CLI, and per-skill `discovery.cjs` scripts became `gateway.cjs` adapters emitting DATA, DISPLAY, and MENU sections.
- Migrations run through a Node orchestrator that supports new `.cjs` migrations alongside the frozen shell fleet (new migrations 047–049).
- Background review agents now write to a dispatch-time skeleton file that flips from `in-flight` to `pending` on completion.
- Planning and review agents no longer commit; the orchestrator commits their tracking files.
- Executor reports are shorter, and Linear MCP tools are granted only to agents that need them.
- README rewritten as a short pitch with install steps, with detail moved to `docs/`.
- Knowledge-base decay is now progress-based rather than wall-clock.

🐛 Fixed
- Fix attempt counters no longer leak between tasks or reset when a task is resumed.
- Manifest writes are now locked and atomic, with safe stale-lock breaking and cleanup of orphaned temp files.
- Knowledge indexing no longer crashes on very long unbroken text runs.
- Knowledge commands now work from subdirectories and no longer evict pending items from the queue.
- A missing API key is now reported as a key problem instead of a misleading "provider changed — rebuild" error.
- Several shell migrations no longer abort the chain or lose content on edge cases.
- Specification restart now resets source tracking, and legacy research splitting rejects illegal names before changing anything.
- Task counts, review-cycle numbering, and dependency checks in planning, review, and implementation entry flows were corrected.

🔒 Security
- Embedding-provider error messages now redact bearer tokens and API keys.
- `knowledge setup` refuses any `--key` flag, and `check` reports a store missing its metadata as not ready.
- Control characters are stripped from knowledge query output.

🗑️ Removed
- Removed the `workflow-manifest` skill and `manifest.cjs` (replaced by `engine manifest`), plus the `migrate.sh` orchestrator.
- Removed the per-type `revisit-phase.md` references, the epic dependency map view, and the single-discussion specification display variants, all replaced by engine-rendered snapshots.
