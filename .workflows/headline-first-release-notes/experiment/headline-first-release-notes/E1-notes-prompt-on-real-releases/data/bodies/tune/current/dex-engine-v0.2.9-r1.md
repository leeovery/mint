✨ Added
- Pages with no recognisable article body are now read a second time with the whole page treated as the article — headings and lists that were previously dropped now survive.

🔧 Changed
- Fetching a URL already held by another item now explains whether it is landed or still owed — and re-fetches it under its owner if it closed with nothing landed.

🐛 Fixed
- Pages an early engine marked dead or skipped can now be fetched again by any item instead of being refused as "already enriches".
- A renamed item keeps its shared enrichment unit instead of losing it to a co-owner that sorts first, which wrote a duplicate copy.
- Lint no longer reports a renamed item's own file as misfiled under its co-owner.
- Parked fetch URLs now state the cause of the failure in their reason.
