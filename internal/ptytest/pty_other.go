//go:build !darwin && !linux

package ptytest

import (
	"errors"
	"os"
)

func openPty() (*os.File, string, error) {
	return nil, "", errors.New("not opened on this platform")
}
