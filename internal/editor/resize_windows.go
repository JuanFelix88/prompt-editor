//go:build windows

package editor

import "time"

func watchResize(done <-chan struct{}) <-chan struct{} {
	out := make(chan struct{}, 1)
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		defer close(out)
		for {
			select {
			case <-done:
				return
			case <-t.C:
				select {
				case out <- struct{}{}:
				default:
				}
			}
		}
	}()
	return out
}
