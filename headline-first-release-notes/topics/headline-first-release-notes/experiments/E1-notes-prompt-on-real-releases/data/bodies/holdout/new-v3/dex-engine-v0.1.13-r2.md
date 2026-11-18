A compiled map of each instance's topics, entities and relations now backs both the catalog and the chat tools.

✨ Added
- `dex-map` compiles `state/map.json` — every topic and entity with counts, members and page status, plus a relation graph — and renders `wiki/index.md` from the same compile.
- `dex enrich place` records taxonomy and entity placement from a JSON payload — the payload is validated whole and refused whole, and the map and index are recompiled afterwards.
- Three new `dex-serve` tools, `topics`, `entities` and `graph`, answer "what does this instance hold" in one call each — the graph is trimmed to a readable default and opened up with `around`, `min_weight` and `full`.
- The compiled `state/map.json` is attached to `dex-serve` as a resource alongside the index and taxonomy.
- Lint advisory for digests that name a defined topic whose items don't record them — a placement miss or a rename residue for you to judge.

🔧 Changed
- `wiki/index.md` is now rendered from the taxonomy and wiki on every compile — hand edits no longer survive, and syntheses are listed by their first `#` heading.
- Lint now flags stale or missing map and index files with a single row, replacing the separate unindexed-page and ghost-index-entry checks — `dex-map` is the repair.
- `dex-exclude` recompiles the map and index after a purge.
- Run flow gains a closing `dex map` step before the health check.
- `fetch` returns the digest's fact body plus its `signal` instead of the raw digest file, and `page` returns the page body without its frontmatter.
- Sync runs a one-time migration that builds the map and replaces the existing model-written index with the rendered one.

🐛 Fixed
- `search` no longer matches or snippets a digest's stale topic classification — only the fact body is searched.
