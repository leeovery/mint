# The look every tape shares, sourced first: agntc's, which is Switchboard's,
# in Lee's Ghostty theme, at 100 × 28. mint lays its output out at most 100 columns wide, so at 100 it
# reaches the right edge as it would in any wider terminal. A still sets a
# height of its own after it, for what it shows: 37 px a row, and 64 more.
#
# 100 columns at 28px are 1600 px, twice the 800 the README shows a frame's
# screen at; with finalize.sh's margin the tour is 1680, twice 840.

Set Shell "bash"
Set FontFamily "JetBrains Mono"
Set FontSize 28
Set LetterSpacing 0
Set LineHeight 1
# 100 × 28 cells of 16 × 37 px: the frames' size, 1600 × 1036.
Set Width 1672
Set Height 1100
Set Padding 16
Set CursorBlink false
Set Framerate 30
Set TypingSpeed 45ms
# Lee's Ghostty theme, Nordfox, from Ghostty's own themes/Nordfox. The cursor
# takes the background, as agntc's does, so the stills show no block.
Set Theme { "name": "Nordfox", "background": "#2e3440", "foreground": "#cdcecf", "cursor": "#2e3440", "selection": "#3e4a5b", "black": "#3b4252", "red": "#bf616a", "green": "#a3be8c", "yellow": "#ebcb8b", "blue": "#81a1c1", "magenta": "#b48ead", "cyan": "#88c0d0", "white": "#e5e9f0", "brightBlack": "#53648d", "brightRed": "#d06f79", "brightGreen": "#b1d196", "brightYellow": "#f0d399", "brightBlue": "#8cafd2", "brightMagenta": "#c895bf", "brightCyan": "#93ccdc", "brightWhite": "#e7ecf4" }
# The world's prompt, as Wait looks for it on the cursor's line.
Set WaitPattern /❯/
Set WaitTimeout 1m
