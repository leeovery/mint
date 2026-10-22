✨ Added
- X videos are transcribed — a post's video is heard and its transcript joins the post, instead of the clip being kept as a file to describe.
- X articles keep their code listings, figures, embedded posts, dividers and link targets, and the cover and figures are saved beside the post.
- GitHub directory links read that directory's README and file listing, and links naming a branch alone read the repo at that branch.
- GitHub raw links, documents attached to issues, and released document downloads are read as files.
- Web pages that declare the markdown they were rendered from are stored from that markdown, keeping tables and code blocks.
- Captures written from a session or `dex` are named with a random suffix (`20260917-082604-a3f9`) so same-second captures never collide.

🔧 Changed
- GitHub links that point at no readable content (a commit, a release page, a wiki) now park for judgment instead of storing the repo's root README.
- A re-fetch that fails or returns a stub no longer replaces content already stored — the stored copy is kept and the report says so.
- Items whose re-fetch is still pending over a stored copy stay `enriched` instead of flipping back to `raw`.
- `dex-enrich compact` keeps the last output-bearing line for a requeued unit, so its stored file is still recognised.
- The Claude desktop scheduled-task setup guide now covers the required description and the folder control on the New-routine form.

🐛 Fixed
- Web pages lose fewer tables and code blocks — scroll-area wrappers (Mintlify docs) are no longer discarded as navigation.
- Math in pages and arXiv papers is kept as TeX instead of vanishing and leaving gaps in the prose, and equation tables and span-based tables are preserved.
- Relative image links on redirected pages now resolve against the page they landed on.
- GitHub `user-attachments` links, directory blob links and `issues/new` links no longer end up dead or blocked.
- Media files staged under a repeated file name no longer share a directory and overwrite each other's bytes.
- Media downloads no longer overwrite a sibling's file when slots were counted by ledger position.
- A rerun that parks for transcription no longer replaces an already-stored transcript.
- Existing instances are healed by new migrations that re-read misread GitHub links, rerun X posts with unheard videos or lossy articles, and re-extract pages stored before tables and math were kept.
