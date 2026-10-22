✨ Added
- Per-instance lens in `lens.md` — each dex now reads shared content for what you say it reads for, and a dex with no lens is a general knowledge dex that reads everything for its plain substance.
- Directives (`dex-directive list | show | done`) — judgment-based upgrade work the engine ships, performed by the run after sync and recorded only once its check passes.
- Directive 1 moves an owner-written CLAUDE.md into `lens.md` and the Discord server and channels into config, and directive 2 rewrites the README from the template.
- A `discord` key in `state/config.json` holding the server and channels a Discord pull exports, with guided setup when it is absent.
- Lens notes on lint and the sync report — flags a `lens.md` that still holds the seed's placeholders or can't be read, without ever failing the check.

🔧 Changed
- Nothing is rejected for its subject any more — every share becomes an item and is dropped only if it yields nothing through the lens, with the reason named in the run report.
- CLAUDE.md is now engine-owned and identical in every instance — sync overwrites it only once git history holds the previous copy, so an uncommitted edit is never lost.
- `dex-new` seeds a named `lens.md` and README and records every shipped directive as done, so new instances start fully current.
- `dex-inbox`, `dex-normalize`, `dex-enrich` and `dex-exclude` refuse to run while a directive is pending and point you at `dex-directive list`.
- `dex-exclude` now removes a dropped item's media and pass records along with its corpus file, enrichment, digest and ledger entries.
- `dex-serve` instructions carry each instance's lens, and a capture goes into the instance you name, with the chat asking which one when several are connected.
- The inbox release checks read the stored `origin` URL, so a `url.insteadOf` rewrite no longer hides a GitHub repository.
- Setup now asks what the dex should read for instead of interviewing you for a scope list.
- Instructions run commonly aliased tools such as `ls` and `find` past any shell alias.

🐛 Fixed
- The "Describe these" report now names the exact `bin/dex enrich item describe` command to run instead of a file path to hand-write.

🗑️ Removed
- The "In scope" list in CLAUDE.md and README, and the owner's scope check at the door, replaced by the lens.
