✨ Added
- Per-topic discovery briefs — at the end of an epic's discovery, each topic gets a brief capturing soft decisions, rejected paths, and open questions that research and discussion read as their starting context.
- Reconcile advisory — when a brief is regenerated after downstream research or discussion has started, that work is flagged for review rather than overwritten.
- Epic discovery session logs are now indexed into the knowledge base — earlier exploration is searchable by later phases and sibling epics.
- Re-entering an epic's discovery now briefs you on recent sessions first — you resume the conversation instead of restarting from the topic list.
- Ambient harvest nudge — when an epic's discovery conversation converges, a light aside tells you topics can be pulled out whenever you're ready.
- Migration that moves existing discovery session logs into a `sessions/` folder.

🔧 Changed
- Epic discovery is now a substantive, challenging design conversation that works through decisions, rather than a breadth-only sketch.
- Discovery session logs are now a running record of the conversation, including rejected paths and in-session findings, rather than a brief summary.
- Discovery session logs moved to `discovery/sessions/session-NNN.md`.
- README rewritten around a two-step install and `/workflow-start` as the single entry point, with the knowledge-base setup tucked into a collapsible section and the feature descriptions condensed.

🗑️ Removed
- The Workflow Explorer interactive page and its GitHub Pages deployment.
- Internal planning and design documents (discovery map phases, knowledge base plans, incoming, and spec-groupings plans).
