//go:build windows

package editor

import "os"

func terminationSignals() []os.Signal { return []os.Signal{os.Interrupt} }
