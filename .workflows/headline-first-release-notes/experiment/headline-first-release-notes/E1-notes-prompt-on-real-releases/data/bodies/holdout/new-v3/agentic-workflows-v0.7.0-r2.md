Hold the product conversation and stage capabilities on a roadmap before committing any of it to delivery.

✨ Added
- Product roadmap — a project-level layer above work units where you name horizons (MVP, v1, someday), keep capability-sized items in them, and see each item's state (waiting, in flight, shipped) computed from the work it became.
- Pull into delivery — select roadmap items and a work unit is created already fenced to them, with the remainder named so a partial pull is never silent.
- Product-altitude conversations — when you describe a product's future rather than a single piece of work, discovery routes to the roadmap before any work unit exists, and the session is recorded and resumable.
- Park to the roadmap — say "that's a v2 thing" during discovery, an epic's exploration or a discussion and the idea lands on the roadmap in the horizon you name; unplaced thoughts still go to the inbox.
- Epic harvest sorts two ways — topics for this epic, or items parked to the roadmap, with a check that stops a new topic duplicating a waiting roadmap item.
- Pull-forward — bring a waiting roadmap item into an in-flight epic as a topic.
- Recognition at shaping — new work that matches a waiting roadmap item or an inbox capture gets a single offer to pull or read it in instead of creating a twin.
- Start menu shows the roadmap — horizon counts in the overview and an `r/roadmap` row to resume, browse, or pull; a harvested roadmap with no work in flight no longer shows an empty screen.
- Roadmap sessions and imports are indexed in the knowledge base, so later work can query the product-level record.
- Horizon management — rename, reorder, merge, split and remove horizons at any time, including ones holding delivered items.

🔧 Changed
- Re-bucketing or removing a roadmap item that has been pulled is refused, because redirecting in-flight work is a delivery decision; cancel it from the epic and the item returns to waiting.
- Cancelling a work unit or a topic hands its roadmap items back to waiting, and removing a never-started topic does the same.
- Absorbing a feature into an epic carries its roadmap items along to the new topic.
- If a product session deepens ground under a pulled item, the epic's topic is flagged to reconcile rather than edited.
- `roadmap` is now a reserved work-unit name.
