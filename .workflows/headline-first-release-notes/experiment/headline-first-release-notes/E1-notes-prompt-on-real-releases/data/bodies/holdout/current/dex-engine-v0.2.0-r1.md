✨ Added
- Instance lens — a `lens.md` states what a dex reads for, and every judgment (digests, topics, wiki pages) reads shared content through it; a dex with no lens is a general knowledge dex.
- Directives — judgment-based upgrade work the engine ships for the run to perform after the pull, via the new `dex-directive` command (`list`, `show`, `done`), each recorded only once its check passes.
- Directive 1 moves an owner-written CLAUDE.md into `lens.md` and the Discord facts into config; directive 2 rewrites the README from the template.
- Discord server and channels now live in a `discord` key in `state/config.json`, with a guided setup for the first pull.
- Lens note on lint and the sync report when `lens.md` still holds the seed's placeholders or can't be read — informational, never a failure.

🔧 Changed
- CLAUDE.md is now engine-owned and identical in every instance, synced from the template; sync leaves a copy git history doesn't hold alone and says so.
- Nothing is rejected at the door any more — every capture becomes an item, and only content that yields nothing through the lens is dropped after it lands.
- `dex-inbox`, `dex-normalize`, `dex-enrich` and `dex-exclude` refuse while a directive is pending and point to `dex-directive list`.
- `dex-exclude` now also removes a dropped item's media and pass records, keeping media another live item still lists.
- New instances are scaffolded with a named `README.md` and `lens.md`, and every shipped directive is recorded as done.
- Chat connection reads each instance's lens, and captures go where the owner says — the chat asks which dex when several are connected rather than choosing from content.
- Setup asks what the dex should read for instead of interviewing for a scope list.
- The inbox release check reads the stored `origin` URL, so `insteadOf` rewrites no longer hide the GitHub repo.
- The describe queue now points to `bin/dex enrich item describe` instead of hand-writing enrichment files.

🗑️ Removed
- The "In scope" lists in CLAUDE.md and README.md, and the scope-filter step at ingest — replaced by the lens.

🐛 Fixed
- Commands in setup and connect guides no longer break on shells that alias `ls`, `find` and similar tools.
