✨ Added
- Product roadmap — a project-level layer above work units for product-altitude conversations, with user-named horizons (MVP, v1, someday) holding capability-level items that stay visibly waiting until you commit to them.
- Pull into delivery — pick one or more roadmap items and a work unit is born already fenced to that slice, with the remainder named so a partial pull is never silent.
- Roadmap parking — say "that's a v2 thing" during discovery or a discussion and the idea lands on the roadmap in the horizon you named; unplaced thoughts still go to the inbox.
- Recognition at shaping — when new work matches a waiting roadmap item or an inbox capture, discovery offers to pull or read it in instead of creating a twin.
- Pull-forward into a running epic — bring a waiting roadmap item into an in-flight epic as a topic.
- Roadmap on the start screen — horizons with in-flight, waiting, and shipped counts, plus an `r/roadmap` row to browse, resume a product session, or pull; a harvested roadmap with no work units never shows an empty screen.
- Roadmap management commands — `engine roadmap` verbs for items, horizons (rename, reorder, merge, split), sessions, imports, and `commit --roadmap`.
- Roadmap session logs and imports are indexed in the knowledge base, so later phases can query the product-level record.
- New docs page, "The product roadmap", plus README and knowledge-base updates.

🔧 Changed
- Discovery routes product-altitude conversations (no single unit of work on the table) to the roadmap before any work unit is created.
- Epic harvest now sorts two ways — topics for this epic, or parks and pull-forwards for the roadmap — and flags topics that would duplicate a waiting roadmap item.
- Cancelling a topic or work unit hands its pulled roadmap items back to waiting, and removing a never-started topic does the same.
- Absorbing a feature into an epic carries its roadmap items along to the new topic.
- Re-bucketing or removing a roadmap item that is already pulled is refused, since that is a delivery decision for the epic.
- Reconcile advisory gains a roadmap case: when a product session deepens ground under in-flight work, the epic is flagged to re-read it without being overwritten.
- Menus and gates are now engine-rendered rather than authored in prose, including the name, shape, synthesis, and knowledge-query-failure gates.
- Discussion off-topic menu gains a roadmap park option.
- Report and voice guidance gains a condition-first rule and a ban on minimizers and inflated wording.
