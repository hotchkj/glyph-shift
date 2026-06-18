package testutil

import (
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/spf13/afero"
)

func TestTailGuardSourceOpenerAccessorsReflectReaderCounters(t *testing.T) {
	t.Parallel()

	path := filepath.Join(string(filepath.Separator), "workspace", "in.txt")
	opener := NewTailGuardSourceOpener([]byte("abcdef"), path)
	reader, err := opener.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	buf := make([]byte, 3)
	if n, readErr := reader.Read(buf); n != 3 || readErr != nil {
		t.Fatalf("Read = (%d, %v), want (3, nil)", n, readErr)
	}
	if pos, seekErr := reader.Seek(1, io.SeekCurrent); pos != 4 || seekErr != nil {
		t.Fatalf("Seek current = (%d, %v), want (4, nil)", pos, seekErr)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	assertInt64(t, "Opens", opener.Opens(), 1)
	assertInt64(t, "AggregateSourceBytesRead", opener.AggregateSourceBytesRead(), 3)
	assertInt64(t, "AggregateSourceReadCalls", opener.AggregateSourceReadCalls(), 1)
	assertInt64(t, "AggregateSourceSeekCalls", opener.AggregateSourceSeekCalls(), 1)
	assertInt64(t, "AggregateSourceCloseCalls", opener.AggregateSourceCloseCalls(), 1)
}

func TestTailGuardSourceOpenerRejectsUnexpectedPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(string(filepath.Separator), "workspace", "in.txt")
	opener := NewTailGuardSourceOpener([]byte("data"), path)

	_, err := opener.Open(filepath.Join(string(filepath.Separator), "workspace", "other.txt"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Open error = %v, want fs.ErrNotExist", err)
	}
}

func TestCountingAndNonRetainingOutputOpenersExposeCountersAndReset(t *testing.T) {
	t.Parallel()

	counting := NewCountingOutputOpener()
	assertOutputCounterLifecycle(t, counting)

	nonRetaining := NewNonRetainingOutputOpener()
	assertOutputCounterLifecycle(t, nonRetaining)
	if got := nonRetaining.OutputBytesSnapshot("/out.txt"); got != nil {
		t.Fatalf("non-retaining snapshot = %q, want nil", got)
	}
}

func TestMeasuringMemOutputOpenerCountsWrites(t *testing.T) {
	t.Parallel()

	opener := NewMeasuringMemOutputOpener(newTestMemFS())
	if err := opener.MkdirAll("/out", 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writer, err := opener.OpenFile("/out/file.txt", OutputWriteTruncCreate, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, writeErr := writer.Write([]byte("payload")); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	assertInt64(t, "MkdirAllCalls", opener.mkdirAllCalls.Load(), 1)
	assertInt64(t, "DestinationOpens", opener.destinationOpens.Load(), 1)
	assertInt64(t, "BytesWritten", opener.bytesWritten.Load(), 7)
}

func newTestMemFS() afero.Fs {
	return afero.NewMemMapFs()
}

type outputCounterLifecycle interface {
	MkdirAll(string, fs.FileMode) error
	OpenFile(string, pipeline.OutputWriteIntent, fs.FileMode) (io.WriteCloser, error)
	MkdirAllCalls() int64
	DestinationOpens() int64
	BytesWritten() int64
	Reset()
}

func assertOutputCounterLifecycle(t *testing.T, opener outputCounterLifecycle) {
	t.Helper()

	if err := opener.MkdirAll("/out", 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writer, err := opener.OpenFile("/out.txt", OutputWriteTruncCreate, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	if _, writeErr := writer.Write([]byte("abc")); writeErr != nil {
		t.Fatalf("Write: %v", writeErr)
	}
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}

	assertOutputCounters(t, opener, 1, 1, 3)

	opener.Reset()
	assertOutputCounters(t, opener, 0, 0, 0)
}

func assertOutputCounters(t *testing.T, opener outputCounterLifecycle, wantMkdir, wantOpen, wantBytes int64) {
	t.Helper()

	assertInt64(t, "MkdirAllCalls", opener.MkdirAllCalls(), wantMkdir)
	assertInt64(t, "DestinationOpens", opener.DestinationOpens(), wantOpen)
	assertInt64(t, "BytesWritten", opener.BytesWritten(), wantBytes)
}

func assertInt64(t *testing.T, name string, got, want int64) {
	t.Helper()

	if got != want {
		t.Fatalf("%s = %d, want %d", name, got, want)
	}
}
