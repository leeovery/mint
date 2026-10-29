package rail

import (
	"strings"

	"github.com/muesli/termenv"
)

// Level is how much colour the terminal draws.
type Level int

const (
	// NoColour draws no escape at all, bold and dim included: the marks alone carry
	// the meaning.
	NoColour Level = iota
	// Colours16 is the theme's own sixteen colours; the accent is its cyan.
	Colours16
	// Colours256 is xterm-256, where colour 79 is the accent exactly.
	Colours256
	// TrueColour is 24-bit colour, the accent #5FD7AF.
	TrueColour
)

// LevelOf is the level a termenv profile draws at. Reading NO_COLOR, COLORTERM
// and TERM into a profile is termenv.EnvColorProfile's.
func LevelOf(p termenv.Profile) Level {
	switch p {
	case termenv.TrueColor:
		return TrueColour
	case termenv.ANSI256:
		return Colours256
	case termenv.ANSI:
		return Colours16
	default:
		return NoColour
	}
}

// ink is a span's foreground: the terminal's own, the accent, or a status colour
// the theme decides.
type ink int

const (
	inkDefault ink = iota
	inkAccent
	inkRed
	inkYellow
)

// style is how a span is drawn.
type style struct {
	ink  ink
	bold bool
	dim  bool
}

var (
	plain      = style{}
	bold       = style{bold: true}
	dim        = style{dim: true}
	accent     = style{ink: inkAccent}
	accentBold = style{ink: inkAccent, bold: true}
	red        = style{ink: inkRed}
	redBold    = style{ink: inkRed, bold: true}
	yellow     = style{ink: inkYellow}
)

// code is the SGR foreground code of i at l, as agntc's are: the status colours the
// theme's own at every level, the accent at the depth the terminal draws.
func (l Level) code(i ink) string {
	switch i {
	case inkRed:
		return "31"
	case inkYellow:
		return "33"
	}
	switch l {
	case TrueColour:
		return "38;2;95;215;175"
	case Colours256:
		return "38;5;79"
	default:
		return "36"
	}
}

// paint is text drawn in s at l. Each span closes with its own codes, never a full
// reset, as agntc's painters do: bold and dim share 22, every colour 39.
func (l Level) paint(text string, s style) string {
	if l == NoColour || s == plain {
		return text
	}
	var open, shut []string
	if s.bold {
		open = append(open, "1")
	}
	if s.dim {
		open = append(open, "2")
	}
	if s.bold || s.dim {
		shut = append(shut, "22")
	}
	if s.ink != inkDefault {
		open = append(open, l.code(s.ink))
		shut = append(shut, "39")
	}
	return "\x1b[" + strings.Join(open, ";") + "m" + text + "\x1b[" + strings.Join(shut, ";") + "m"
}
