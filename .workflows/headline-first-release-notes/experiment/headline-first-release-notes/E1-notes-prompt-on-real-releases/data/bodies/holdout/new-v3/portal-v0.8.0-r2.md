A redesigned picker with its own light/dark canvas, a new help screen on every page, and a cold-start loading screen that shows real progress.

💥 Breaking
- Navigation is now arrows only — vim keys (`j`/`k`/`g`/`G`/`h`/`l`), `PgUp`/`PgDn`/`Home`/`End` and uppercase bindings are gone; use `↑`/`↓` and `Ctrl+↑`/`Ctrl+↓` to page.
- Session kill moved from `K` to `k` and rename from `R` to `r`.
- The `p` key (Sessions to Projects) and the `s` key (Projects to Sessions) are removed; use `x` to switch between the two pages.
- In the scrollback preview, `[`/`]` no longer change window; use `←`/`→` to change window and `Tab` to change pane.
- The kill and delete confirmations no longer cancel on `n`; press `Esc` instead.
- Tags are now case-sensitive and stored exactly as typed, so `Work` and `work` are different tags.

✨ Added
- Redesigned TUI with Portal's own light or dark canvas, auto-detected from your terminal and restored when Portal exits.
- `appearance` setting in `prefs.json` (`auto`, `light`, `dark`) to pin the canvas and skip detection.
- `NO_COLOR` support — Portal paints no canvas and renders without colour.
- Press `?` on Sessions, Projects or the preview for a help panel listing that page's keys.
- Cold-start loading screen with a progress bar and live steps, including a restore counter such as `8 / 12`.
- A bootstrap failure on cold start now shows an in-app error screen, quit with `q` or `Esc`, and exits non-zero.
- Notice bands for transient warnings, the "No tags yet" hint and the pending-command banner.
- Dedicated empty-state screens for no sessions, no projects and no filter matches.
- The preview is now a full-screen cyan "peek mode" frame with window and pane counters and a footer of key hints.

🔧 Changed
- Starting Portal on a fresh tmux server now opens the picker immediately while restore runs in the background.
- Startup warnings on a cold start now appear as a banner in the picker instead of printing to stderr.
- The project edit modal saves every change immediately, with navigate and edit modes, bordered fields and chips for aliases and tags.
- Kill, delete and rename confirmations were restyled; delete now states that only Portal's record is removed.
- Sessions and Projects pages were restyled with a new header, section headers, a condensed footer and a persistent `/ to filter` hint.

🐛 Fixed
- The session list now refreshes after a cold-start restore instead of showing an empty list.
