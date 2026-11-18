🐛 Fixed

- Articles on pages with no recognisable body container now keep their headings and lists — previously only the paragraphs were stored.
- A renamed item's shared enrichment is no longer moved to a co-owner and duplicated there, and lint no longer reports it as misfiled.
- Fetching a URL already held by another item now requeues it under its owner when it closed dead, skipped or done with nothing landed — previously it was refused as "already enriches".

🔧 Changed

- Fetching a URL whose enrichment is still queued or blocked under another item now says it is already owed, rather than claiming it already enriches.
- Parked bad fetch URLs report the underlying cause in the parked reason.
