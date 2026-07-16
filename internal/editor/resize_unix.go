//go:build !windows

package editor

import (
	"os"
	"os/signal"
	"syscall"
)

func watchResize(done <-chan struct{}) <-chan struct{} {
	out := make(chan struct{}, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	go func() {
		defer signal.Stop(sig)
		for {
			select {
			case <-done:
				close(out)
				return
			case <-sig:
				select {
				case out <- struct{}{}:
				default:
				}
			}
		}
	}()
	return out
}
