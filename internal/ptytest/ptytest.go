// Package ptytest opens pseudo-terminals for tests that need a real terminal: a
// read that only a terminal answers in raw mode, input a terminal queues, a stream a
// program judges to be a terminal. It is test-only, never linked into the binary.
package ptytest

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Open is a pseudo-terminal pair, closed at the test's end: tty, the terminal a
// program under test reads and writes, and pty, the side the test types into and
// reads what was written from. Where the platform has no pseudo-terminals the test
// is skipped.
func Open(t *testing.T) (pty, tty *os.File) {
	t.Helper()
	pty, name, err := openPty()
	if err != nil {
		t.Skipf("no pseudo-terminal: %v", err)
	}
	tty, err = os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		_ = pty.Close()
		t.Fatalf("open the terminal %s: %v", name, err)
	}
	// The pty closes first: a terminal closed with output nobody has read, the echo
	// of what was typed, waits for it to drain.
	t.Cleanup(func() {
		_ = pty.Close()
		_ = tty.Close()
	})
	return pty, tty
}

// quiet is how long Written waits for more before it takes the terminal as done.
const quiet = 200 * time.Millisecond

// Written is everything written to the terminal so far, read from pty until nothing
// more comes for a moment. The terminal's own output processing applies, so a
// newline reads as "\r\n".
func Written(t *testing.T, pty *os.File) string {
	t.Helper()
	var b strings.Builder
	buf := make([]byte, 4096)
	for {
		if err := pty.SetReadDeadline(time.Now().Add(quiet)); err != nil {
			t.Fatalf("time the read of the pty: %v", err)
		}
		n, err := pty.Read(buf)
		b.Write(buf[:n])
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return b.String()
		}
		if err != nil {
			t.Fatalf("read the pty: %v", err)
		}
	}
}

// Type types keys at the terminal, queued for whatever reads it next.
func Type(t *testing.T, pty *os.File, keys string) {
	t.Helper()
	if _, err := pty.WriteString(keys); err != nil {
		t.Fatalf("type %q: %v", keys, err)
	}
}
