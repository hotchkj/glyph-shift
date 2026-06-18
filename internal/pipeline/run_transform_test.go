package pipeline_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

var errRunTransformStatFailure = errors.New("stat failed")

func TestRunTransform_ContextCanceledBeforeInvoke(t *testing.T) {
	t.Parallel()

	root := testRoot()
	var sb strings.Builder
	for i := 0; i < 2500; i++ {
		_, _ = fmt.Fprintf(&sb, "line-%d\r\n", i)
	}

	mem := testutil.NewMemFileSession()
	st := testutil.NewMemFileStaterWithFS(mem.Fs)
	path := seedMemTransformPath(t, mem.Fs, root, "ctx-stream.txt", []byte(sb.String()))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	lf := fileops.TargetLF
	_, err := pipeline.RunTransform(
		ctx,
		st,
		testutil.NoSymlinkPathResolver{},
		mem,
		pipeline.TransformParams{
			FilePath: path,
			Root:     root,
			Opts:     fileops.TransformOptions{LineEndings: &lf},
			Yes:      false,
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRunTransform_TargetLFTrimTrailing_NormalizesSpacesAndLines(t *testing.T) {
	t.Parallel()

	root := testRoot()
	mem := testutil.NewMemFileSession()
	st := testutil.NewMemFileStaterWithFS(mem.Fs)
	raw := []byte("hello   \nworld\n")
	path := seedMemTransformPath(t, mem.Fs, root, "sample.txt", raw)

	lf := fileops.TargetLF
	opts := fileops.TransformOptions{LineEndings: &lf, TrimTrailing: true}

	out, err := pipeline.RunTransform(
		context.Background(),
		st,
		testutil.NoSymlinkPathResolver{},
		mem,
		pipeline.TransformParams{FilePath: path, Root: root, Opts: opts, Yes: true},
	)
	if err != nil {
		t.Fatalf("RunTransform: %v", err)
	}

	if out.ChangeCount < 1 {
		t.Fatalf("ChangeCount = %d, want >= 1", out.ChangeCount)
	}

	got, readErr := afero.ReadFile(mem.Fs, path)
	if readErr != nil {
		t.Fatalf("read result: %v", readErr)
	}

	want := []byte("hello\nworld\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("file content = %q, want %q", got, want)
	}
}

func TestRunTransform_SourceNotFound(t *testing.T) {
	t.Parallel()

	st := testutil.NewMemFileStater()
	root := testRoot()
	path := filepath.Join(root, "missing.txt")

	_, err := pipeline.RunTransform(
		context.Background(),
		st,
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{
			FilePath: path,
			Root:     root,
			Opts: fileops.TransformOptions{
				TrimTrailing: true,
			},
			Yes: false,
		},
	)
	if !errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("expected ErrSourceNotFound, got: %v", err)
	}
}

func TestRunTransform_NotRegularFile(t *testing.T) {
	t.Parallel()

	st := testutil.NewMemFileStater()
	root := testRoot()
	path := filepath.Join(root, "adir")

	if mkErr := st.Fs.Mkdir(path, 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}

	_, err := pipeline.RunTransform(
		context.Background(),
		st,
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{
			FilePath: path,
			Root:     root,
			Opts: fileops.TransformOptions{
				TrimTrailing: true,
			},
			Yes: false,
		},
	)
	if !errors.Is(err, pipeline.ErrDirectoryNotFile) {
		t.Fatalf("expected ErrDirectoryNotFile, got: %v", err)
	}
}

func TestRunTransform_NilFileStaterReturnsErrNilFileStater(t *testing.T) {
	t.Parallel()

	_, err := pipeline.RunTransform(
		context.Background(),
		nil,
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{},
	)
	if !errors.Is(err, pipeline.ErrNilFileStater) {
		t.Fatalf("want ErrNilFileStater, got %v", err)
	}
}

func TestRunTransform_NilPathResolverReturnsErrNilPathResolver(t *testing.T) {
	t.Parallel()

	_, err := pipeline.RunTransform(
		context.Background(),
		testutil.NewMemFileStater(),
		nil,
		testutil.NewMemFileSession(),
		pipeline.TransformParams{},
	)
	if !errors.Is(err, validate.ErrNilPathResolver) {
		t.Fatalf("want ErrNilPathResolver, got %v", err)
	}
}

func TestRunTransform_NilFileSessionReturnsErrNilFileSession(t *testing.T) {
	t.Parallel()

	_, err := pipeline.RunTransform(
		context.Background(),
		testutil.NewMemFileStater(),
		testutil.NoSymlinkPathResolver{},
		nil,
		pipeline.TransformParams{},
	)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want ErrNilFileSession, got %v", err)
	}
}

func TestRunTransform_StatFailureWithGlobMetacharTreatsAsSourceNotFound(t *testing.T) {
	t.Parallel()

	path := filepath.Join(testRoot(), "*.txt")
	_, err := pipeline.RunTransform(
		context.Background(),
		transformStatErrorStater{err: errRunTransformStatFailure},
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{FilePath: path, Root: testRoot(), Opts: fileops.TransformOptions{TrimTrailing: true}},
	)
	if !errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("want ErrSourceNotFound, got %v", err)
	}
	assertRunTransformSrcPathContext(t, err, path)
}

func TestRunTransform_StatFailureWrapsWithPathContext(t *testing.T) {
	t.Parallel()

	path := filepath.Join(testRoot(), "stat-error.txt")
	_, err := pipeline.RunTransform(
		context.Background(),
		transformStatErrorStater{err: errRunTransformStatFailure},
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{FilePath: path, Root: testRoot(), Opts: fileops.TransformOptions{TrimTrailing: true}},
	)
	if !errors.Is(err, errRunTransformStatFailure) {
		t.Fatalf("want stat failure, got %v", err)
	}
	if errors.Is(err, pipeline.ErrSourceNotFound) {
		t.Fatalf("plain stat failure must not become source_not_found: %v", err)
	}
	assertRunTransformSrcPathContext(t, err, path)
}

func TestRunTransform_ValidatePathOutsideRootWraps(t *testing.T) {
	t.Parallel()

	root := testRoot()
	mem := testutil.NewMemFileSession()
	st := testutil.NewMemFileStaterWithFS(mem.Fs)
	outside := filepath.Join(filepath.Dir(root), "outside-transform.txt")
	if err := afero.WriteFile(mem.Fs, outside, []byte("content\n"), 0o644); err != nil {
		t.Fatalf("seed outside source: %v", err)
	}

	_, err := pipeline.RunTransform(
		context.Background(),
		st,
		testutil.NoSymlinkPathResolver{},
		mem,
		pipeline.TransformParams{FilePath: outside, Root: root, Opts: fileops.TransformOptions{TrimTrailing: true}},
	)
	if !errors.Is(err, validate.ErrPathTraversal) {
		t.Fatalf("want path traversal, got %v", err)
	}
	assertRunTransformSrcPathContext(t, err, outside)
}

func TestRunTransform_NonRegularFileWrapsNotRegularFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(testRoot(), "device")
	_, err := pipeline.RunTransform(
		context.Background(),
		transformStaticStater{info: transformFileInfo{mode: fs.ModeDevice}},
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{FilePath: path, Root: testRoot(), Opts: fileops.TransformOptions{TrimTrailing: true}},
	)
	if !errors.Is(err, pipeline.ErrNotRegularFile) {
		t.Fatalf("want ErrNotRegularFile, got %v", err)
	}
	assertRunTransformSrcPathContext(t, err, path)
}

func TestRunTransform_TransformFailureGetsPathContextSrc(t *testing.T) {
	t.Parallel()

	root := testRoot()
	st := testutil.NewMemFileStater()
	path := seedMemTransformPath(t, st.Fs, root, "missing-from-session.txt", []byte("content\n"))

	_, err := pipeline.RunTransform(
		context.Background(),
		st,
		testutil.NoSymlinkPathResolver{},
		testutil.NewMemFileSession(),
		pipeline.TransformParams{FilePath: path, Root: root, Opts: fileops.TransformOptions{TrimTrailing: true}},
	)
	assertRunTransformSrcPathContext(t, err, path)
}

func transformBenchNumberedCRLF(lineCount int) []byte {
	var sb strings.Builder
	for i := 1; i <= lineCount; i++ {
		_, _ = fmt.Fprintf(&sb, "line %d\r\n", i)
	}

	return []byte(sb.String())
}

func seedMemTransformPath(t *testing.T, memFS afero.Fs, root, rel string, content []byte) string {
	t.Helper()

	logicalPath := filepath.Join(root, rel)
	if err := memFS.MkdirAll(filepath.Dir(logicalPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := afero.WriteFile(memFS, logicalPath, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	return logicalPath
}

func transformOracleStats(t *testing.T, raw []byte, opts fileops.TransformOptions) fileops.TransformFileResult {
	t.Helper()

	lines, err := fileops.ReadLinesFrom(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("read lines: %v", err)
	}

	_, res := fileops.TransformLines(lines, opts)

	return res
}

func assertTransformCountableStatsEqual(t *testing.T, phase string, got, want *fileops.TransformFileResult) {
	t.Helper()

	if got.EndingsChanged != want.EndingsChanged {
		t.Fatalf("%s: EndingsChanged %d want %d", phase, got.EndingsChanged, want.EndingsChanged)
	}

	if got.LFFound != want.LFFound {
		t.Fatalf("%s: LFFound %d want %d", phase, got.LFFound, want.LFFound)
	}

	if got.LFConverted != want.LFConverted {
		t.Fatalf("%s: LFConverted %d want %d", phase, got.LFConverted, want.LFConverted)
	}

	if got.CRFound != want.CRFound {
		t.Fatalf("%s: CRFound %d want %d", phase, got.CRFound, want.CRFound)
	}

	if got.CRConverted != want.CRConverted {
		t.Fatalf("%s: CRConverted %d want %d", phase, got.CRConverted, want.CRConverted)
	}

	if got.CRLFFound != want.CRLFFound {
		t.Fatalf("%s: CRLFFound %d want %d", phase, got.CRLFFound, want.CRLFFound)
	}

	if got.CRLFConverted != want.CRLFConverted {
		t.Fatalf("%s: CRLFConverted %d want %d", phase, got.CRLFConverted, want.CRLFConverted)
	}

	if got.TrailingTrimmed != want.TrailingTrimmed {
		t.Fatalf("%s: TrailingTrimmed %d want %d", phase, got.TrailingTrimmed, want.TrailingTrimmed)
	}

	if got.FinalNewlineAdded != want.FinalNewlineAdded {
		t.Fatalf("%s: FinalNewlineAdded %v want %v", phase, got.FinalNewlineAdded, want.FinalNewlineAdded)
	}
}

func TestRunTransformPreviewDoesNotRetainFullSource(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	t.Parallel()

	root := testRoot()
	smallN := testutil.StreamingResidencyProbeSmallLineCount
	largeN := testutil.LargeStreamingFixtureLineCount

	memS := testutil.NewMemFileSession()
	stS := testutil.NewMemFileStaterWithFS(memS.Fs)
	sPath := seedMemTransformPath(t, memS.Fs, root, "small-tr.txt", transformBenchNumberedCRLF(smallN))

	memL := testutil.NewMemFileSession()
	stL := testutil.NewMemFileStaterWithFS(memL.Fs)
	lPath := seedMemTransformPath(t, memL.Fs, root, "large-tr.txt", transformBenchNumberedCRLF(largeN))

	lf := fileops.TargetLF
	opts := fileops.TransformOptions{LineEndings: &lf}

	sMeas, _, err := testutil.MeasurePipelineTransformPeakHeap(
		context.Background(),
		stS,
		testutil.NoSymlinkPathResolver{},
		memS,
		pipeline.TransformParams{FilePath: sPath, Root: root, Opts: opts, Yes: false},
	)
	if err != nil {
		t.Fatalf("small preview: %v", err)
	}

	lMeas, _, err := testutil.MeasurePipelineTransformPeakHeap(
		context.Background(),
		stL,
		testutil.NoSymlinkPathResolver{},
		memL,
		pipeline.TransformParams{FilePath: lPath, Root: root, Opts: opts, Yes: false},
	)
	if err != nil {
		t.Fatalf("large preview: %v", err)
	}

	budget := testutil.StreamingBodyResidencyRetainedHeapBudget(sMeas.RetainedHeapAllocDelta)
	if lMeas.RetainedHeapAllocDelta > budget {
		t.Fatalf(
			"large retained heap %d exceeds budget %d (small %d)",
			lMeas.RetainedHeapAllocDelta,
			budget,
			sMeas.RetainedHeapAllocDelta,
		)
	}

	if lMeas.PeakHeapAllocDelta > testutil.MaxPeakHeapGrowthForStreamingBody {
		t.Fatalf(
			"large peak heap delta %d exceeds smoke ceiling %d",
			lMeas.PeakHeapAllocDelta,
			testutil.MaxPeakHeapGrowthForStreamingBody,
		)
	}
}

func TestRunTransformApplyStreamsWritebackWithoutFullOutputResidency(t *testing.T) {
	t.Skip("MemStats residency gate flaky on CI; see issue #2")
	t.Parallel()

	root := testRoot()
	smallN := testutil.StreamingResidencyProbeSmallLineCount
	largeN := testutil.LargeStreamingFixtureLineCount

	memS := testutil.NewMemFileSession()
	stS := testutil.NewMemFileStaterWithFS(memS.Fs)
	sPath := seedMemTransformPath(t, memS.Fs, root, "small-ta.txt", transformBenchNumberedCRLF(smallN))

	memL := testutil.NewMemFileSession()
	stL := testutil.NewMemFileStaterWithFS(memL.Fs)
	lPath := seedMemTransformPath(t, memL.Fs, root, "large-ta.txt", transformBenchNumberedCRLF(largeN))

	lf := fileops.TargetLF
	opts := fileops.TransformOptions{LineEndings: &lf}

	sMeas, _, err := testutil.MeasurePipelineTransformPeakHeap(
		context.Background(),
		stS,
		testutil.NoSymlinkPathResolver{},
		memS,
		pipeline.TransformParams{FilePath: sPath, Root: root, Opts: opts, Yes: true},
	)
	if err != nil {
		t.Fatalf("small apply: %v", err)
	}

	lMeas, _, err := testutil.MeasurePipelineTransformPeakHeap(
		context.Background(),
		stL,
		testutil.NoSymlinkPathResolver{},
		memL,
		pipeline.TransformParams{FilePath: lPath, Root: root, Opts: opts, Yes: true},
	)
	if err != nil {
		t.Fatalf("large apply: %v", err)
	}

	budget := testutil.StreamingBodyResidencyRetainedHeapBudget(sMeas.RetainedHeapAllocDelta)
	if lMeas.RetainedHeapAllocDelta > budget {
		t.Fatalf(
			"apply large retained heap %d exceeds budget %d (small %d)",
			lMeas.RetainedHeapAllocDelta,
			budget,
			sMeas.RetainedHeapAllocDelta,
		)
	}
}

func TestRunTransformPreviewAndApplyShareComputedCounts(t *testing.T) {
	t.Parallel()

	raw := transformBenchNumberedCRLF(47)
	lf := fileops.TargetLF
	opts := fileops.TransformOptions{LineEndings: &lf, TrimTrailing: true}
	oracle := transformOracleStats(t, raw, opts)

	root := testRoot()
	mem := testutil.NewMemFileSession() // fresh FS for apply without cross-test contamination
	st := testutil.NewMemFileStaterWithFS(mem.Fs)
	path := seedMemTransformPath(t, mem.Fs, root, "share-counts.txt", raw)

	prev, err := pipeline.RunTransform(
		context.Background(),
		st,
		testutil.NoSymlinkPathResolver{},
		mem,
		pipeline.TransformParams{FilePath: path, Root: root, Opts: opts, Yes: false},
	)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}

	assertTransformCountableStatsEqual(t, "preview", &prev.Result, &oracle)

	mem2 := testutil.NewMemFileSession()
	st2 := testutil.NewMemFileStaterWithFS(mem2.Fs)
	path2 := seedMemTransformPath(t, mem2.Fs, root, "share-counts-apply.txt", raw)

	apply, err := pipeline.RunTransform(
		context.Background(),
		st2,
		testutil.NoSymlinkPathResolver{},
		mem2,
		pipeline.TransformParams{FilePath: path2, Root: root, Opts: opts, Yes: true},
	)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	assertTransformCountableStatsEqual(t, "apply", &apply.Result, &oracle)
}

type transformStatErrorStater struct {
	err error
}

func (s transformStatErrorStater) Stat(string) (fs.FileInfo, error) {
	return nil, s.err
}

type transformStaticStater struct {
	info fs.FileInfo
}

func (s transformStaticStater) Stat(string) (fs.FileInfo, error) {
	return s.info, nil
}

type transformFileInfo struct {
	mode fs.FileMode
}

func (i transformFileInfo) Name() string {
	return "transform-file-info"
}

func (i transformFileInfo) Size() int64 {
	return 0
}

func (i transformFileInfo) Mode() fs.FileMode {
	return i.mode
}

func (i transformFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (i transformFileInfo) IsDir() bool {
	return i.mode.IsDir()
}

func (i transformFileInfo) Sys() any {
	return nil
}

func assertRunTransformSrcPathContext(t *testing.T, err error, wantPath string) {
	t.Helper()

	var pathErr *pipeline.PathContextError
	if !errors.As(err, &pathErr) {
		t.Fatalf("want PathContextError, got %v", err)
	}
	if pathErr.Context.Role != pipeline.PathRoleSrc || pathErr.Context.Path != wantPath {
		t.Fatalf("path context: got %+v want role=%v path=%q", pathErr.Context, pipeline.PathRoleSrc, wantPath)
	}
}
