//go:build integration

// Real-OS justification: this contract test verifies pipeline source-reader
// lifetime against OS-backed cooperative file locks.
package pipeline_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

type gateCloseReadSeekCloser struct {
	inner        io.ReadSeekCloser
	enteredClose chan<- struct{}
	releaseClose <-chan struct{}
}

func (g *gateCloseReadSeekCloser) Read(p []byte) (int, error) {
	return g.inner.Read(p)
}

func (g *gateCloseReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return g.inner.Seek(offset, whence)
}

func (g *gateCloseReadSeekCloser) Close() error {
	select {
	case g.enteredClose <- struct{}{}:
	default:
	}

	<-g.releaseClose

	return g.inner.Close()
}

type gateCloseSourceOpener struct {
	enteredClose chan struct{}
	releaseClose chan struct{}
}

func (o *gateCloseSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	lsr, err := fileops.OpenLockedSourceRead(path, fileops.NewOSFileSession())
	if err != nil {
		return nil, err
	}

	return &gateCloseReadSeekCloser{
		inner:        lsr,
		enteredClose: o.enteredClose,
		releaseClose: o.releaseClose,
	}, nil
}

func seedGateExtractDirs(t *testing.T, dir string) (srcPath, destPath string) {
	t.Helper()

	srcPath = filepath.Join(dir, "in.txt")
	destPath = filepath.Join(dir, "out.txt")

	if err := os.WriteFile(srcPath, []byte("alpha\nbravo\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	return srcPath, destPath
}

func startGateExclusiveModify(t *testing.T, srcPath string) chan error {
	t.Helper()

	modifyEntered := make(chan struct{})
	modifyDone := make(chan error, 1)

	go func() {
		close(modifyEntered)
		modifier, err := fileops.OpenForModify(srcPath, fileops.NewOSFileSession())
		if err != nil {
			modifyDone <- err

			return
		}

		modifier.Abort()
		modifyDone <- nil
	}()

	<-modifyEntered

	return modifyDone
}

func assertExclusiveDoesNotFinishUnderGate(t *testing.T, modifyDone chan error, releaseClose chan struct{}) {
	t.Helper()

	select {
	case err := <-modifyDone:
		close(releaseClose)

		if err != nil {
			t.Fatalf("OpenForModify returned unexpected error while reader Close gated: %v", err)
		}

		t.Fatal(
			"red evidence: cooperative exclusive modify completed while RunExtract still gated source Close " +
				"(expected shared-safe lock semantics)",
		)
	default:
	}
}

func drainExclusiveAfterGateReleased(t *testing.T, modifyDone chan error) {
	t.Helper()

	select {
	case err := <-modifyDone:
		if err != nil {
			t.Fatalf("OpenForModify after releasing Close gate: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for OpenForModify after releasing Close gate")
	}
}

func runGateExtractAsync(
	src pipeline.SourceOpener,
	out *testutil.ThroughMemOutputOpener,
	publishFS fileops.FileSession,
	dir, srcPath, destPath string,
) <-chan error {
	extractDone := make(chan error, 1)

	go func() {
		_, err := pipeline.RunExtract(
			context.Background(),
			src,
			out,
			testutil.NoSymlinkPathResolver{},
			publishFS,
			pipeline.ExtractParams{
				SrcPath:  srcPath,
				DestPath: destPath,
				Root:     dir,
				Lines:    fileops.LineRange{Start: 1, End: 1},
				Mkdir:    true,
			},
		)
		extractDone <- err
	}()

	return extractDone
}

// TestRunExtract_SourceLockCooperativeExclusiveModifyMustNotCompleteWhilePipelineReaderOpen
// Invariant: while pipeline holds the source reader open, cooperative exclusive modify via fileops safe path
// must block or fail deterministically (no silent concurrent modification window).
func TestRunExtract_SourceLockCooperativeExclusiveModifyMustNotCompleteWhilePipelineReaderOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath, destPath := seedGateExtractDirs(t, dir)

	enteredClose := make(chan struct{}, 1)
	releaseClose := make(chan struct{})

	srcOp := &gateCloseSourceOpener{
		enteredClose: enteredClose,
		releaseClose: releaseClose,
	}

	out := testutil.NewThroughMemOutputOpener()

	publishFS := testutil.NewMemFileSession()
	publishFS.SetFs(out.Fs)

	extractDone := runGateExtractAsync(srcOp, out, publishFS, dir, srcPath, destPath)

	<-enteredClose

	modifyDone := startGateExclusiveModify(t, srcPath)
	assertExclusiveDoesNotFinishUnderGate(t, modifyDone, releaseClose)

	close(releaseClose)

	drainExclusiveAfterGateReleased(t, modifyDone)

	if err := <-extractDone; err != nil {
		t.Fatalf("RunExtract: %v", err)
	}
}
