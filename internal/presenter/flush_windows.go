package presenter

import (
	"os"

	"golang.org/x/sys/windows"
)

// flushInput discards the input the console f has queued and nothing has read.
func flushInput(f *os.File) {
	_ = windows.FlushConsoleInputBuffer(windows.Handle(f.Fd()))
}
