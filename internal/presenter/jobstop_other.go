//go:build !unix

package presenter

import "os"

// stopSignals is no signal where the shell stops no process.
func stopSignals() (<-chan os.Signal, func()) { return nil, nil }
