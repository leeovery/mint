✨ Added
- Modern Vivid TUI reskin — a colourful picker on its own mode-matched light/dark canvas, with restyled header, section headers, footer, modals, and loading screen.
- Light/dark canvas auto-detection via OSC 11, or pin it with `"appearance": "auto" | "light" | "dark"` in `prefs.json`; `NO_COLOR` renders without any canvas.
- Per-page `?` help modal listing the page's full keymap, including an overlay version on the scrollback preview.
- Honest cold-boot loading screen — the tmux server starts and sessions restore while the TUI is already running, showing live step progress and a "Restoring sessions N / M" counter.
- In-TUI fatal error frame when startup fails, and restore warnings now appear as a notice band after the picker loads instead of being flushed to stderr.
- Notice bands for inline flashes, the "No tags yet" signpost, and a command-pending banner on the Projects page.
- Centred empty states for no sessions, no projects, and no filter matches, each with a tailored footer.
- Contextual filter footers and a locked orange `/ query` header while a filter is applied.
- Offline `capturetool` visual-capture harness and `vhs` tapes for re-capturing reference screenshots of each screen.

🔧 Changed
- Navigation is now arrows only (`↑`/`↓`, `Ctrl+↑`/`Ctrl+↓` to page) — vim aliases, `PgUp`/`PgDn`/`Home`/`End` and uppercase bindings are gone.
- `k` kills a session, `d` deletes a project, and `x` toggles between Sessions and Projects in both directions; `p` and `s` no longer switch pages.
- Kill and delete confirmations are dismissed with `Esc` only; `n` no longer cancels.
- The edit-project modal has navigate and edit modes, and every Name, Alias and Tag change now saves immediately with no confirm step.
- Preview navigation is now `←`/`→` for windows and `Tab` for panes, in a full-screen cyan "peek mode" frame with `Window x/y · Pane x/y` counters.
- Tags are now case-sensitive and stored exactly as typed.
- Modals clear the page to the canvas and show a centred panel rather than overlaying the list.
- Upgraded to Bubble Tea v2, Lipgloss v2 and Bubbles v2.
- The README was refreshed with screenshots and a new Getting Started section.

🐛 Fixed
- Cold-boot picker no longer shows a stale, empty session list — sessions are re-enumerated after restore completes.
- Terminal background is restored on exit, including on terminals that ignore the OSC 111 reset, without re-sticking the canvas colour.
