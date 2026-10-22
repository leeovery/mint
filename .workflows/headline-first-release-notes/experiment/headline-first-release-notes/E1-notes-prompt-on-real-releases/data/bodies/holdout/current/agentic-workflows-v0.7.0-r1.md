✨ Added
- Product roadmap — a project-level layer above work units for product conversations, with horizons you name yourself (MVP, v1, someday) and capability-sized items that carry pointers back to the thinking behind them.
- New `/workflow-roadmap` skill with genesis, open, and pull modes, plus an `r/roadmap` row on the start menu once a roadmap exists.
- Pull into delivery — pick one or more waiting items and a work unit is born already fenced to them, with any remaining items named and left visibly waiting.
- Park an idea with "that's a v2 thing" during epic discovery or a discussion — it lands on the roadmap in the horizon you name, and the roadmap is created on first use.
- Epic harvests can now sort material two ways, into this epic's topics or onto the roadmap, and pull a waiting item forward into an in-flight epic as a topic.
- Item lifecycle on the roadmap is computed from the work unit it joined (waiting, in flight, shipped), so the roadmap never disagrees with delivery.
- Roadmap session logs and imports are indexed in the knowledge base, and product-level imports have their own project-level home.
- Start screen shows the roadmap's horizons with counts, and never renders as empty when a roadmap exists with no work in flight.
- `engine roadmap` commands for items, horizons, pulls, sessions and imports, plus `engine commit --roadmap` and new `render` surfaces for the roadmap views and gates.
- Recognition at shaping — if new work matches a waiting roadmap item or a forgotten inbox capture, discovery offers the pull or read-in instead of creating a twin.

🔧 Changed
- Discovery now routes product-altitude conversations (no single piece of work on the table) out to the roadmap before any work unit exists.
- Cancelling a topic or work unit hands its pulled roadmap items back to waiting with their history intact, and absorbing a feature re-points the joins to the epic topic.
- Pulled items refuse re-bucketing or removal, since redirecting in-flight work is the work unit's decision, while summary edits and horizon relabeling stay open.
- Several static gates (name confirm, shape confirm, synthesis confirm, knowledge query failure) are now engine-rendered instead of hand-drawn in prose.
- Reports and voice guidance gain a condition-first rule and a ban on minimizers and marketing words.
