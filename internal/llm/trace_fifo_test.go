package llm

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A trace file is created, never opened over what is there. Opened to
// truncate, a FIFO under its name waited for a reader, and the request
// it was to record never left (ADR-0050). A FIFO, or a file, already
// under the name is an error at once; a new name is a file of the
// trace's mode.
func TestTraceFileIsNotOpenedOverAFifo(t *testing.T) {
	dir := t.TempDir()
	fifo := fifoAt(t, filepath.Join(dir, "stamp-request.json"))
	var err error
	returnsAtOnce(t, fifo, func() { _, err = openTraceFile(fifo) })
	if !errors.Is(err, os.ErrExist) {
		t.Errorf("over a FIFO: %v, want an error that the name exists", err)
	}

	fresh := filepath.Join(dir, "stamp-response.sse")
	f, err := openTraceFile(fresh)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if st, err := os.Stat(fresh); err != nil || st.Mode().Perm() != traceFileMode {
		t.Errorf("a new trace file: %v, %v — want mode %o", st, err, traceFileMode)
	}
	if _, err := openTraceFile(fresh); !errors.Is(err, os.ErrExist) {
		t.Errorf("over a file already there: %v, want an error that the name exists", err)
	}
}

// fifoAt makes a FIFO at path, with its parent directories.
func fifoAt(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// returnsAtOnce runs fn and fails the test if it has not returned in
// two seconds. What waits on a FIFO — an open for reading or one for
// writing — is released by opening the FIFO at both ends, so the test
// binary can still exit.
func returnsAtOnce(t *testing.T, fifo string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
	}
	for {
		if rw, err := os.OpenFile(fifo, os.O_RDWR|syscall.O_NONBLOCK, 0); err == nil {
			_ = rw.Close()
		}
		select {
		case <-done:
			t.Fatal("it waited on a FIFO")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
