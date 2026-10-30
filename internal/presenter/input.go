package presenter

import (
	"bufio"
	"io"
	"os"
	"os/signal"
)

// Interrupts starts watching for the interrupt Ctrl-C sends, until stop is called.
type Interrupts func() (interrupted <-chan os.Signal, stop func())

// interruptSignals watches for os.Interrupt.
func interruptSignals() (<-chan os.Signal, func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	return c, func() { signal.Stop(c) }
}

// input is what a presenter's questions read their answers from: in, and the one
// buffered reader over it that every typed line shares, since a fresh one per read
// would drop what it read ahead.
type input struct {
	in     io.Reader
	reader *bufio.Reader
	// interrupts ends a typed line's read at Ctrl-C, where set: a terminal reading a
	// typed line sends Ctrl-C as SIGINT, not as a byte the read could see.
	interrupts Interrupts
	// abandoned is set once an interrupt has ended a read, which is left blocked on
	// in; every read from then on is closed, so nothing reads in alongside it.
	abandoned bool
}

// discardTypedAhead drops what was typed before a question is asked; see
// discardTypedAhead. Once a read is abandoned it touches nothing.
func (i *input) discardTypedAhead() {
	if !i.abandoned {
		discardTypedAhead(i.in, i.reader)
	}
}

// line reads one typed line as readTypedLine does. Ctrl-C ends the read with
// ErrInputClosed, as it ends a keypress read in raw mode.
func (i *input) line() (line string, ended bool, err error) {
	if i.abandoned {
		return "", false, ErrInputClosed
	}
	reader := bufferedReader(i.in, &i.reader)
	if i.interrupts == nil {
		return readTypedLine(reader)
	}
	interrupted, stop := i.interrupts()
	defer stop()
	type typed struct {
		line  string
		ended bool
		err   error
	}
	read := make(chan typed, 1)
	go func() {
		var t typed
		t.line, t.ended, t.err = readTypedLine(reader)
		read <- t
	}()
	select {
	case t := <-read:
		return t.line, t.ended, t.err
	case <-interrupted:
		// The read stays blocked on in until the process exits: the run aborts on the
		// closed input, and with abandoned set nothing reads in again.
		i.abandoned = true
		return "", false, ErrInputClosed
	}
}
