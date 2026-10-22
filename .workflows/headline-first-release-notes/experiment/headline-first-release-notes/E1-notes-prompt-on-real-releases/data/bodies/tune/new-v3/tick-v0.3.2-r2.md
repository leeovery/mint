Tasks with very long lines in `tasks.jsonl` now load, list and rebuild correctly, and `tick doctor` reads the file the same way the rest of tick does.

## ✨ Added
- Descriptions are capped at 50,000 characters on `create` and `update` — longer ones are refused with a clear message and nothing is saved.

## 🔧 Changed
- `tick doctor` JSONL syntax check now flags any line the store cannot load, such as `null`, `{}`, wrong-typed fields or bad timestamps, and gives the loader's reason.
- `tick doctor` no longer advises `tick rebuild` when a line fails to load — it points you to fix the named lines first.
- `tick migrate` from beads now applies the same title rules (500 characters, single line) and the description cap, skipping and reporting offending issues.
- `tick rebuild` reads and parses `tasks.jsonl` before touching the cache, so a parse failure leaves `cache.db` unchanged.
- Parse errors for `tasks.jsonl` now name the line and, where available, the task ID.

## 🐛 Fixed
- Tasks with lines over 64 KiB in `tasks.jsonl` no longer break listing, rebuilding, updating or doctor checks.
- `tick migrate` from beads no longer fails on issue lines longer than 64 KiB.
- `tick doctor` line numbers now match those reported by `tick list` for whitespace-only lines and CRLF files.
- `tick doctor` reports a failure for each check when `tasks.jsonl` can only be partly read, instead of wrongly reporting it as not found.
- Files with CRLF line endings or a trailing carriage return are read the same as LF files.
