package presenter_test

import (
	"bytes"
	"fmt"
	"testing"
)

// assertBytePureASCII asserts the plain byte-purity contract over buf: no ESC
// (0x1b), no CR (0x0d), and nothing outside the printable ASCII range
// (0x20–0x7e) except newline. context names the producing site (e.g. "plain
// warn output") and is woven into every failure message alongside the byte
// offset and offending byte. This is the single definition of the scan that all
// plain byte-purity guards share.
func assertBytePureASCII(t *testing.T, buf *bytes.Buffer, context string) {
	t.Helper()

	for i, b := range buf.Bytes() {
		switch {
		case b == 0x1b:
			t.Errorf("%s: byte %d is ESC (0x1b) — ANSI escape leaked into %s", context, i, context)
		case b == 0x0d:
			t.Errorf("%s: byte %d is CR (0x0d) — carriage-return leaked into %s", context, i, context)
		case b == '\n':
			// the only permitted control byte: a line terminator
		case b < 0x20 || b > 0x7e:
			t.Errorf("%s: byte %d = 0x%02x is outside the printable ASCII range %s uses", context, i, b, context)
		}
	}
}

// assertBytePureASCIIStreams applies assertBytePureASCII to each buffer in bufs,
// suffixing context with the buffer's ordinal stream label ("out", "err", …) so
// a failure names which stream leaked. It mirrors the out+err call sites that
// assert both presenter streams are byte-pure.
func assertBytePureASCIIStreams(t *testing.T, context string, bufs ...*bytes.Buffer) {
	t.Helper()

	streamLabels := []string{"out", "err"}
	for i, buf := range bufs {
		label := fmt.Sprintf("stream %d", i)
		if i < len(streamLabels) {
			label = streamLabels[i]
		}
		assertBytePureASCII(t, buf, fmt.Sprintf("%s (%s)", context, label))
	}
}
