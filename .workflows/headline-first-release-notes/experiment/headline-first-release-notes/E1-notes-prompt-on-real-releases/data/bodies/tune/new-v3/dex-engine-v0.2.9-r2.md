🐛 Fixed

- Articles on pages with no recognisable body container now keep their headings and lists — previously only the paragraphs were stored.
- A newsletter box or similar small block matched ahead of the article no longer strips the article's headings and lists.
- A renamed item now keeps its shared enrichment file instead of a co-owner taking it over and writing a second copy.
- Lint no longer reports a renamed item's own enrichment file as misfiled under its co-owner.
- `enrich fetch` of a URL whose earlier fetch closed dead, skipped or done with nothing stored now fetches it again under its owner instead of refusing it as "already enriches".

🔧 Changed

- `enrich fetch` of a URL still queued or blocked under another item now says it is already owed, rather than claiming it already enriches.
