Workflow state, displays and menus are now computed by a deterministic engine instead of being re-derived by Claude each session.

## 💥 Breaking

- The `workflow-manifest` skill and its `manifest.cjs` CLI are removed — use `node .claude/skills/workflow-engine/scripts/engine.cjs manifest <command>` instead, where mutations now answer with a JSON line rather than silent output.
- The `workflow-migrate` skill is no longer invoked and `migrate.sh` is replaced by a Node `migrate.cjs` run through `engine boot` — new migrations are `.cjs` modules, and the existing `.sh` ones are frozen.
- `knowledge setup` refuses any `--key` flag — supply the API key through `$OPENAI_API_KEY`, `knowledge setup --key-only`, or the interactive wizard.

## ✨ Added

- `engine boot` runs migrations, the knowledge check and compaction in one call at the start of `/workflow-start`, and stops hard if a migration fails.
- Dashboards, menus, discovery maps and discussion maps are rendered by the engine, so the same state always produces identical output.
- The knowledge base can be set up conversationally from `/workflow-start` through non-interactive forms: `--from-system`, `--keyword-only`, `--provider …` and `--key-only`, which keeps the key out of chat and shell history.
- Transactional commands cover work-unit lifecycle: `workunit create`, `complete`, `cancel`, `reactivate`, `pivot`, `absorb` and `promote`.
- Epic discovery-map edits are now `engine discovery-map` commands: add, edit, remove, rename, re-route, handle/unhandle and sequence.
- Epic discovery sessions can be opened and closed with `engine discovery-session`.
- The Discussion Map is now tracked in the manifest with a new `deferred` state, and unresolved subtopics are deferred when you conclude early.
- Implementation task bookkeeping gains `engine task` commands, with the fix-attempt threshold, cycle limit and approval gate menus rendered by the engine.
- Inbox archive, restore and delete run as single committed transactions via `engine inbox`.
- A full `docs/` set covers the workflow, engine, agents and knowledge base.

## 🔧 Changed

- Quick-fix review verifies against the task's Verification section instead of acceptance criteria.
- Task-executor completion reports are slimmer, with fewer fields.
- Background review and perspective agents now rewrite a skeleton file written at dispatch, so in-flight agents are visible after a context refresh.
- Planning agents no longer commit; the orchestrator does.
- Restarting a discussion, research or specification now also clears stale agent caches and resets tracking rows.
- Completed phases that are reopened now go through a dedicated `topic reopen` transition.

## 🐛 Fixed

- Indexing no longer crashes on huge unbroken tokens such as base64 blobs, which previously overflowed the tokenizer.
- Knowledge commands now resolve artifact paths from the project root, so they work from subdirectories.
- A missing API key now says to supply the key instead of wrongly advising a destructive `knowledge rebuild`.
- Embedding vectors whose width does not match the configured dimensions produce a clear error.
- Credentials in upstream error bodies are redacted.
- Query output strips control characters from indexed content.
- `knowledge check` no longer reports ready when `metadata.json` is missing.
- Single-file indexing failures that exhaust retries are queued for automatic retry.
- Several migrations (001–003, 006, 019–021, 026, 029, 033, 034, 040, 042, 044) are hardened against crashes, truncated writes and data loss.
- Specification and planning resume now handle superseded and promoted specs and missing sources correctly.

## 🔒 Security

- API keys never pass through command arguments, chat or stdout, and are read only from environment variables or a mode-0600 credentials file.
