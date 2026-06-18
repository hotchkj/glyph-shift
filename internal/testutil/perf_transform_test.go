package testutil_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/spf13/afero"
)

func stubTransformRoot() string {
	return filepath.FromSlash("/workspace")
}

type stubTransformPathResolver struct{}

func (stubTransformPathResolver) Lstat(string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (stubTransformPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

func writeMemSource(t *testing.T, mem afero.Fs, root, logicalName string, content []byte) string {
	t.Helper()

	path := filepath.Join(root, logicalName)
	if err := mem.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := afero.WriteFile(mem, path, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return path
}

func numberedCRLFSource(lineCount int) []byte {
	var sb strings.Builder
	for i := 1; i <= lineCount; i++ {
		_, _ = fmt.Fprintf(&sb, "line %d\r\n", i)
	}

	return []byte(sb.String())
}

func TestMeasurePipelineTransformCountingSrcMem(t *testing.T) {
	t.Parallel()

	root := stubTransformRoot()
	mem := testutil.NewMemFileSession()
	st := testutil.NewMemFileStaterWithFS(mem.Fs)

	srcPath := writeMemSource(t, mem.Fs, root, "count.txt", numberedCRLFSource(500))

	ctrSess := &testutil.CountingTransformMemSession{Mem: mem}

	lf := fileops.TargetLF

	meas, res, err := testutil.MeasurePipelineTransformCountingSrcMem(
		context.Background(),
		st,
		stubTransformPathResolver{},
		ctrSess,
		pipeline.TransformParams{
			FilePath: srcPath,
			Root:     root,
			Opts: fileops.TransformOptions{
				LineEndings: &lf,
			},
			Yes: false,
		},
	)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	if !res.Result.WouldChange {
		t.Fatalf("expected would_change preview for CRLF→LF")
	}

	if meas.SourceBytesMaterialized != 0 {
		t.Fatalf("preview materialized %d source bytes via OpenRDWR, want 0", meas.SourceBytesMaterialized)
	}
}

func TestMeasurePipelineTransformCountingSrcMem_NilInnerMemSession(t *testing.T) {
	t.Parallel()

	mem := afero.NewMemMapFs()
	st := testutil.NewMemFileStaterWithFS(mem)

	_, _, err := testutil.MeasurePipelineTransformCountingSrcMem(
		context.Background(),
		st,
		stubTransformPathResolver{},
		&testutil.CountingTransformMemSession{},
		pipeline.TransformParams{},
	)
	if err == nil {
		t.Fatal("MeasurePipelineTransformCountingSrcMem: expected nil inner mem session error")
	}

	if !errors.Is(err, testutil.ErrCountingTransformMemSessionNilMem) {
		t.Fatalf("MeasurePipelineTransformCountingSrcMem: want ErrCountingTransformMemSessionNilMem, got %v", err)
	}
}

func TestMeasurePipelineTransformRecordsPreviewWithoutWrites(t *testing.T) {
	t.Parallel()

	root := stubTransformRoot()
	mem := testutil.NewMemFileSession()
	st := testutil.NewMemFileStaterWithFS(mem.Fs)

	srcPath := writeMemSource(t, mem.Fs, root, "prev.txt", numberedCRLFSource(120))

	lf := fileops.TargetLF

	meas, _, err := testutil.MeasurePipelineTransformRecordsPreviewWithoutWrites(
		context.Background(),
		st,
		stubTransformPathResolver{},
		mem,
		pipeline.TransformParams{
			FilePath: srcPath,
			Root:     root,
			Opts: fileops.TransformOptions{
				LineEndings: &lf,
			},
			Yes: false,
		},
	)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	if meas.PreviewTempCreates != 0 {
		t.Fatalf("preview temp creates = %d want 0", meas.PreviewTempCreates)
	}
}

func TestMeasurePipelineTransformApplyCountsWritebackBytes(t *testing.T) {
	t.Parallel()

	root := stubTransformRoot()
	mem := testutil.NewMemFileSession()
	st := testutil.NewMemFileStaterWithFS(mem.Fs)

	src := numberedCRLFSource(80)
	srcPath := writeMemSource(t, mem.Fs, root, "apply.txt", src)

	lf := fileops.TargetLF

	meas, _, err := testutil.MeasurePipelineTransformApplyCountsWritebackBytes(
		context.Background(),
		st,
		stubTransformPathResolver{},
		mem,
		pipeline.TransformParams{
			FilePath: srcPath,
			Root:     root,
			Opts: fileops.TransformOptions{
				LineEndings: &lf,
			},
			Yes: true,
		},
		mem.Fs,
		srcPath,
	)
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	if meas.ApplyWritebackBytes <= 0 {
		t.Fatalf("apply writeback bytes = %d", meas.ApplyWritebackBytes)
	}

	// LF output must be shorter than CRLF source for this fixture.
	if meas.ApplyWritebackBytes >= int64(len(src)) {
		t.Fatalf("apply size %d should be < source %d after CRLF→LF", meas.ApplyWritebackBytes, len(src))
	}
}
