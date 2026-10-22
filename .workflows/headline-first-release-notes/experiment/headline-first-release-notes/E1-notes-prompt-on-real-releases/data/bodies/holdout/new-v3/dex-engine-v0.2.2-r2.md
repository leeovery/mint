✨ Added
- X posts with a video are now transcribed — the transcript joins the post's text instead of the video being kept as a file to describe.
- X long-form articles keep their code listings, link targets, dividers and embedded posts, and their figures and cover are downloaded.
- GitHub directory links give that directory's README and file listing, and `raw` and `tree` links to a file read the file.
- GitHub release assets, source archives and `user-attachments` links that are documents are read as files.
- Web pages that declare the markdown they were rendered from are stored from that markdown, keeping tables and code blocks.
- Math is kept as TeX in extracted pages and arXiv papers, with equation tables and span-typeset tables rebuilt.
- Session captures get a random suffix in the file name (`yyyyMMdd-HHmmss-a3f9`) so two machines never collide.
- Four migrations re-read already-stored GitHub links, X video posts, X articles and article-seam pages with the fixed extraction.

🔧 Changed
- A rerun whose re-fetch fails or comes back thin now keeps the stored copy instead of replacing it with a dead or parked entry.
- A rerun that parks for transcription again keeps the stored transcript under the re-fetched notes.
- Items stay `enriched` while a rerun drains over a landing that still stands, instead of flipping back to `raw`.
- `dex-enrich compact` keeps a requeued unit's last landing so the stored file stays identifiable.
- Staged media is filed under its asset id rather than its file name, so repeated names like `screenshot.png` no longer share a directory.
- Links to GitHub pages with no route (commits, release pages, wikis) now park naming their shape instead of storing the repo's root README.

🐛 Fixed
- GitHub links to a directory, a branch root, a raw file or an issue form were stored as the repo's root README or failed as dead; they now read what they point at.
- Docs pages lost their tables and code blocks to scroll-area wrappers during extraction.
- Math values were deleted from arXiv text, leaving gaps like "an average gain of  points".
- Table-only appendices in arXiv papers vanished from the stored text.
- Relative image and markdown links on redirected pages resolved against the wrong URL.
- Media downloads no longer overwrite a sibling's file when two units claim the same slot.
