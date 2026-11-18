Tasks with very long lines in tasks.jsonl now work everywhere, and `tick doctor` checks the store exactly as the other commands read it.

✨ Added
- Descriptions are capped at 50,000 characters on `create` and `update` — longer ones are refused with a clear error and nothing is saved.

🔧 Changed
- `tick doctor` now loads every line of tasks.jsonl the same way the other commands do — its JSONL syntax check reports each line that would fail to load, with the same line number and reason.
- `tick doctor` reads tasks.jsonl once and shares the result across all checks.
- When a line cannot load, `tick doctor` points the stale or missing cache advice at fixing those lines instead of suggesting `tick rebuild`, which would fail.
- Errors for a bad tasks.jsonl line now name the line number and, where available, the task ID.
- `tick migrate` from Beads now skips issues whose title is over 500 characters or spans several lines, or whose description is over 50,000 characters, and reports each in its failures list.

🐛 Fixed
- Tasks whose line in tasks.jsonl exceeds 64 KiB can now be listed, shown, updated, removed and rebuilt instead of failing.
- `tick migrate` from Beads no longer fails on issues with very long lines.
- Files with CRLF line endings, whitespace-only lines or a trailing carriage return are now read consistently by the store and `tick doctor`.
- `tick rebuild` no longer deletes the existing cache when tasks.jsonl cannot be read or parsed — the cache is left untouched.
- `tick doctor` reports a read that stops partway as a read error instead of "tasks.jsonl not found".
