package lsp

import (
	"os"
	"testing"
	"time"
)

func TestServe_EOFExitsGracefully(t *testing.T) {
	// Serve reads from stdin; if stdin is closed immediately it should return
	// without panic.
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Close the write end so reader gets EOF immediately.
	w.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		Serve()
	}()

	// Should complete quickly; if it hangs, something is wrong.
	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Error("Serve did not exit on EOF within 2s")
	}
}
