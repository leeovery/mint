✨ Added
- Descriptions are now capped at 50,000 characters on `create`, `update` and `migrate` — over-limit input is refused with a clear message and nothing is saved.
- `.worktreeinclude` file so the knowledge store files carry into new worktrees.

🔧 Changed
- Task lines in `tasks.jsonl` can now be any length — tasks over 64 KiB list, show, update, remove and rebuild normally.
- `tick doctor` reads the store once and checks each line exactly as the store loads it — lines that fail to load are reported with their line number and the loader's reason.
- `tick doctor` reports a read interrupted partway as an error on every line-based check instead of "tasks.jsonl not found".
- `tick doctor` points a stale or missing cache at the failing lines rather than `tick rebuild` when a line cannot be loaded.
- `tick migrate` now applies the CLI's title rules (500 characters, single line) and skips offending issues with a stated reason.
- Error messages for unreadable `tasks.jsonl` lines now name the line and, where available, the task ID.

🐛 Fixed
- `tick rebuild` no longer deletes `cache.db` when `tasks.jsonl` fails to parse — the existing cache is left untouched.
- Beads migration no longer fails on issue lines longer than 64 KiB.
- Whitespace-only lines and CRLF line endings in `tasks.jsonl` are numbered and handled the same way by `doctor` and the store.
