Tasks with very long lines in the task store can now be read, listed and rebuilt without errors.

💥 Breaking
- Descriptions are capped at 50,000 characters — `create`, `update` and `migrate` now refuse longer ones, so shorten them first.
- `migrate` now skips issues whose title is over 500 characters or spans multiple lines, and lists them as failures.

🔧 Changed
- `tick doctor` now reads the store the same way other commands do, so its line numbers and JSONL syntax verdicts match what `list` reports.
- `tick doctor` points you at the failing lines instead of `tick rebuild` when a rebuild would fail anyway.
- `tick doctor` reports a read error, rather than "not found", when `tasks.jsonl` cannot be read in full.
- Parse errors for `tasks.jsonl` now name the line and the task ID with the reason.

🐛 Fixed
- Tasks whose stored line exceeds 64 KiB no longer break `list`, `show`, `update`, `rebuild` or other commands.
- `tick rebuild` no longer deletes the existing cache when `tasks.jsonl` fails to parse.
- `migrate --from beads` no longer fails on issue lines over 64 KiB.
- `tick doctor` no longer reports false problems for CRLF files, whitespace-only lines or oversized lines.
