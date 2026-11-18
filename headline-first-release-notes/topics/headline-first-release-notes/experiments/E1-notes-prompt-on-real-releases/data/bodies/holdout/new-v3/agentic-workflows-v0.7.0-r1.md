Hold a product-level conversation and stage capabilities on a roadmap before committing any of them to delivery.

✨ Added
- Product roadmap (`/workflow-roadmap`) — a project-level layer above work units, with horizons you name yourself and capability-sized items that carry pointers back to the conversation behind them.
- Pull into delivery — pick waiting items and a work unit is born already fenced to them, with the items left behind named so a partial pull is never silent.
- Product-altitude detection in discovery — when no single piece of work is on the table, the conversation routes to the roadmap before any work unit exists.
- Park a thought on the roadmap mid-work ("that's a v2 thing") from epic discovery, discussions and shaping, with an unplaced thought still going to the inbox.
- Roadmap session logs and imports at project level — sessions can be paused and resumed, and are searchable in the knowledge base.
- Epic harvest sorts findings two ways — topics for this epic, or parked on a horizon — and warns when a proposed topic would duplicate a waiting roadmap item.
- Pull-forward — bring a waiting roadmap item into an in-flight epic as a map topic.
- Recognition at shaping — new work that matches a waiting roadmap item or an inbox capture gets one offer to pull or read it in rather than creating a twin.
- Roadmap on the start screen — an `r/roadmap` row, a horizon summary, and a roadmap-only view when no work is in flight.
- `engine roadmap` commands for items, horizons, pulls, sessions and imports, plus `engine commit --roadmap`.

🔧 Changed
- Cancelling a pulled work unit or topic returns its roadmap items to waiting, with their history kept.
- Absorbing a feature into an epic carries its roadmap links to the new topic.
- Removing a never-started topic that was pulled from the roadmap returns the item to waiting.
- The discussion off-topic menu gains a roadmap park option.
- Re-bucketing or removing an item that has been pulled into delivery is refused, with the cancel path named.
- `roadmap` is now a reserved work-unit name.
- Reconcile advisories gain a roadmap case when the product record deepens beneath in-flight work.
