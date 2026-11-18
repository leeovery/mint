✨ Added
- X posts with video are now transcribed — the transcript joins the post's text, and the video is no longer stored as a file to describe.
- X long-form articles keep their code listings, link targets, dividers and embedded posts, and their figures and cover are downloaded in reading order.
- GitHub directory links read the directory's README and file listing, and links to a ref's root read the repo at that ref.
- GitHub raw links, release-asset and archive downloads, and `user-attachments` links are now handled — documents become file work and other downloads park with a clear reason.
- Web pages that declare a markdown source (`rel="alternate" type="text/markdown"`) are stored from that markdown, keeping tables and code blocks.
- Math is preserved in fetched pages and arXiv papers as TeX, and LaTeXML equation tables and span-typeset tables come through as real content.
- Four new migrations (15–18) requeue items stored by the old behaviour so they are re-read with the fixed routes.

🔧 Changed
- Capture files now carry a random suffix (`20260917-082604-a3f9.md`) — same-second captures from different machines no longer conflict.
- Media captured through the inbox is filed by its asset rather than its file name — repeated names like `screenshot.png` no longer overwrite each other.
- Reruns that fail to re-fetch keep the stored copy — a rotted source no longer replaces good content with a dead entry.
- Reruns keep an existing transcript and wrap the re-fetched notes around it instead of parking over it.
- Items whose rerun still has its earlier landing on disk stay `enriched` instead of flipping back to `raw`.
- `dex-enrich compact` keeps the last output-bearing line for a requeued unit.
- Scheduled-task setup docs now cover the required description and folder control in the desktop New-routine form.

🐛 Fixed
- GitHub links to commits, release pages, wikis and other unsupported shapes no longer store the repo's root README as if it were the linked page — they park for judgment.
- `issues/new` links are no longer treated as issue "new" and marked dead.
- Media downloads no longer overwrite a sibling's file when two units compete for the same slot.
- Docs pages with scrollbar-styled wrappers no longer lose their tables and code blocks.
- Relative image URLs on redirected pages now resolve against the page's final address.
- Posts' video renditions are matched by video ID, so the right downloaded file is retired when a transcript lands.
