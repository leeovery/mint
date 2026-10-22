✨ Added
- `dex-map` command compiles `state/map.json` — every topic and entity with counts and members, plus a typed relation graph of wikilink and shared-item edges.
- `dex-enrich place` writes taxonomy and entity placement from a JSON payload — validated whole, refused whole on any error, and written in one canonical form.
- Three new MCP tools, `topics`, `entities` and `graph` — answer "what does this instance hold?" in one call, with the graph trimmed to a readable default and opened up by `around`, `min_weight` and `full`.
- The compiled `state/map.json` is now offered as an MCP resource alongside the index and taxonomy.
- Lint advisory for digests that name a canonical topic the taxonomy doesn't record them under — a placement miss or a rename leftover for you to judge.
- Migration 10 compiles the map and index for existing instances on their next sync.

🔧 Changed
- `wiki/index.md` is now rendered from the taxonomy, entity members and wiki on every compile — never hand-edited, so a catalog correction is made through `enrich place`.
- Lint's per-entry index checks are replaced by a single map-and-index freshness check — one stale row, one repair with `bin/dex map`.
- `dex-exclude` recompiles the map and index after a purge, and a recompile failure no longer hides the deletions that landed.
- The run flow gains a closing "compile the map" step before the health check.
- `fetch` now returns the digest's fact body plus its `signal` instead of the raw digest file.
- `page` returns the page body without its `generated:` frontmatter bookkeeping.

🐛 Fixed
- `search` no longer matches or snippets digest frontmatter, so stale digest-time topic names don't surface as hits.
