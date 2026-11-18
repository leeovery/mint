A redesigned, colourful picker that paints its own light or dark canvas and opens straight onto an honest loading screen on a fresh boot.

💥 Breaking
- Navigation is now arrows only — vim keys (`j`/`k`/`h`/`l`/`g`/`G`), `PgUp`/`PgDn`/`Home`/`End` and uppercase bindings are gone; use `↑`/`↓` and `Ctrl+↑`/`Ctrl+↓` to page.
- Key bindings changed: `k` now kills a session, `d` deletes a project, `x` is the only Sessions⟷Projects toggle, and the old `p` (Sessions) and `s` (Projects) page-switch keys are removed.
- Preview navigation changed — `←`/`→` switch windows, `Tab` cycles panes, and `]`/`[` no longer work.
- Tags are now case-sensitive and stored exactly as typed, so `Work` and `work` become separate tags.

✨ Added
- Modern Vivid look for the whole picker, with a new header, section headers, condensed footer and an owned light/dark canvas.
- Light or dark canvas is auto-detected from your terminal, or pinned with `"appearance": "light"|"dark"` in `prefs.json`; `NO_COLOR` gives a colourless render.
- Press `?` on any page for a keymap help modal, including over the scrollback preview.
- Cold-boot loading screen shows real step-by-step progress, including a "Restoring sessions N / M" counter.
- A fatal startup error now appears in the loading screen with a quit hint, and Portal exits non-zero.
- Notice bands for the "No tags yet" signpost, the command-pending banner and flash messages.
- Centred empty states for no sessions, no projects, and no filter matches, each with its own footer.
- Distinct filter footers for typing a filter and for browsing filtered results.
- Terminal background is restored on exit, so the canvas doesn't stick in terminals that ignore the reset.

🔧 Changed
- On a fresh tmux server the TUI now opens immediately while startup runs in the background; the warm path and the CLI are unchanged.
- Startup warnings on a fresh boot now show as an in-app notice band after the picker appears, instead of printing to stderr.
- The edit-project modal now saves every change immediately, with navigate and edit modes and chip fields for aliases and tags.
- Kill, delete and rename confirmations are redesigned, and cancelling now takes `Esc` only (`n` no longer cancels).
- Scrollback preview has a new cyan "peek mode" frame with `Window x/y · Pane x/y` counters and a hints footer.
- Session and project lists have new row styling with a full-height selection bar.

🐛 Fixed
- After a fresh-boot restore, the session list now refreshes so it shows the restored sessions rather than an empty snapshot.
