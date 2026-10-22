✨ Added
- `tick show --field` (alias `--fields`) — select fields by name, with a lone value printed bare and list sections narrowed by position such as `notes.2`.
- `--` end-of-flags marker — everything after it is treated as text, so titles and notes that start with a dash or spell a global flag can be stored.
- Attached flag values with `--flag=value` — lets a value that is exactly `--` or spells a global flag be passed safely.
- Notes now carry a 1-based `index` in TOON and JSON output — the number to pass to `note remove`.
- `dep tree` now draws cycles and dangling blockers, the latter shown with status `missing`.

🔧 Changed
- Status commands (`start`, `done`, `cancel`, `reopen`) now report every moved task in one `changed` table with an `auto` column, in TOON and JSON.
- `create` and `update` now include a `changed` section when they reopen or auto-complete parent tasks.
- TOON output now uses named `key: value` fields, inline lists, and a single quoted description, so documents decode cleanly.
- `stats` and `dep tree` TOON summaries are now top-level fields instead of one-row tables.
- Focused `dep tree` always returns both `blocked_by` and `blocks` in TOON and JSON, and empty graphs return an emptied document instead of a message.
- JSON `dep tree` uses `trees` instead of `roots`, and `dep add`/`dep remove` JSON uses `blocker` instead of `blocked_by`.
- `--ready` and `--blocked` now include in-progress tasks.
- `migrate` trims leading and trailing whitespace from imported titles and descriptions.
- `migrate` now accepts `--from=value`.
- Help output aligns flag descriptions and documents the `--` marker.

🐛 Fixed
- Notes are now numbered in stored order rather than sorted by timestamp, so indexes always match `note remove`.
- TOON output no longer silently drops data it cannot encode — the command fails, naming the task and field, while the value stays readable via `--json` and `show --field`.
- Dash-leading note text such as `- text` is now accepted by `note add` without needing `--`.
