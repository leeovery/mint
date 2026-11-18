Tasks with very long lines in the store now work everywhere, and doctor checks the store the same way every other command loads it.

✨ Added
- Descriptions are capped at 50,000 characters on `create` and `update` — over-long ones are refused with a clear error and nothing is saved.

🔧 Changed
- `tick doctor` JSONL syntax check now flags every line the store cannot load as a task, with the loader's reason — not just malformed JSON.
- `tick doctor` points you at the failing lines instead of `tick rebuild` when a rebuild would fail anyway.
- `tick doctor` reports a failed read of tasks.jsonl as a read error instead of "not found".
- `tick migrate` from beads now skips issues with titles over 500 characters, multi-line titles, or descriptions over 50,000 characters, listing each as a failure.

🐛 Fixed
- Tasks whose stored line exceeds 64 KiB no longer break list, show, update, remove, rebuild or doctor.
- Large descriptions, notes and dependency changes that push a task past 64 KiB now save and read back correctly.
- `tick migrate` from beads no longer fails on issue lines over 64 KiB.
- `tick doctor` no longer misreports line numbers or relationships around whitespace-only, CRLF or very long lines.
- `tick rebuild` leaves the existing cache untouched when tasks.jsonl cannot be parsed, instead of deleting it first.
- Parse errors for tasks.jsonl now name the line and task ID with the reason for the failure.
