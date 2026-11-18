💥 Breaking
- Status commands (`start`, `done`, `cancel`, `reopen`) now print a `changed` table (TOON) or `changed` list (JSON) of every task that moved — scripts reading the old `id: old → new` lines or the JSON `id`/`from`/`to` and `transition`/`cascaded` objects must read the new shape instead.
- TOON output is restructured: `show` and `stats` now emit top-level `key: value` fields, descriptions are one quoted value, tags and refs are inline lists, and `dep tree` summary fields are top-level — update any parser of the old layout.
- `dep tree --json` full graph now returns `trees` instead of `roots`, and `dep add`/`dep remove` JSON uses `blocker` instead of `blocked_by`.
- Focused `dep tree` JSON now always includes `blocked_by` and `blocks` as arrays, with no `message` key; no-dependency graphs return the emptied document instead of a message in TOON and JSON.
- A TOON command whose output would carry an unencodable control character (such as a pasted ANSI escape) now fails with an error naming the task and field instead of printing a broken document — use `--json` or `show --field` to read the value.

✨ Added
- `tick show --field` (alias `--fields`) selects fields by name, returns a lone value bare, and narrows list sections by position such as `notes.2`.
- `--` ends flag parsing so titles and note text starting with a dash, or spelling a global flag, are stored as text.
- Value-taking flags accept the attached form `--flag=value`, including on `migrate --from=beads`.
- `create` and `update` output a `changed` section listing any tasks whose status moved, such as a reopened or auto-completed parent.
- Task notes now show a 1-based `index` in TOON and JSON that matches what `note remove` accepts.
- Dependency graphs list every participant, including cycles and blockers whose task no longer exists (shown as `missing`).

🔧 Changed
- `ready` and `blocked` filters now include in-progress tasks as well as open ones.
- Imported titles and descriptions from `migrate` are trimmed of leading and trailing whitespace.
- `note add` accepts dash-leading text without needing `--`.
- Flag help columns are aligned to the widest flag label.

🐛 Fixed
- Notes now keep their stored order instead of being sorted by timestamp, so displayed positions match `note remove`.
- Dropped values no longer silently disappear from TOON output; empty types in lists render as `""`.
