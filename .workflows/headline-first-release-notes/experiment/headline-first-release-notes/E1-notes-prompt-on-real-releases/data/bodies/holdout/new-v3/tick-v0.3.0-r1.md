💥 Breaking
- TOON output shapes changed — `show`, `stats`, `dep tree`, `create`, `update` and status commands now print top-level `key: value` fields, `tags`/`refs` as inline lists, notes with an `index` column and descriptions as one quoted value, so scripts parsing the old layout must be updated.
- Status commands (`start`, `done`, `cancel`, `reopen`) now print a `changed` table in TOON and a `changed` list in JSON instead of arrow lines and the old `transition`/`cascaded` objects.
- JSON `dep tree` full-graph output renames `roots` to `trees`, and focused output always includes `blocked_by` and `blocks` and no longer carries `message`.
- JSON `dep add` and `dep remove` output renames the `blocked_by` key to `blocker`.

✨ Added
- `tick show <id> --field name,...` (alias `--fields`) selects fields, returns a single value bare, and narrows list sections by position such as `notes.2`.
- `--` ends flag parsing so dash-leading titles and note text can be passed, e.g. `tick create -- "--dry-run support"`.
- Flag values can be attached with `=`, e.g. `--title=--` or `--from=beads`.
- `create` and `update` output includes a `changed` section listing tasks whose status moved, such as a done parent reopened or an old parent auto-completed.
- Notes in `show` output carry their 1-based `index`, matching what `note remove` accepts.

🔧 Changed
- `dep tree` with no dependencies returns an emptied TOON/JSON document with zero counts instead of a prose message.
- `list --ready` and `--blocked` now consider in-progress tasks as well as open ones.
- Imported titles and descriptions from `migrate` are trimmed of edge whitespace.
- Help output aligns flag descriptions to the widest flag label per command.

🐛 Fixed
- TOON output is now properly quoted and round-trips awkward text (dashes, commas, quotes, blank lines) intact, and a value TOON can't encode fails with a clear error naming the task instead of printing a broken document.
- `dep tree` now shows tasks in cycles and dependencies on missing blockers rather than omitting them.
- Notes now display in stored order even when their timestamps run backwards.
- Empty task type in TOON lists is now quoted so rows decode correctly.
