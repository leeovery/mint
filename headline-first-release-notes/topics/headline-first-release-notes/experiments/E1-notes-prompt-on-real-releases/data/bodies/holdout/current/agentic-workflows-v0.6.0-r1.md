✨ Added
- Deterministic workflow engine (`engine.cjs`) that owns state, transitions, and rendering — dashboards, trees, and menus are computed from disk and emitted verbatim instead of re-derived by Claude.
- Single-call transactions for work-unit lifecycle (create, complete, cancel, reactivate, pivot, absorb, promote) — validated up front and committed once, so a failure leaves files untouched.
- Engine commands for discovery map edits, discovery sessions, discussion map subtopics, topic transitions, inbox archive/restore/delete, and analysis-cache stamping.
- `engine boot` runs migrations, the knowledge check, and compaction in one call at startup.
- Conversational knowledge-base setup with non-interactive `knowledge setup` forms (`--from-system`, `--keyword-only`, `--provider`, `--key-only`) — API keys never pass through chat or command arguments.
- Engine-rendered gate menus for the implementation loop (task gate, fix gate, blocked tasks, analysis cycle limit).
- Full documentation set under `docs/` covering every phase, the engine, knowledge base, agents, and project history.
- Migrations can now be written as Node `.cjs` modules alongside the frozen bash fleet, with new migrations for discussion maps, discovery item status, and nested `.workflows/.gitignore`.
- Cross-cutting work units can now run specification end to end, including source validation and entry handoff.
- Discussion subtopics can be marked `deferred` when concluding with unresolved items.
- Quick-fix scoping can be revisited: adjust the existing spec and plan, or restart from scratch.
- Typechecking (`npm run typecheck`) and a much larger `npm test` suite.

🔧 Changed
- All per-skill discovery scripts are now thin `gateway.cjs` adapters emitting DATA / DISPLAY / MENU sections, and the manifest CLI is folded into `engine manifest` with batched `set`.
- Discussion Map state now lives in the manifest rather than the discussion file.
- Background agents get a dispatch-time skeleton file marked `in-flight`, making running agents visible to resume and conclusion checks.
- Planning and review agents no longer commit; the orchestrator owns all commits.
- Implementation executor reports are trimmed to a minimal format.
- Task-verifier statuses are now lowercase (`complete | incomplete | issues_found`).
- Final reviews only dispatch a fresh cycle when the artifact changed since the last one.
- README rewritten as a concise pitch, pointing to the new docs.

🐛 Fixed
- Fix attempts no longer leak across tasks, and restarts no longer wipe a task's fix history.
- Legacy research split rejects illegal topic names and avoids overwriting existing research files.
- Indexing very long unbroken text runs no longer crashes the knowledge indexer.
- Knowledge commands now work from subdirectories and report a missing API key instead of a misleading "rebuild" prompt.
- Several frozen migrations hardened against crashes, truncated writes, and data loss (e.g. missing status lines, hand-edited settings, directory merges).
- Missing or partial knowledge stores are now reported as not-ready instead of passing the boot check.

🔒 Security
- Provider error messages redact bearer tokens and API-key-shaped strings.
- Query output strips terminal control characters from indexed content.

🗑️ Removed
- `workflow-manifest` skill and CLI, the `/workflow-migrate` flow, and per-skill `discovery.cjs` scripts.
