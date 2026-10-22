✨ Added
- `dex-map` compiles the instance map into `state/map.json` — every topic and entity with counts and members, plus a relation graph of wikilink and shared-item edges.
- `dex-enrich place` writes taxonomy and entity placement from a JSON payload — validated whole, refused whole, and written in one canonical form.
- Three new MCP tools, `topics`, `entities` and `graph`, answer "what does this instance hold?" in one call each — the graph is trimmed by default and widened with `around`, `min_weight` and `full`.
- The compiled `state/map.json` is attached as a resource for each served instance.
- Migration 10 compiles the map and index for existing instances at sync time, so the new tools work right after upgrading.
- Lint advisory for digests that name a canonical topic the taxonomy does not record them under — a placement miss or a rename residue for you to judge.

🔧 Changed
- `wiki/index.md` is now rendered by the engine from the taxonomy, entity members and wiki — regenerated whole on every compile and never hand-edited.
- Placement, exclusion and the run flow's new closing step recompile the map and index automatically.
- Lint checks map and index freshness with a byte-for-byte recompile, replacing the per-entry "missing from index" and "ghost index entry" rows.
- `fetch` returns the digest's facts plus its `signal`, and `page` returns the body without frontmatter — digest-time topic classification no longer surfaces as an item's current topics.
- `search` no longer matches digest frontmatter, only the facts and the item itself.
- The run, lint and query skills and their references now route taxonomy, entity-member and index changes through `bin/dex enrich place` and `bin/dex map`.
