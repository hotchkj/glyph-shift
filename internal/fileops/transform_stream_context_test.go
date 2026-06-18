package fileops

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

// oneByteAtATimeReader returns at most one byte per Read so bufio cannot
// batch past a byte-accurate cancellation boundary.
type oneByteAtATimeReader struct {
	r         *bytes.Reader
	delim     int
	cancel    context.CancelFunc
	delivered int
}

func (o *oneByteAtATimeReader) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}

	if o.delivered >= o.delim {
		o.cancel()
	}

	b, err := o.r.ReadByte()
	if err != nil {
		return 0, err
	}

	buffer[0] = b
	o.delivered++

	return 1, nil
}

func newlineTerminatedLines(lineCount int) []byte {
	if lineCount <= 0 {
		return nil
	}

	var sb strings.Builder
	for range lineCount {
		sb.WriteString("a\n")
	}

	return []byte(sb.String())
}

// TestRunTransformStream_ContextCanceledMidStream verifies cancellation is
// observed on periodic checks while scanning lines (same cadence as
// [contextCheckInterval]), not only at entry. cancel fires after byte
// 2*(2*contextCheckInterval-1), i.e. once line (2*contextCheckInterval-1) has been fully
// delivered and before line 2*contextCheckInterval is read, so the check
// immediately after line 2*contextCheckInterval completes sees ctx canceled.
func TestRunTransformStream_ContextCanceledMidStream(t *testing.T) {
	t.Parallel()

	linesBeforeCancel := 2*contextCheckInterval - 1
	const extraLines = 500
	src := newlineTerminatedLines(linesBeforeCancel + extraLines)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Each logical line is "a\n" (2 bytes). After this many bytes, the next
	// Read is the first byte of line (linesBeforeCancel + 1).
	delim := linesBeforeCancel * 2

	wrapped := &oneByteAtATimeReader{
		r:      bytes.NewReader(src),
		delim:  delim,
		cancel: cancel,
	}

	want := TargetLF
	opts := TransformOptions{LineEndings: &want}

	var out bytes.Buffer
	_, err := runTransformStream(ctx, wrapped, opts, &out, memWhitespaceSpillForTests(&out, opts))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
