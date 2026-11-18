Starting a phase now happens inside the phase itself, with fewer hand-offs and a lighter start menu.

✨ Added
- Epic menu can start a new discussion or research topic directly — it asks for the name, checks it isn't already on the map, and carries it straight into the phase.
- Specification grouping now runs inside the epic menu (`s/spec`) — analyze, group, unify and pick a specification without leaving the menu, with `b/back` at each step.
- `b/back` on the epic and linear continue menus returns to the start menu.
- A specification that takes in a discussion an earlier specification already covers now incorporates that specification and supersedes it on completion, whichever way it was started.
- Planning asks for extra context and cross-cutting references up front and records them in the plan file, so a resumed plan keeps them.

🔧 Changed
- Feature, bugfix, quick-fix and cross-cutting units now share one continue menu.
- Each phase checks its own prerequisites, then resumes, reopens or starts in one step, replacing the separate intake skills.
- The confirm before a specification starts appears only when it incorporates another specification, unifies groupings, or follows the single-discussion path.
- Sibling-discussion corrections are now raised as tension notes on the receiving grouping instead of consult references, and consult references are removed (a migration clears them).
- Promoting a specification now moves its source discussions properly: the epic marks them promoted and the new unit keeps them as the specification's sources, so they can be revisited by name.
- Rarely used start-up steps (migration review, setup notices, walkthrough offer, session label prompt, baseline judgment) load only when they apply, making each start lighter.
- The framework instructions load once per conversation rather than at every skill.

🗑️ Removed
- The per-phase intake skills and the separate per-type continue skills are gone, along with the continue skills' pick-a-unit step.
- Plans no longer backfill the `storage_paths` field mid-session.

🐛 Fixed
- Promoted discussions no longer stay marked completed in the epic, where they could be regrouped into a new specification.
- Reactivating a cancelled experiment series from an older release now settles it as completed (a migration repairs these).
