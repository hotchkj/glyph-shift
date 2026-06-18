// perf_tail_write_publish_test.go covers tail-guard source path policy and seek/read edge cases,
// write-through memory output flag behavior, and AtomicPublish staging wrap chaining without touching the
// workspace filesystem.
package testutil_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func TestTailGuardSourceOpener_DisallowedPathReturnsNotExist(t *testing.T) {
	t.Parallel()

	allowed := filepath.Join(string([]rune{filepath.Separator}), "allowed", "in.txt")
	other := filepath.Join(string([]rune{filepath.Separator}), "allowed", "other.txt")
	opener := testutil.NewTailGuardSourceOpener([]byte("x"), allowed)

	_, err := opener.Open(other)
	if err == nil {
		t.Fatal("Open: expected error for disallowed path")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open: want fs.ErrNotExist, got %v", err)
	}
}

func TestTailGuardReadSeekCloser_SeekInvalidWhence(t *testing.T) {
	t.Parallel()

	opener := testutil.NewTailGuardSourceOpener([]byte("abc"), "")
	rc, err := opener.Open("any")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = rc.Close() })

	_, seekErr := rc.Seek(0, 99)
	if seekErr == nil {
		t.Fatal("Seek: expected error for invalid whence")
	}

	if !errors.Is(seekErr, testutil.ErrTailGuardSeekInvalidWhence) {
		t.Fatalf("Seek: want ErrTailGuardSeekInvalidWhence, got %v", seekErr)
	}
}

func TestTailGuardReadSeekCloser_SeekNegativePosition(t *testing.T) {
	t.Parallel()

	opener := testutil.NewTailGuardSourceOpener([]byte("abc"), "")
	rc, err := opener.Open("any")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = rc.Close() })

	_, seekErr := rc.Seek(-1, io.SeekStart)
	if seekErr == nil {
		t.Fatal("Seek: expected error for negative absolute position")
	}

	if !errors.Is(seekErr, testutil.ErrTailGuardSeekNegativePosition) {
		t.Fatalf("Seek: want ErrTailGuardSeekNegativePosition, got %v", seekErr)
	}
}

func TestTailGuardReadSeekCloser_SeekEndClampsToPrefix(t *testing.T) {
	t.Parallel()

	prefix := []byte("abcde")
	opener := testutil.NewTailGuardSourceOpener(prefix, "")
	rc, err := opener.Open("any")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = rc.Close() })

	pos, seekErr := rc.Seek(100, io.SeekEnd)
	if seekErr != nil {
		t.Fatalf("Seek: %v", seekErr)
	}

	want := int64(len(prefix))
	if pos != want {
		t.Fatalf("Seek end+large offset: got position %d want clamped %d", pos, want)
	}
}

func TestTailGuardReadSeekCloser_ReadZeroLenBufferAtEOF(t *testing.T) {
	t.Parallel()

	prefix := []byte("ab")
	opener := testutil.NewTailGuardSourceOpener(prefix, "")
	rc, err := opener.Open("any")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = rc.Close() })

	buf := make([]byte, len(prefix))
	if _, readErr := io.ReadFull(rc, buf); readErr != nil {
		t.Fatalf("ReadFull prefix: %v", readErr)
	}

	readCount, readErr := rc.Read(make([]byte, 0))
	if readErr != nil {
		t.Fatalf("Read zero-length at EOF: %v", readErr)
	}

	if readCount != 0 {
		t.Fatalf("Read zero-length: got n=%d want 0", readCount)
	}
}

func TestThroughMemOutputOpener_WritePersistsBeforeClose(t *testing.T) {
	t.Parallel()

	opener := testutil.NewThroughMemOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "through-mem", "early.txt")

	wc, openErr := opener.OpenFile(path, testutil.OutputWriteTruncCreate, 0o644)
	if openErr != nil {
		t.Fatalf("OpenFile: %v", openErr)
	}

	payload := []byte("before-close")

	written, writeErr := wc.Write(payload)
	if writeErr != nil || written != len(payload) {
		t.Fatalf("Write: n=%d err=%v", written, writeErr)
	}

	if got := opener.FileContent(path); !bytes.Equal(got, payload) {
		t.Fatalf("FileContent before Close: got %q want %q", got, payload)
	}

	if closeErr := wc.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	if got := opener.FileContent(path); !bytes.Equal(got, payload) {
		t.Fatalf("FileContent after Close: got %q want %q", got, payload)
	}
}

func TestThroughMemOutputOpener_OEXCLWhenExists(t *testing.T) {
	t.Parallel()

	opener := testutil.NewThroughMemOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "through-mem", "excl.txt")

	wFirst, firstErr := opener.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if firstErr != nil {
		t.Fatalf("first OpenFile: %v", firstErr)
	}

	if closeErr := wFirst.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	_, secondErr := opener.OpenFile(path, testutil.OutputWriteExclusiveCreate, 0o644)
	if secondErr == nil {
		t.Fatal("second exclusive OpenFile: expected error")
	}

	if !errors.Is(secondErr, fs.ErrExist) {
		t.Fatalf("second OpenFile: want os.ErrExist (fs.ErrExist), got %v", secondErr)
	}
}

func TestThroughMemOutputOpener_OTruncResets(t *testing.T) {
	t.Parallel()

	opener := testutil.NewThroughMemOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "through-mem", "trunc.txt")

	w1, err := opener.OpenFile(path, testutil.OutputWriteTruncCreate, 0o644)
	if err != nil {
		t.Fatalf("first OpenFile: %v", err)
	}

	if _, writeErr := w1.Write([]byte("hello")); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}

	if closeErr := w1.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	if got := string(opener.FileContent(path)); got != "hello" {
		t.Fatalf("after first close: got %q want hello", got)
	}

	w2, err := opener.OpenFile(path, testutil.OutputWriteTruncCreate, 0o644)
	if err != nil {
		t.Fatalf("trunc OpenFile: %v", err)
	}

	if got := opener.FileContent(path); len(got) != 0 {
		t.Fatalf("after trunc open (write-through): got %q want empty", got)
	}

	if _, err := w2.Write([]byte("x")); err != nil {
		t.Fatalf("Write after trunc: %v", err)
	}

	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := string(opener.FileContent(path)); got != "x" {
		t.Fatalf("final content: got %q want x", got)
	}
}

func TestThroughMemOutputOpener_OAppendSeedsExisting(t *testing.T) {
	t.Parallel()

	opener := testutil.NewThroughMemOutputOpener()
	path := filepath.Join(string([]rune{filepath.Separator}), "through-mem", "append.txt")

	w1, err := opener.OpenFile(path, testutil.OutputWriteTruncCreate, 0o644)
	if err != nil {
		t.Fatalf("first OpenFile: %v", err)
	}

	if _, writeErr := w1.Write([]byte("hello")); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}

	if closeErr := w1.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	w2, err := opener.OpenFile(path, testutil.OutputWriteAppendCreate, 0o644)
	if err != nil {
		t.Fatalf("append OpenFile: %v", err)
	}

	if got := string(opener.FileContent(path)); got != "hello" {
		t.Fatalf("after append open: got %q want hello", got)
	}

	if _, err := w2.Write([]byte(" world")); err != nil {
		t.Fatalf("append Write: %v", err)
	}

	if err := w2.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	want := "hello world"
	if got := string(opener.FileContent(path)); got != want {
		t.Fatalf("final content: got %q want %q", got, want)
	}
}

type prevChainWriter struct {
	inner      io.Writer
	writeCalls *atomic.Int64
}

func (p *prevChainWriter) Write(b []byte) (int, error) {
	if p.writeCalls != nil {
		p.writeCalls.Add(1)
	}

	return p.inner.Write(b)
}

func TestAtomicPublishStagingWrapChainsAndTallies(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		destBase string
		payload  []byte
	}{
		{name: "split", destBase: "split-out.txt", payload: []byte("alpha")},
		{name: "blocks", destBase: "blocks-out.txt", payload: []byte("bravo")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			assertStagingWrapChainsAndTallies(t, testCase.destBase, testCase.payload)
		})
	}
}

func assertStagingWrapChainsAndTallies(
	t *testing.T,
	destBase string,
	payload []byte,
) {
	t.Helper()

	dest := filepath.Join(string([]rune{filepath.Separator}), "publish-chain", destBase)
	var tally atomic.Int64
	wraps := atomic.Int32{}
	prevWrites := atomic.Int64{}
	base := &bytes.Buffer{}

	prev := func(path string, writer io.Writer) io.Writer {
		if path != dest {
			t.Errorf("prev staging path: got %q want %q", path, dest)
		}

		wraps.Add(1)

		return &prevChainWriter{inner: writer, writeCalls: &prevWrites}
	}

	mw := testutil.ChainAtomicPublishStagingWrap(prev, &tally)

	out := mw(dest, base)

	written, writeErr := out.Write(payload)
	if writeErr != nil || written != len(payload) {
		t.Fatalf("Write: n=%d err=%v", written, writeErr)
	}

	if wraps.Load() != 1 {
		t.Fatalf("prev wraps: got %d want 1", wraps.Load())
	}

	if prevWrites.Load() != 1 {
		t.Fatalf("prev writer Write calls: got %d want 1", prevWrites.Load())
	}

	if tally.Load() != int64(len(payload)) {
		t.Fatalf("tally: got %d want %d", tally.Load(), len(payload))
	}

	if got := base.Bytes(); !bytes.Equal(got, payload) {
		t.Fatalf("base buffer: got %q want %q", got, payload)
	}
}
