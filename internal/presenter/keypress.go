package presenter

import (
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"golang.org/x/term"
)

// errNoRawMode is readKeypress's report that the terminal would not go into raw mode,
// so the caller reads a line instead.
var errNoRawMode = errors.New("the terminal would not go into raw mode")

// singleKeyGate reports whether every key g declares is one character, so a single
// keypress can name each choice.
func singleKeyGate(g Gate) bool {
	for _, key := range g.Keys() {
		if utf8.RuneCountInString(string(key)) != 1 {
			return false
		}
	}
	return true
}

// rawKeyFile reports whether in can host the single-keypress read: a real
// *os.File backed by a terminal. Anything else (the scripted readers tests
// inject, a pipe) falls back to the shared line-read loop.
func rawKeyFile(in io.Reader) (*os.File, bool) {
	f, ok := in.(*os.File)
	if !ok {
		return nil, false
	}
	return f, term.IsTerminal(int(f.Fd()))
}

// readKeypress reads g's answer as a single keypress from the terminal f, in raw mode,
// with no Enter needed, and puts the terminal back as it was before it returns, so
// the caller can write again. It returns errNoRawMode, having read nothing, when raw
// mode can't be entered.
//
// It reads a small chunk, not a single byte, so a bare Escape (a lone 0x1b) is told
// apart from an escape sequence (an arrow or function key sends 0x1b and more bytes in
// the same read): in raw mode (VMIN=1) Read blocks for one byte at least and returns
// a whole sequence in one call on any real terminal.
func readKeypress(f *os.File, g Gate) (Choice, error) {
	fd := int(f.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return "", errNoRawMode
	}
	defer func() { _ = term.Restore(fd, old) }()
	buf := make([]byte, 8)
	for {
		n, rerr := f.Read(buf)
		if rerr != nil || n == 0 {
			return "", ErrInputClosed
		}
		if choice, answered, err := keyChoice(buf[:n], g); answered || err != nil {
			return choice, err
		}
	}
}

// keyChoice is what one read of keys answers g with: Enter (CR or LF) and Space
// the gate's default; a lone Escape its decline, where it has one; Ctrl-C and
// Ctrl-D ErrInputClosed, a deliberate stop and never a default; a declared key its
// choice. Anything else, an escape sequence included, answers nothing and the read
// goes on.
func keyChoice(keys []byte, g Gate) (Choice, bool, error) {
	switch b := keys[0]; {
	case b == 0x1b && len(keys) == 1:
		no, ok := declineChoice(g)
		return no, ok, nil
	case b == 0x1b:
		return "", false, nil
	case b == 0x03 || b == 0x04:
		return "", false, ErrInputClosed
	case b == '\r' || b == '\n' || b == ' ':
		return g.Default, true, nil
	}
	choice, ok := parseChoice(string(rune(keys[0])), g)
	return choice, ok, nil
}

// declineChoice returns the gate's decline choice (ChoiceNo) when the gate declares
// it, so the Escape key can map to the same "abort" the "n" key triggers. A gate
// without a ChoiceNo (an enumerated source/target gate) has no decline, so Escape is
// ignored there.
func declineChoice(g Gate) (Choice, bool) {
	if g.Has(ChoiceNo) {
		return ChoiceNo, true
	}
	return "", false
}
