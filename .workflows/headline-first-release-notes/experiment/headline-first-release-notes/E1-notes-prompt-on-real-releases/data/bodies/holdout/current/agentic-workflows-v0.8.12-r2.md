✨ Added
- Epic menu can start a discussion or research on a new topic directly — it asks for a name, checks it against the map, and hands off, with a `back` option to return.
- Epic menu now runs specification grouping in place (`s/spec`) — analysis, groupings and specifications menus, and unify all live there, each with `b/back`.
- Back to the start menu (`b/back`) from the epic menu and the new linear continue menu.
- A specification that takes in a discussion an earlier specification already covers now works the earlier one in beside it and supersedes it on completion — with a confirm shown only when the start does this.
- Planning's "add context" offer and cross-cutting references are now saved in the plan itself (`## Plan Context`, `## Cross-Cutting References`), so a resumed plan keeps them.

🔧 Changed
- Every phase now starts in its own skill: checks, resume/reopen and the opening interview all happen there, with no separate "entry" step — fewer hops and a cleaner handoff.
- Feature, bugfix, quick-fix and cross-cutting continue menus are merged into one linear continue menu.
- Sibling-discussion corrections are now recorded as tensions on the receiving grouping and raised during specification construction, replacing consult references.
- The framework instructions load once per conversation instead of at every skill, trimming what each phase start reads.
- `workflow-start` loads its rare branches (migration review, mod notice, walkthrough offer, session labels, baseline judgment) only when they apply.
- Promoting a specification to a cross-cutting unit now moves its source discussions properly: the epic marks them promoted and decided, and the new unit keeps them as sources you can revisit.

🗑️ Removed
- Consult references and their sign-off checks on specifications.
- Cancelled experiment series status — migrated to completed.
- Plan `storage_paths` backfill checks, `--no-session-log` on workunit creation, and review's single/multi/all scope.
- The per-type `workflow-*-entry` skills and the per-type `workflow-continue-*` skills.

🐛 Fixed
- Epic menu's specification start no longer skips incorporating an existing specification that shares a discussion.
- Revisiting a promoted cross-cutting unit no longer starts an empty discussion.
- Starting a topic whose name is already on the map now shows where it stands and how to get back to it, then returns to the menu instead of ending the session.
