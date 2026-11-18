✨ Added
- X posts with video are now transcribed — the video's audio becomes part of the post's text instead of being saved as a file to describe.
- X long-form articles keep their code listings, link targets, dividers, embedded posts and figures, with figures and the cover downloaded in reading order.
- GitHub directory links read that directory's README and file listing, and `raw` and `tree` links read the file or folder they point at.
- GitHub links to a repo at a specific branch or tag read that ref's README.
- GitHub release-asset, source-archive and `user-attachments` document links are now extracted as documents.
- Web pages that declare their markdown source are stored from it, keeping tables and code blocks that extraction used to drop.
- Math in web pages and arXiv papers is preserved as TeX, and LaTeXML tables and equations come through intact.
- Four new migrations re-read GitHub links, X videos, X articles and web pages stored before these fixes.

🔧 Changed
- Captures are named with a random suffix (`20260917-082604-a3f9`) — two machines capturing in the same second no longer conflict.
- Media captures are filed by their staged asset rather than file name — repeated names like `screenshot.png` no longer overwrite each other.
- GitHub links with no matching route (commits, release pages, wikis) now park for judgment instead of storing the repo's root README.
- A rerun whose re-fetch fails keeps the stored copy and a stored transcript instead of replacing it.
- Items stay enriched while a rerun over an existing landing is pending.
- `dex-enrich compact` keeps the last line that named an output for a requeued unit.
- Scheduled-task setup in docs now covers the required description and folder fields.

🐛 Fixed
- Docs pages with scrollbar-styled wrappers no longer lose their tables and code blocks.
- Relative image and markdown links now resolve against the page's final URL after redirects.
- Media downloads no longer overwrite a sibling's file when two units claim the same slot.
- `issues/new` and similar GitHub links no longer die as bogus issue lookups.
- Multi-paragraph quotes and list items in X articles keep their formatting.
