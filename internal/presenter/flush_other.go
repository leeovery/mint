//go:build !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !linux && !windows

package presenter

import "os"

// flushInput discards nothing where the platform gives no way to.
func flushInput(*os.File) {}
