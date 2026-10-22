Every dex now reads what you share through a lens you write, instead of filtering shares against a scope list.

💥 Breaking
- CLAUDE.md is now engine-owned and identical in every instance, so scope written there moves to `lens.md` — the first run after upgrading does this for you through a directive, and `bin/dex sync` leaves a CLAUDE.md that git history does not hold alone until you commit it.
- Content commands (`dex-inbox`, `dex-normalize`, `dex-enrich`, `dex-exclude`) refuse while an engine directive is pending — run `bin/dex directive list` and follow it, then re-run.
- The scope check at the door is gone: every capture now becomes an item, and only content that yields nothing through the lens is dropped later.

✨ Added
- `lens.md`, a free-form note of what an instance reads for — it steers what facts are taken, how topics are split and what the wiki says, and an instance with no lens is a general knowledge dex.
- Engine directives (`dex-directive list`, `show`, `done`) for judgment work on an instance's own files, performed by the run after the pull and recorded only once each check passes.
- Two directives that bring older instances up to shape: one rehomes the old CLAUDE.md scope into `lens.md` and its Discord facts into config, the other rewrites README.md from the template.
- Discord server and channels now live in a validated `discord` key in `state/config.json`, and the run walks you through setting it up when it is missing.
- Lint and the sync report carry a Lens note when `lens.md` still holds the seed's placeholders or can't be read — informational, never a failure.
- `dex-new` seeds a `lens.md` and a README named for the instance, and records every shipped directive as done.

🔧 Changed
- Dropping an item at the lens verdict through `dex-exclude` now also removes its media files and pass records, not just its corpus file, enrichment, digest and ledger entries.
- Chat connections now present each instance's lens, and saving from a chat goes where you say, asking which instance when several are connected, never choosing from the content.
- Setup asks what a dex should read for instead of interviewing for a scope list.
- The describe queue in run reports now points at `bin/dex enrich item describe` for each missing description.
- The inbox release checks read the stored `origin` URL, so a git `insteadOf` rewrite no longer hides the GitHub repository.
