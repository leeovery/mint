Every dex now reads what you share through a lens you write, instead of filtering shares against a scope list.

💥 Breaking
- CLAUDE.md is now engine-owned and identical in every instance — what an instance reads for moves to `lens.md`, and a directive migrates your old CLAUDE.md content on the next run, so don't hand-edit CLAUDE.md afterward.
- Content commands (`dex-inbox`, `dex-normalize`, `dex-enrich`, `dex-exclude`) now refuse while directives are pending — run `bin/dex directive list` and follow it first.
- The scope check at the door is gone — every capture becomes an item, and only content that yields nothing through the lens is dropped afterward.

✨ Added
- `lens.md`, a free-form note of what a dex reads for — digests, topics and wiki pages are all written from that angle, and a dex without one is a general knowledge dex.
- Directives and `dex-directive` (`list`, `show <n>`, `done <n>`) — judgment-based upgrades the run performs after the pull, recorded only once their check passes.
- Existing instances are upgraded automatically: one directive moves your scope into `lens.md` and Discord facts into config, another rewrites the README from the template.
- A `discord` key in `state/config.json` holds the server and channels a Discord pull exports.
- Lint and the sync report show a Lens note when `lens.md` still holds the seed's placeholders or can't be read.
- The sync report lists pending directives.

🔧 Changed
- `dex-exclude` now also removes the dropped item's media and pass records, and keeps media that another item still lists.
- Connected chats show each instance's lens, and saving from a chat goes where you say, asking which instance when several are connected.
- `dex-new` seeds `lens.md` and a README named for the instance, and records all shipped directives as done.
- Sync overwrites CLAUDE.md only once git history holds the current copy, so an uncommitted edit is never lost.
- The describe-these report now names the `bin/dex enrich item describe` command to run.
- The setup guide asks what the dex should read for instead of interviewing for a scope list.

🐛 Fixed
- The inbox release check reads the stored `origin` URL, so machines that rewrite GitHub URLs no longer lose the check.
