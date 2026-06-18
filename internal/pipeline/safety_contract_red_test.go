package pipeline_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

// errInjectedSafetyFault marks deterministic injected faults in safety red tests.
var errInjectedSafetyFault = errors.New("pipeline_test: injected safety fault")

var (
	errDivergentSeekWhence = errors.New("pipeline_test: divergent seek whence")
	errNegativeSeekPos     = errors.New("pipeline_test: negative seek position")
	errDivergentBufLen     = errors.New("pipeline_test: divergent buffer length mismatch")
)

// divergentScanCopySeek serves scan-phase reads from scan bytes and copy-phase reads from copyBuf
// after sequential scan reaches EOF. Equal-length buffers preserve scan metadata while proving
// silent divergence risk across passes (contract: source stability across scan and byte-span replay).
type divergentScanCopySeek struct {
	scan      []byte
	copyBuf   []byte
	scanPhase bool
	scanPos   int64
	copyPos   int64

	// scanCompleted is set once the logical scan slice is fully consumed (EOF). Until then,
	// Seek(0, Start) rewinds the scan phase so binary-guard + bounded scans observe scan bytes only.
	// After completion, all reads/seeks follow copyBuf so span replay can diverge from scan metadata.
	scanCompleted bool
}

func (d *divergentScanCopySeek) Read(buf []byte) (int, error) {
	if d.scanPhase {
		if d.scanPos >= int64(len(d.scan)) {
			d.scanPhase = false
			d.scanCompleted = true

			return 0, io.EOF
		}

		n := copy(buf, d.scan[d.scanPos:])
		d.scanPos += int64(n)

		return n, nil
	}

	if d.copyPos >= int64(len(d.copyBuf)) {
		return 0, io.EOF
	}

	n := copy(buf, d.copyBuf[d.copyPos:])
	d.copyPos += int64(n)

	return n, nil
}

func (d *divergentScanCopySeek) divergentSeekAbs(offset int64, whence int) (abs int64, err error) {
	length := int64(len(d.copyBuf))

	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		switch {
		case d.scanCompleted || !d.scanPhase:
			abs = d.copyPos + offset
		default:
			abs = d.scanPos + offset
		}
	case io.SeekEnd:
		abs = length + offset
	default:
		return 0, errDivergentSeekWhence
	}

	if abs < 0 {
		return 0, errNegativeSeekPos
	}

	return abs, nil
}

func (d *divergentScanCopySeek) Seek(offset int64, whence int) (int64, error) {
	abs, err := d.divergentSeekAbs(offset, whence)
	if err != nil {
		return 0, err
	}

	length := int64(len(d.copyBuf))

	if whence == io.SeekStart && offset == 0 && !d.scanCompleted {
		d.scanPhase = true
		d.scanPos = 0
		d.copyPos = 0

		return 0, nil
	}

	if d.scanCompleted {
		if abs > length {
			abs = length
		}

		d.scanPhase = false
		d.copyPos = abs

		return abs, nil
	}

	if d.scanPhase {
		d.scanPos = abs
	} else {
		d.copyPos = abs
	}

	return abs, nil
}

func (*divergentScanCopySeek) Close() error {
	return nil
}

// faultAfterCommittedWriter fails writes once cumulative payload bytes reach threshold on the wrapped writer.
type faultAfterCommittedWriter struct {
	inner     io.Writer
	threshold int
	committed int
	failErr   error
}

func (w *faultAfterCommittedWriter) Write(p []byte) (int, error) {
	written, err := w.inner.Write(p)
	if err != nil {
		return written, err
	}

	w.committed += written
	if w.threshold > 0 && w.committed >= w.threshold {
		return written, w.failErr
	}

	return written, nil
}

type divergentSplitSourceOpener struct {
	allowedPath string
	scan        []byte
	copyBuf     []byte
}

func (o *divergentSplitSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	if filepath.Clean(path) != filepath.Clean(o.allowedPath) {
		return nil, fmt.Errorf("open %q: %w", path, fs.ErrNotExist)
	}

	if len(o.scan) != len(o.copyBuf) {
		return nil, errDivergentBufLen
	}

	return &divergentScanCopySeek{
		scan:      o.scan,
		copyBuf:   o.copyBuf,
		scanPhase: true,
	}, nil
}

// divergentExtractValidateReplaySeek serves the validation/planning passes from pass1 then switches to pass2
// on Seek(0, Start) after deterministic extract rewinds beyond the preamble (binary rewind, scan-LineSpans
// rewind-to-zero, replay rewind). Equal-length buffers preserve line boundaries while changing bytes inside
// the extracted range.
type divergentExtractValidateReplaySeek struct {
	pass1, pass2 []byte
	active       []byte
	pos          int64
	seekZeroHits int
}

func newDivergentExtractValidateReplaySeek(pass1, pass2 []byte) (*divergentExtractValidateReplaySeek, error) {
	if len(pass1) != len(pass2) {
		return nil, errDivergentBufLen
	}

	return &divergentExtractValidateReplaySeek{
		pass1:  pass1,
		pass2:  pass2,
		active: pass1,
		pos:    0,
	}, nil
}

func (d *divergentExtractValidateReplaySeek) Read(p []byte) (int, error) {
	if d.pos >= int64(len(d.active)) {
		return 0, io.EOF
	}

	n := copy(p, d.active[d.pos:])
	d.pos += int64(n)

	return n, nil
}

func (d *divergentExtractValidateReplaySeek) Seek(offset int64, whence int) (int64, error) {
	length := int64(len(d.pass1))

	var abs int64

	switch whence {
	case io.SeekStart:
		if offset == 0 {
			d.seekZeroHits++
			// 1: binary guard rewind; 2: ScanLineSpans stream start rewind; >=3 replay-after-plan rewind.
			if d.seekZeroHits >= 3 {
				d.active = d.pass2
			} else {
				d.active = d.pass1
			}
		}

		abs = offset
	case io.SeekCurrent:
		abs = d.pos + offset
	case io.SeekEnd:
		abs = length + offset
	default:
		return 0, errDivergentSeekWhence
	}

	if abs < 0 {
		return 0, errNegativeSeekPos
	}

	if abs > length {
		abs = length
	}

	d.pos = abs

	return abs, nil
}

func (*divergentExtractValidateReplaySeek) Close() error {
	return nil
}

type divergentExtractSourceOpener struct {
	allowedPath string
	pass1       []byte
	pass2       []byte
}

func (o *divergentExtractSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	if filepath.Clean(path) != filepath.Clean(o.allowedPath) {
		return nil, fmt.Errorf("open %q: %w", path, fs.ErrNotExist)
	}

	return newDivergentExtractValidateReplaySeek(o.pass1, o.pass2)
}

// Invariant: split failure while writing section N may leave earlier fully published outputs, but section N
// must be absent or unchanged (no durable partial tail at the failed output path).
func TestRunSplit_SectionNWriteFailureLeavesEarlierOutputsPublishedAndSectionNAbsentOrUnchanged(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	src := testutil.NewMemSourceOpener()

	var splitHuge strings.Builder
	splitHuge.WriteString("---\nB\n---\nC\n---\n")
	for range 400 {
		splitHuge.WriteString("p\n")
	}

	splitHuge.WriteString("D\n")

	content := splitHuge.String()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(content))

	through := testutil.NewThroughMemOutputOpener()
	thirdPath := filepath.Join(outDir, "003.txt")
	thirdClean := filepath.Clean(thirdPath)
	re := regexp.MustCompile(`^---$`)

	publishFS := testutil.NewMemStagingPublishSession(through.Fs, func(destPath string, w io.Writer) io.Writer {
		if filepath.Clean(destPath) != thirdClean {
			return w
		}

		return &faultAfterCommittedWriter{inner: w, threshold: 48, failErr: errInjectedSafetyFault}
	})

	_, err := pipeline.RunSplit(
		context.Background(),
		src,
		through,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Mkdir:     true,
		},
	)
	if err == nil {
		t.Fatal("expected error from injected fault")
	}

	got1 := through.FileContent(filepath.Join(outDir, "001.txt"))
	want1 := []byte("---\nB\n")
	if !bytes.Equal(got1, want1) {
		t.Fatalf("published section 001 must remain intact; got %q want %q", got1, want1)
	}

	got2 := through.FileContent(filepath.Join(outDir, "002.txt"))
	want2 := []byte("---\nC\n")
	if !bytes.Equal(got2, want2) {
		t.Fatalf("published section 002 must remain intact; got %q want %q", got2, want2)
	}

	if through.FileExists(thirdPath) {
		t.Fatalf(
			"failed split section output must stay absent until successful atomic publish "+
				"(committed prefix forbidden); path=%q len=%d",
			thirdPath,
			len(through.FileContent(thirdPath)),
		)
	}
}

// TestRunBlocks_BlockNWriteFailureLeavesEarlierOutputsPublishedAndBlockNAbsentOrUnchanged
// Invariant: blocks matches split — failure while writing block N may leave earlier outputs published,
// but block N must be absent or unchanged.
func TestRunBlocks_BlockNWriteFailureLeavesEarlierOutputsPublishedAndBlockNAbsentOrUnchanged(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	var blkHuge strings.Builder
	blkHuge.WriteString("header\n<<BEGIN>>\na\n<<END>>\n<<BEGIN>>\nb\n<<END>>\n<<BEGIN>>\n")
	for range 300 {
		blkHuge.WriteString("z\n")
	}

	blkHuge.WriteString("c\n<<END>>\n")

	tripleBlocksSource := blkHuge.String()

	src := testutil.NewMemSourceOpener()
	mustWriteAferoFile(t, src.Fs, srcPath, []byte(tripleBlocksSource))

	through := testutil.NewThroughMemOutputOpener()
	thirdPath := filepath.Join(outDir, "003.txt")
	thirdClean := filepath.Clean(thirdPath)
	startRE := regexp.MustCompile(`^<<BEGIN>>$`)
	endRE := regexp.MustCompile(`^<<END>>$`)

	publishFS := testutil.NewMemStagingPublishSession(through.Fs, func(destPath string, w io.Writer) io.Writer {
		if filepath.Clean(destPath) != thirdClean {
			return w
		}

		return &faultAfterCommittedWriter{inner: w, threshold: 24, failErr: errInjectedSafetyFault}
	})

	_, err := pipeline.RunBlocks(
		context.Background(),
		src,
		through,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Mkdir:          true,
		},
	)
	if err == nil {
		t.Fatal("expected error from injected fault")
	}

	got1 := through.FileContent(filepath.Join(outDir, "001.txt"))
	want1 := []byte("a\n")
	if !bytes.Equal(got1, want1) {
		t.Fatalf("published block 001 must remain intact; got %q want %q", got1, want1)
	}

	got2 := through.FileContent(filepath.Join(outDir, "002.txt"))
	want2 := []byte("b\n")
	if !bytes.Equal(got2, want2) {
		t.Fatalf("published block 002 must remain intact; got %q want %q", got2, want2)
	}

	if through.FileExists(thirdPath) {
		t.Fatalf(
			"failed blocks output must stay absent until successful atomic publish; path=%q len=%d",
			thirdPath,
			len(through.FileContent(thirdPath)),
		)
	}
}

// TestRunSplit_SourceStability_ByteMismatchBetweenScanAndReplayMustFailDeterministically
// Invariant: split must not silently emit outputs built from source bytes that differ between bounded scan
// and byte-span replay on the same handle pass ordering.
func TestRunSplit_SourceStability_ByteMismatchBetweenScanAndReplayMustFailDeterministically(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	pad := strings.Repeat("@\n", 6000)
	scanBytes := append([]byte(pad), []byte(runSplitTripleSectionSource)...)
	copyBytes := append([]byte(pad), []byte("---\nX\n---\nC\n---\nD\n")...)
	if len(scanBytes) != len(copyBytes) {
		t.Fatalf("test setup requires equal-length divergent buffers")
	}

	src := &divergentSplitSourceOpener{allowedPath: srcPath, scan: scanBytes, copyBuf: copyBytes}

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	re := regexp.MustCompile(`^---$`)

	_, err := pipeline.RunSplit(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      root,
			Delimiter: re,
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Mkdir:     true,
		},
	)

	if err == nil {
		t.Fatal(
			"red evidence: RunSplit succeeded without deterministic failure despite scan/copy byte divergence " +
				"(contract: source stability)",
		)
	}

	if !errors.Is(err, fileops.ErrSpanFingerprintMismatch) {
		t.Fatalf("want wrapped ErrSpanFingerprintMismatch, got %v", err)
	}
}

// TestRunBlocks_SourceStability_ByteMismatchBetweenScanAndReplayMustFailDeterministically
// Invariant: blocks must not silently emit outputs from differing scan vs replay bytes (same as split).
func TestRunBlocks_SourceStability_ByteMismatchBetweenScanAndReplayMustFailDeterministically(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	outDir := filepath.Join(root, "out")

	const scanTriple = "header\n<<BEGIN>>\na\n<<END>>\n<<BEGIN>>\nb\n<<END>>\n<<BEGIN>>\nc\n<<END>>\n"
	const copyTriple = "header\n<<BEGIN>>\na\n<<END>>\n<<BEGIN>>\nx\n<<END>>\n<<BEGIN>>\nc\n<<END>>\n"

	pad := strings.Repeat("@\n", 6000)
	scanBytes := append([]byte(pad), []byte(scanTriple)...)
	copyBytes := append([]byte(pad), []byte(copyTriple)...)
	if len(scanBytes) != len(copyBytes) {
		t.Fatalf("test setup requires equal-length divergent buffers")
	}

	src := &divergentSplitSourceOpener{allowedPath: srcPath, scan: scanBytes, copyBuf: copyBytes}

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)
	startRE := regexp.MustCompile(`^<<BEGIN>>$`)
	endRE := regexp.MustCompile(`^<<END>>$`)

	_, err := pipeline.RunBlocks(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           root,
			StartDelimiter: startRE,
			EndDelimiter:   endRE,
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Mkdir:          true,
		},
	)

	if err == nil {
		t.Fatal(
			"red evidence: RunBlocks succeeded without deterministic failure despite scan/copy byte divergence " +
				"(contract: source stability)",
		)
	}

	if !errors.Is(err, fileops.ErrSpanFingerprintMismatch) {
		t.Fatalf("want wrapped ErrSpanFingerprintMismatch, got %v", err)
	}
}

// TestRunExtract_SourceStability_ByteMismatchBetweenValidateAndReplayMustFailDeterministically
// Invariant: extract must not publish when serialized line bytes observed during apply-mode planning
// diverge from the replay pass on the same locked handle ordering (multi-pass source stability).
func TestRunExtract_SourceStability_ByteMismatchBetweenValidateAndReplayMustFailDeterministically(t *testing.T) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	pass1 := []byte("a\nb\n")
	pass2 := []byte("x\nb\n")

	src := &divergentExtractSourceOpener{allowedPath: srcPath, pass1: pass1, pass2: pass2}

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
		},
	)

	if err == nil {
		t.Fatal(
			"expected error when validation-pass and replay-pass source bytes differ " +
				"(contract: extract source stability)",
		)
	}

	if !errors.Is(err, fileops.ErrSpanFingerprintMismatch) {
		t.Fatalf("want wrapped ErrSpanFingerprintMismatch, got %v", err)
	}

	if out.FileExists(destPath) {
		t.Fatalf("fingerprint mismatch must not publish destination; path=%q", destPath)
	}
}

// TestRunExtract_OpenEnded_SourceStability_ByteMismatchBetweenValidateAndReplayMustFailDeterministically
// Invariant: open-ended extract streams through EOF in both passes; divergence after the planning pass must
// still fail before publication (same contract as closed-range extract).
func TestRunExtract_OpenEnded_SourceStability_ByteMismatchBetweenValidateAndReplayMustFailDeterministically(
	t *testing.T,
) {
	t.Parallel()

	root := testRoot()
	srcPath := filepath.Join(root, "in.txt")
	destPath := filepath.Join(root, "out.txt")

	pass1 := []byte("a\nb\n")
	pass2 := []byte("a\nz\n")

	src := &divergentExtractSourceOpener{allowedPath: srcPath, pass1: pass1, pass2: pass2}

	out := testutil.NewMemOutputOpener()
	publishFS := newMemPublishSession(t, out.Fs)

	_, err := pipeline.RunExtract(
		context.Background(),
		src,
		out,
		testutil.NoSymlinkPathResolver{},
		publishFS,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 0},
		},
	)

	if err == nil {
		t.Fatal("expected error when open-ended validation vs replay bytes differ")
	}

	if !errors.Is(err, fileops.ErrSpanFingerprintMismatch) {
		t.Fatalf("want wrapped ErrSpanFingerprintMismatch, got %v", err)
	}

	if out.FileExists(destPath) {
		t.Fatalf("fingerprint mismatch must not publish destination; path=%q", destPath)
	}
}
