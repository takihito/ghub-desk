package debuglog

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestEnableDebugWithWriterRoutesOutput(t *testing.T) {
	var buf bytes.Buffer
	EnableDebugWithWriter(&buf)
	defer EnableDebugWithWriter(nil) // reset to stderr so other tests aren't affected

	Debugf("hello %s", "world")

	if got := buf.String(); !strings.Contains(got, "hello world") {
		t.Fatalf("expected output to contain %q, got %q", "hello world", got)
	}
}

func TestEnableDebugWithWriterNilFallsBackToStderr(t *testing.T) {
	defer debugLogger.SetOutput(io.Discard) // reset so other tests aren't affected by stderr output

	// Passing nil must not panic; it should fall back to os.Stderr internally.
	EnableDebugWithWriter(nil)
}
