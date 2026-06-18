package cmd

import (
	"bytes"
	"testing"
)

func TestNopWriteCloserCloseReturnsNil(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	w := nopWriteCloser{Writer: &out}
	if _, err := w.Write([]byte("ok")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if out.String() != "ok" {
		t.Fatalf("output = %q, want ok", out.String())
	}
}
