Starting a phase now takes fewer hops: each phase checks and starts itself, and the epic menu handles everything that has no topic yet.

✨ Added
- Epic menu's `s/spec` row now runs the specification grouping in place, with `b/back` to return to the epic menu.
- Epic menu and the single-topic continue menu gain a `b/back` row that returns to the start menu.
- A specification that takes in a discussion an earlier specification already covers now records and incorporates that earlier specification on every route, and supersedes it on completion.
- The `d`/`r` doors on the epic menu now ask for the topic name there and, if the name is already on the map, point to the row that brings it back.
- Planning now asks at its start for any context added since the specification completed, and records it with the cross-cutting references in the plan file.

🔧 Changed
- Feature, bugfix, quick-fix and cross-cutting units now share one continue menu (`workflow-continue-linear`) instead of four near-identical ones.
- The specification confirmation only appears when the start does something the pick didn't show — incorporating a specification, unifying groupings, or the single-discussion path.
- Rarely used start-up steps (migration review, mod notice, walkthrough offer, session labels, baseline judgment) load only when they apply, making a typical start lighter.
- The framework instructions now load once per conversation instead of at every skill.
- Promoting an epic specification to a cross-cutting unit now carries its source discussions as the unit's sources and marks them promoted in the epic, so they're no longer grouped, cancelled or postponed there.
- Resuming a finished topic now reopens it directly in the phase instead of passing through a separate continue-or-restart menu.

🐛 Fixed
- After promotion, revisiting a moved discussion in the cross-cutting unit now reaches it by name instead of starting an empty one.

🗑️ Removed
- Consult references are gone; a correction a sibling discussion owes a grouping is now noted as a tension on that grouping and raised during specification construction.
- Phase entry skills (`workflow-*-entry`) and the per-type continue skills are removed; a move into a phase now goes straight to its phase skill.
- The old per-series cancelled status for experiments is gone; such series now read completed.
