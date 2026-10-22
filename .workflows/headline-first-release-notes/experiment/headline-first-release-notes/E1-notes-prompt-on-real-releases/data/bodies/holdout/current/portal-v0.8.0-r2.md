✨ Added
- Modern Vivid TUI reskin — a colourful picker that paints its own light or dark canvas across every screen.
- Light/dark auto-detection from your terminal background, with an `appearance` setting (`auto`, `light`, `dark`) in `prefs.json` to pin it.
- NO_COLOR support — set `NO_COLOR` to render without colour on your terminal's native background.
- Per-page `?` help modal listing the complete keymap for Sessions, Projects, and Preview.
- Honest cold-boot loading screen with a progress bar, step checklist, and a live "Restoring sessions N / M" counter.
- In-TUI error screen when a fatal startup step fails — quit with `q` or `Esc` for a non-zero exit.
- Inline notice bands for warnings, success messages, the "No tags yet" hint, and the pending-command banner.
- Friendly empty states for no sessions, no projects, and filters with no matches.
- Offline capture tool and `vhs` tapes for screenshotting each TUI screen from fixtures, without a tmux server.

🔧 Changed
- Bootstrap now runs concurrently with the TUI on a cold start, so the picker appears immediately instead of after startup finishes.
- Startup warnings on a cold start show as an in-TUI notice after the picker loads instead of being printed to stderr.
- Navigation is arrows only — vim keys (`h/j/k/l`, `g/G`), `PgUp/PgDn/Home/End`, and uppercase bindings are removed; page with `Ctrl+↑`/`Ctrl+↓`.
- `x` is now the single Sessions⟷Projects toggle; the `p` and `s` aliases are gone.
- Kill and delete confirmations cancel with `Esc` only, and the kill modal now shows the session's window count.
- Preview navigation uses `←`/`→` for windows and `Tab` for panes, replacing `[`/`]`.
- The project editor now saves every change immediately, including Name and Aliases, with separate navigate and edit modes.
- Session list pagination dots are restyled and centred.
- Upgraded to Bubble Tea v2, Lipgloss v2, and Bubbles v2.

🐛 Fixed
- Cold-boot session list no longer shows a stale empty snapshot — it refreshes after restore completes.
- Terminal background is restored correctly on exit, even when the terminal echoes the canvas colour back as its original.
- Alias chips in the project editor now appear in a stable alphabetical order.
