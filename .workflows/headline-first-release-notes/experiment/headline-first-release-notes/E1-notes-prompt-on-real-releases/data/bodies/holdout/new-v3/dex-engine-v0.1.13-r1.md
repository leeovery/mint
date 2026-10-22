Instances get a compiled map of their topics, entities and relations, served to chat clients and kept fresh by the engine.

✨ Added
- `dex-map` compiles `state/map.json` — every topic and entity with counts and members, plus a relation graph of wikilinks and shared items.
- `topics`, `entities` and `graph` tools on the `dex-serve` server — ask "what does this instance hold" in one call, with `graph` trimmed by default and widened via `around`, `min_weight` and `full`.
- `dex-enrich place` writes taxonomy and entity placement from a JSON payload — validated whole, refused whole on any error, and it recompiles the map and index.
- The compiled `state/map.json` is attached as a resource for each served instance.
- Health check advisory for digests that name a defined topic which does not record them — a placement miss or a rename leftover.
- Health check reports when `state/map.json` or `wiki/index.md` is missing or out of date, with `bin/dex map` as the repair.

🔧 Changed
- `wiki/index.md` is now rendered by the engine from the taxonomy, entities and syntheses — hand edits no longer survive the next compile.
- `dex-exclude` recompiles the map and index after a purge.
- Syncing existing instances runs a one-time migration that builds the map and replaces the old index.

🗑️ Removed
- Per-entry index checks (pages missing from index, ghost index entries) — replaced by the single map freshness row.

🐛 Fixed
- Search and fetch no longer expose a digest's stale classification — search matches only the digest's facts, and fetch returns the fact body plus a separate `signal`.
- Page reads from `dex-serve` return the page body without its `generated:` frontmatter bookkeeping.
