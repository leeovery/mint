✨ Added
- Multi-select mode in the session picker — press `m` to mark several sessions, then Enter to open them all, each in its own host terminal window.
- New `portal spawn` command opens one or more sessions in separate host-terminal windows, with `--detect` to print the detected terminal.
- Host terminal detection, with native Ghostty window spawning on macOS.
- Custom terminal support via `terminals.json` — define your own argv or script recipe to open windows in other terminals.
- Banners for an unsupported or non-local terminal, a live `Opening n/N…` progress indicator, and Esc/Ctrl-C to cancel a burst in progress.
- Multi-select footer and help entry showing the new mode's keys.

🔧 Changed
- Multi-select disables kill, rename, new-session and page-toggle keys so they can't discard a marked set.
- Opening several sessions is all-or-nothing up front — if any marked session has disappeared, nothing opens and the gone sessions are flagged and removed from the selection.
- If some windows fail to open, the ones that did open are left in place and unmarked, so pressing Enter again retries only the missing ones.

🐛 Fixed
- Narrow terminals now truncate the footer on one line with an ellipsis instead of wrapping.
