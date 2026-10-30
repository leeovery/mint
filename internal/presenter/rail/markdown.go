package rail

import "strings"

// The Markdown a release notes body is drawn in, for display only: what ships, the
// editor's text and plain keep the body byte for byte. It is line by line and
// bounded, with no library: an ATX heading loses its markers and draws bold, a list
// item's marker draws as a bullet, inline code draws in the accent and **bold** draws
// bold, each losing its markers; everything else, links included, is drawn as
// written.

const (
	// maxHeading is the deepest ATX heading, ######.
	maxHeading = 6
	// maxIndent is the most spaces a heading's markers may stand in by.
	maxIndent = 3
	bullet    = "•"
)

// markdown is a body line, already shown, as its Markdown reads.
func markdown(line string) []span {
	if text, ok := heading(line); ok {
		return inline(text, bold)
	}
	if indent, text, ok := listItem(line); ok {
		return append([]span{s(indent+bullet+" ", plain)}, inline(text, plain)...)
	}
	return inline(line, plain)
}

// heading is line's text where line is an ATX heading: up to three spaces, one to
// six #, then a space or the line's end.
func heading(line string) (string, bool) {
	rest := strings.TrimLeft(line, " ")
	if len(line)-len(rest) > maxIndent {
		return "", false
	}
	text := strings.TrimLeft(rest, "#")
	level := len(rest) - len(text)
	if level == 0 || level > maxHeading || text != "" && text[0] != ' ' {
		return "", false
	}
	return strings.TrimSpace(text), true
}

// listItem is line's indent and text where line is a list item: its marker, -, * or
// +, then a space.
func listItem(line string) (indent, text string, ok bool) {
	rest := strings.TrimLeft(line, " ")
	if len(rest) < 2 || !strings.ContainsRune("-*+", rune(rest[0])) || rest[1] != ' ' {
		return "", "", false
	}
	return line[:len(line)-len(rest)], rest[2:], true
}

// inline is text in base with its inline code in the accent and its **bold** bold,
// their markers gone. A marker with no partner, or around nothing, is text.
func inline(text string, base style) []span {
	var out []span
	plainRun := strings.Builder{}
	flush := func() {
		if plainRun.Len() > 0 {
			out = append(out, s(plainRun.String(), base))
			plainRun.Reset()
		}
	}
	for text != "" {
		marker, st := spanMarker(text, base)
		if marker != "" {
			if inner, after, ok := strings.Cut(text[len(marker):], marker); ok && inner != "" {
				flush()
				out = append(out, s(inner, st))
				text = after
				continue
			}
		}
		plainRun.WriteByte(text[0])
		text = text[1:]
	}
	flush()
	return out
}

// spanMarker is the inline marker text opens with, ` or **, and the style what it
// marks draws in over base; "" where it opens with neither.
func spanMarker(text string, base style) (string, style) {
	switch {
	case strings.HasPrefix(text, "`"):
		return "`", style{ink: inkAccent, bold: base.bold}
	case strings.HasPrefix(text, "**"):
		return "**", style{ink: base.ink, bold: true}
	}
	return "", base
}
