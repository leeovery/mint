Pages without a recognisable article container now keep their headings and lists when fetched.

🔧 Changed
- Fetched articles that trafilatura cannot find a body for are read a second time with the whole page body as the article — headings and lists are kept instead of only paragraphs.
- `enrich fetch` of a URL held by another item whose unit closed dead, skipped, or done with nothing landed now re-fetches it under its owner instead of refusing it.
- `enrich fetch` of a URL still queued or blocked under another item now says it is owed, not that it "already enriches".

🐛 Fixed
- A renamed item keeps its shared enrichment unit instead of a co-owner taking it and writing a second copy.
- Lint no longer reports a renamed item's own enrichment file as misfiled under its co-owner.
- Articles with a small matched box, such as a newsletter signup, ahead of the real content no longer lose their headings and lists.
- Fetching a dead pre-rename unit from another item now lands the page under the live item id instead of a dead one.
- The parked-URL note for an unfetchable `enrich fetch` URL now states the cause.
