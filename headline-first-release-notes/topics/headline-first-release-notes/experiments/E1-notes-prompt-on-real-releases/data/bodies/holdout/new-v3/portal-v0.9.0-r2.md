Open several sessions at once, each in its own host-terminal window, from the picker or the command line.

✨ Added
- Multi-select mode in the session picker — press `m` to mark sessions, Enter to open them all, each in its own terminal window, with the last one attaching in the current window.
- `portal spawn <sessions...>` opens the given sessions in separate host-terminal windows, confirming each one came up before attaching the last.
- `portal spawn --detect` prints the host terminal Portal detects, without opening anything.
- Ghostty on macOS is supported natively for opening windows.
- `terminals.json` lets you define your own window-opening command (argv template or script) for other terminals, matched by app name, bundle id or glob.
- Picker banner warns up front when your terminal can't open extra windows (unsupported terminal or no host-local terminal).
- Opening progress (`Opening n/N…`) shows while windows are being created, and Ctrl-C or Esc cancels while leaving already-opened windows in place.
- Multi-select footer and a help-modal entry for the new `m` key.

🔧 Changed
- If a window fails to open, the ones that did open stay open and are unmarked, so pressing Enter again retries only the missing ones.
- If a selected session disappears before opening, nothing opens and a red banner names it — the remaining marked sessions stay selected.
- Space preview keeps your marked sessions, dropping any killed in the meantime.
- Row actions `k`, `r`, `n` and `x` are disabled while in multi-select mode.
