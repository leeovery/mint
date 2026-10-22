Open several sessions at once in separate host-terminal windows from the picker or the new `portal spawn` command.

✨ Added
- Multi-select mode in the session picker — press `m` to mark sessions, Enter to open them all, each in its own terminal window, with the last one opening in the current window.
- `portal spawn <sessions...>` command opens the given sessions in host-terminal windows, and `portal spawn --detect` prints which terminal Portal detects.
- Ghostty window opening is supported natively on macOS, with a permission hint if macOS Automation access is blocked.
- `terminals.json` lets you add support for other terminals with a custom argv or script recipe, matched by name, bundle id or glob.
- Picker banner warns up front when your terminal can't open extra windows, or when there is no host-local terminal (such as over SSH or mosh).
- An `Opening n/N…` progress band shows while windows are being opened, and Ctrl-C or Esc cancels while leaving already-opened windows in place.
- If a marked session disappears before you press Enter, nothing opens, the missing session is flagged, and the rest stay marked for a retry.
- If some windows fail to open, the ones that did open are unmarked and the failed ones stay marked so Enter retries only those.
