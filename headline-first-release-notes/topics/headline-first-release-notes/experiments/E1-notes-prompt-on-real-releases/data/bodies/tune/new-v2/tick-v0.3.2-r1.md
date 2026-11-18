Tasks with very long lines in the task store now load and work normally, and `tick doctor` judges the store the same way the other commands do.

💥 Breaking
- Descriptions are capped at 50,000 characters after trimming — `create` and `update` now refuse longer ones, so shorten any longer description before saving.

✨ Added
- `tick doctor` reports a clear "could not be read in full" error, naming the line, when tasks.jsonl cannot be read to the end.

🔧 Changed
- `tick doctor`'s JSONL syntax check now fails every line the store cannot load, with the store's own reason and line number, rather than checking JSON syntax alone.
- `tick doctor` advises fixing the failing tasks.jsonl lines instead of running `tick rebuild` when the store cannot load, since a rebuild would fail the same way.
- `tick migrate` now applies the same title (500 characters, single line) and description limits as `create`, skipping and reporting issues that break them.

🐛 Fixed
- Task lines longer than 64 KiB no longer break list, show, update, rebuild, doctor or migrate, and tasks that grow past that size through updates, notes, status changes or dependencies keep working.
- `tick doctor` no longer misnumbers lines or mishandles CRLF files, whitespace-only lines, or a trailing carriage return.
- `tick rebuild` leaves the existing cache untouched when tasks.jsonl cannot be read or parsed, instead of deleting it first.
- Parse errors for tasks.jsonl now name the line and the task ID, for example `line 2 (tick-a1b2c3)`.
- `tick migrate --from beads` no longer fails on issue lines longer than 64 KiB.
