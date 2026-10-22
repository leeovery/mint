✨ Added
- `tick show --field` (alias `--fields`) selects which fields to print — one name returns the bare value, several return a trimmed document.
- Narrow list sections by position, e.g. `--field notes.2`, with real note indexes preserved.
- `--` end-of-flags marker — everything after it is treated as text, so titles and notes starting with a dash or spelling a global flag work.
- Flags now accept the attached form `--flag=value`, so values like `--` or `--json` can be passed literally.
- `create` and `update` output now includes a `changed` section listing tasks whose status moved (e.g. a done parent reopened).
- Notes in `show` output now carry a 1-based `index` matching what `note remove` accepts.

🔧 Changed
- `start`, `done`, `cancel` and `reopen` print one `changed` table (TOON) or list (JSON) with an `auto` flag marking cascaded tasks.
- TOON output is now valid, decodable TOON: single objects become named fields, tags/refs are inline lists, and descriptions are quoted single values.
- `stats` TOON output uses top-level fields instead of a `stats` row.
- `dep tree` always returns a populated document in TOON and JSON (empty graphs return zero counts), includes cycles and dangling blockers, and uses `trees` instead of `roots` in JSON.
- Focused `dep tree` documents open with the target's id, title and status and always include both `blocked_by` and `blocks` sections.
- TOON `dep tree` emits one edge per stored dependency, with summary values as top-level fields.
- JSON `dep add`/`dep remove` output uses a `blocker` key instead of `blocked_by`.
- `--ready` and `--blocked` now include in-progress tasks.
- `migrate` trims leading and trailing whitespace from imported titles and descriptions, and accepts `--from=beads`.
- `note add` accepts dash-leading text without needing the marker unless it spells a global flag.
- Help output aligns flag columns and documents the new `show` flags and the `--` marker.

🐛 Fixed
- Notes are listed in stored order rather than sorted by timestamp, so indexes match `note remove`.
- TOON output containing control characters it cannot encode now fails with an error naming the task and field, instead of printing a silently incorrect document.
- Pretty and JSON output no longer drop or misreport data for tasks hidden by cycles or missing blockers in `dep tree`.
