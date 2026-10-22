Pages with no recognisable article container now keep their headings and lists when fetched.

🔧 Changed
- Fetched pages whose article body can't be located are read a second time, so headings and lists are kept instead of just paragraphs — the second reading is used only when it loses no words, links, code fences or table rows.
- Fetching a URL already held by another item now says whether it is owed or already landed, and a page that closed dead, skipped or with no output is requeued under its owner instead of being refused.

🐛 Fixed
- A renamed item now keeps its shared enrichment file instead of a co-owner taking it over and writing a second copy.
- Lint no longer reports a renamed item's own file as misfiled under a co-owner.
