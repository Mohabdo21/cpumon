package main

import (
	"context"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestWatchQuitStopDoesNotWaitForBlockedStdinRead(t *testing.T) {
	// Pipe2 gives us blocking FDs like a TTY; os.Pipe uses the runtime poller.
	var fds [2]int
	if err := syscall.Pipe2(fds[:], syscall.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	stdin := os.NewFile(uintptr(fds[0]), "stdin")
	writer := os.NewFile(uintptr(fds[1]), "stdin-writer")
	orig := os.Stdin
	os.Stdin = stdin
	t.Cleanup(func() {
		os.Stdin = orig
		_ = writer.Close() // EOF releases the key reader
		_ = stdin.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	stop := watchQuit(ctx, cancel)

	if _, err := writer.Write([]byte{'x'}); err != nil {
		t.Fatal(err)
	}
	waitPipeDrained(t, fds[0])
	cancel()

	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("stop is still waiting for the blocked stdin read")
	}
}

func waitPipeDrained(t *testing.T, fd int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var readable syscall.FdSet
		readable.Bits[fd/64] |= 1 << (fd % 64)
		n, err := syscall.Select(fd+1, &readable, nil, nil, &syscall.Timeval{})
		if err == nil && n == 0 {
			return
		}
		if err != nil && err != syscall.EINTR {
			t.Fatal(err)
		}
		runtime.Gosched()
	}
	t.Fatal("stdin reader never consumed the key")
}
