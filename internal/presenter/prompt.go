package presenter

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// ErrInputClosed is the EXPORTED sentinel the input seam (Prompt's shared
// read-and-loop core, and AskLine) returns when the input stream hits EOF with no
// usable line. It is deliberately NON-NIL and distinct from a declared Choice so
// the caller fails loud rather than silently default-accepting — the underpinning
// of the fail-loud / unattended-without-`-y` behaviour: an empty Enter selects the
// default, but a closed stream must NOT be mistaken for one.
//
// It is exported so the engine can branch on it via errors.Is, exactly like
// ErrNotInteractive. Unlike ErrNotInteractive, this path renders NOTHING through
// the presenter: the forbidden combination is a predictable startup state the
// presenter narrates itself, whereas EOF arrives mid-interaction where the ENGINE
// owns the failure semantics (abort, exit code, any closing narration) — the
// presenter only reports the closed stream through this error.
var ErrInputClosed = errors.New("prompt: input closed before a choice was entered")

// ErrNotInteractive is the EXPORTED sentinel both presenters return from Prompt on
// the forbidden combination — stdin is NOT a TTY and -y was NOT passed, so an
// interactive gate can be neither answered nor safely blocked on. It is exported
// precisely so the engine/main can map THIS path to a
// non-zero exit via errors.Is; the presenter itself sets no exit code. The
// failure is ALSO surfaced through the presenter as a rendered failure (styled in
// pretty, terse in plain) and the one-line summary goes to stderr — this sentinel
// is the machine-readable companion to that human-facing rendering. The message
// is the spec's ASCII form (a semicolon, never the em-dash) so the engine-facing
// string stays byte-pure; the pretty rendering uses the em-dash form separately.
var ErrNotInteractive = errors.New("stdin is not a TTY; pass -y to run unattended")

// parseChoice is the SHARED, mode-agnostic parse for one line of gate input. It is
// the single point that turns a raw input line into a declared Choice, used
// identically by both presenters so the parse can never drift between modes.
//
// Rules (all read from the gate's DECLARED set — nothing hardcodes y/n/e/r):
//   - The line is trimmed of surrounding whitespace. A whitespace-only line
//     therefore trims to empty and is treated exactly like a deliberate empty
//     Enter — ordinary CLI line-read behaviour.
//   - An empty (or whitespace-only) line returns g.Default, true — the empty-Enter
//     accept path. g.Default is always a member of the declared set.
//   - Otherwise the trimmed input is compared CASE-INSENSITIVELY (strings.EqualFold)
//     against each declared key, in declared order, and on a match the DECLARED key
//     is returned with true — the canonical key, never the raw input. Folding both
//     sides (rather than lower-casing the input and comparing verbatim) means a
//     gate that declares a mixed-case key is still selectable; nothing here
//     hardcodes a set or a casing convention.
//   - Any input that matches no declared key returns ("", false) so the caller
//     re-prompts; an unrecognised key is NEVER mapped to a choice and NEVER
//     silently accepted as the default.
func parseChoice(line string, g Gate) (Choice, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return g.Default, true
	}
	for _, key := range g.Keys() {
		if strings.EqualFold(trimmed, string(key)) {
			return key, true
		}
	}
	return "", false
}

// readChoice is the SHARED read-and-loop core both presenters drive from Prompt.
// Factoring it as a free function (taking the persistent buffered reader and a
// mode-specific render closure) keeps the parse/loop identical across modes —
// only the render differs.
//
// Each pass: render the prompt, read ONE line, and parse it. On a recognised line
// (including the empty-Enter default) the choice is returned. On an unrecognised
// line the loop RE-RENDERS and reads again — repeated garbage keeps re-prompting.
//
// EOF handling is the load-bearing safety property: bufio.Reader.ReadString
// returns the bytes read so far ALONGSIDE io.EOF, so a final line with no trailing
// newline ("y" then EOF) is still parsed. Only when EOF arrives with no usable
// line (an empty trailing read) does this return ErrInputClosed — never a silent
// default-accept. A genuine empty Enter ("\n") is a real line, not EOF, so it
// still selects the default.
func readChoice(reader *bufio.Reader, render func(), g Gate) (Choice, error) {
	for {
		render()
		line, err := reader.ReadString('\n')
		if line != "" {
			if choice, ok := parseChoice(line, g); ok {
				return choice, nil
			}
			// A non-empty but unrecognised line: re-prompt. If that line also
			// arrived with EOF, the next ReadString returns "" + io.EOF below.
			if err != nil {
				return "", ErrInputClosed
			}
			continue
		}
		// No bytes read. On EOF this is the closed-stream case (fail loud); any
		// other read error is likewise unusable input.
		if err != nil {
			return "", ErrInputClosed
		}
	}
}

// readLine is the SHARED free-text read core both presenters drive from AskLine.
// It reads ONE raw line from the persistent buffered reader and returns it with
// only the line terminator stripped — the trailing "\n" and any preceding "\r"
// (a CRLF terminal) — otherwise VERBATIM: leading, inner, and trailing spaces are
// preserved and the empty string is a legal result (a deliberate empty Enter).
//
// EOF handling mirrors readChoice: bufio.Reader.ReadString returns the bytes read
// so far ALONGSIDE io.EOF, so a final line with no trailing newline is still
// returned as the answer. Only when EOF (or any other read error) arrives with NO
// bytes does this return ErrInputClosed — a closed stream is never mistaken for a
// deliberate empty answer.
func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if line == "" && err != nil {
		return "", ErrInputClosed
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// plainKeyHint builds the slash-joined key hint (e.g. "y/n/e/r") from the gate's
// DECLARED keys in render order. It reads g.Keys() so a two-choice gate renders
// "y/n" — the hint is never a hardcoded y/n/e/r literal. Both the plain terse
// prompt and the minimal pretty prompt use it this phase (the full pretty vertical
// menu is task 3-4).
func plainKeyHint(g Gate) string {
	keys := g.Keys()
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = string(key)
	}
	return strings.Join(parts, "/")
}

// bufferedReader lazily wraps an io.Reader in a SINGLE persistent *bufio.Reader,
// memoising it on first use. The persistence is essential: bufio.Reader may read
// ahead past the current line into its buffer, so constructing a fresh wrapper per
// read would discard those buffered bytes and lose subsequent input across a
// re-prompt. One wrapper per presenter, reused for every read, preserves the
// buffered tail.
func bufferedReader(in io.Reader, cached **bufio.Reader) *bufio.Reader {
	if *cached == nil {
		*cached = bufio.NewReader(in)
	}
	return *cached
}
