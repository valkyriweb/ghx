package main

import (
	"io"
	"os"
	"os/exec"
	"sync"
)

// watchDiagLimit is the stderr tail kept for the retry decision. A transport
// error is one short line, usually printed last. The rest of a long watch must
// not accumulate in memory.
const watchDiagLimit = 4096

// prepareWatchCmd leaves stdout on the caller's terminal so `gh pr checks
// --watch` can redraw. Stderr is forwarded live. A bounded tail of stderr is
// the only text the retry decision may read.
func prepareWatchCmd(path string, args []string) (*exec.Cmd, func() []byte) {
	cmd := exec.Command(path, args...)
	diag := &tailBuffer{max: watchDiagLimit}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, diag)
	return cmd, diag.Bytes
}

// tailBuffer keeps the last max bytes written. Write is safe for one copier;
// Bytes may run after that copier has finished.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.max:]...)
	}
	return len(p), nil
}

func (t *tailBuffer) Bytes() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]byte(nil), t.buf...)
}
