🔧 Changed
- Fetching a URL already held by another item now requeues it under its owner when it closed with nothing landed (dead, skipped, or done without output) — previously it was refused as "already enriches".
- Notes for refused fetches now say whether the unit is already owed or already enriches under another item.

🐛 Fixed
- Articles on pages with no recognisable body container now keep their headings and lists — previously only paragraphs were stored.
- A renamed item keeps its shared enrichment instead of the unit moving to a co-owner that sorts first and getting a second copy written there.
- Lint no longer reports a renamed item's own file as misfiled under a co-owner.
- A fetch URL that fails to parse now reports the cause in its parked reason.
