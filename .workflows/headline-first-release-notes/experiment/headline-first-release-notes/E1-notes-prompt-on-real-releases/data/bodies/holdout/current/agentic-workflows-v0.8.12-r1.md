✨ Added
- Epic menu can start a new discussion or research topic directly — name it, it's checked against the map, and the work begins without a separate intake step.
- Specification grouping now runs inside the epic menu's `s` row, with `b/back` to return to the menu.
- Specifications now automatically absorb earlier specifications that cover the same discussions, and ask for confirmation before doing so.
- Back option on the epic and linear continue menus to return to the start menu.

🔧 Changed
- Every phase now starts in its own skill, which checks prerequisites, resumes or reopens existing work, and gathers context where none exists — the separate "entry" skills are gone.
- Feature, bugfix, quick-fix and cross-cutting continue menus merged into one shared linear continue menu.
- Completed discussions promoted with a cross-cutting specification now move with it, are marked as promoted in the epic, and can be revisited by name from the new unit.
- Planning now collects extra context and cross-cutting references as part of its own initialization and records them in the plan file.
- The framework instructions load once per conversation instead of at every skill, trimming startup cost.
- Rarely used start-menu branches (migration review, setup notices, walkthrough offer, session labels, baseline judgment) load only when needed, making a typical start lighter.
- Sibling discussions that change something a grouping depends on are now recorded as tension notes on that grouping, raised during specification construction.
- Re-running a stale analysis with no groupings left now goes straight through the normal analyze prompt.

🗑️ Removed
- Consult references are retired, along with their manifest field and sign-off check; existing ones are converted to tension notes by migration.
- Obsolete handling for cancelled experiment series, logless work units, and `storage_paths` backfills is removed; migrations settle existing data.

🐛 Fixed
- Cancelling or postponing a topic whose specification was promoted is now refused, since promotion is terminal.
- Epic menu's "already on the map" refusal now names the right way back (reactivate, pull forward, resume or reopen) and returns to the menu rather than stopping.
- Reactivating a specification now points to the epic menu's `s/spec` for regrouping instead of a removed entry.
