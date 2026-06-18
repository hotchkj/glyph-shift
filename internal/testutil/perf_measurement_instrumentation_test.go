package testutil

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/spf13/afero"
)

var errPoisonRemove = errors.New("poison remove")

type poisonRemoveMemMapFs struct {
	*afero.MemMapFs
	failPath string
}

func newPoisonRemoveMemMapFs(t *testing.T, failPath string) *poisonRemoveMemMapFs {
	t.Helper()

	base := afero.NewMemMapFs()
	mm, ok := base.(*afero.MemMapFs)
	if !ok {
		t.Fatalf("NewMemMapFs type = %T", base)
	}

	return &poisonRemoveMemMapFs{MemMapFs: mm, failPath: filepath.Clean(failPath)}
}

func (p *poisonRemoveMemMapFs) Remove(name string) error {
	if filepath.Clean(name) == p.failPath {
		return errPoisonRemove
	}

	return p.MemMapFs.Remove(name)
}

func TestCountingSrcMemAdjustPublishSuccessfulReadOverridesOpensAndBytes(t *testing.T) {
	t.Parallel()

	memFs := afero.NewMemMapFs()
	dest := filepath.Join(string(filepath.Separator), "w", "out.txt")

	if werr := afero.WriteFile(memFs, dest, []byte("abc"), 0o644); werr != nil {
		t.Fatalf("WriteFile: %v", werr)
	}

	opens, bytesWr := countingSrcMemAdjustPublishInstrumentation(
		memFs,
		nil,
		pipeline.ExtractParams{Preview: false, DestPath: dest},
		9,
		9,
	)
	if opens != 1 || bytesWr != 3 {
		t.Fatalf("got opens=%d bytes=%d want 1 and 3", opens, bytesWr)
	}
}

func TestCountingSrcMemAdjustPublishMissingFileKeepsPriorInstrumentation(t *testing.T) {
	t.Parallel()

	memFs := afero.NewMemMapFs()

	opens, bytesWr := countingSrcMemAdjustPublishInstrumentation(
		memFs,
		nil,
		pipeline.ExtractParams{Preview: false, DestPath: filepath.Join(string(filepath.Separator), "missing-out.txt")},
		7,
		90,
	)
	if opens != 7 || bytesWr != 90 {
		t.Fatalf("got opens=%d bytes=%d want 7 and 90", opens, bytesWr)
	}
}

func TestCountingSrcMemAdjustPublishPreviewSkipsOverride(t *testing.T) {
	t.Parallel()

	memFs := afero.NewMemMapFs()
	dest := filepath.Join(string(filepath.Separator), "w", "preview-out.txt")

	if werr := afero.WriteFile(memFs, dest, []byte("xyz"), 0o644); werr != nil {
		t.Fatalf("WriteFile: %v", werr)
	}

	opens, bytesWr := countingSrcMemAdjustPublishInstrumentation(
		memFs,
		nil,
		pipeline.ExtractParams{Preview: true, DestPath: dest},
		4,
		5,
	)
	if opens != 4 || bytesWr != 5 {
		t.Fatalf("got opens=%d bytes=%d want 4 and 5", opens, bytesWr)
	}
}

func TestCountingSrcMemAdjustPublishDestinationExistsMapsSingleOpen(t *testing.T) {
	t.Parallel()

	memFs := afero.NewMemMapFs()
	dest := filepath.Join(string(filepath.Separator), "w", "exists-out.txt")

	opens, bytesWr := countingSrcMemAdjustPublishInstrumentation(
		memFs,
		pipeline.ErrDestinationExists,
		pipeline.ExtractParams{DestPath: dest},
		2,
		50,
	)
	if opens != 1 || bytesWr != 50 {
		t.Fatalf("got opens=%d bytes=%d want 1 and 50", opens, bytesWr)
	}
}

func TestMeasurePipelineExtractPublishInstrumentation_MemSessionReadsPublishedFile(t *testing.T) {
	t.Parallel()

	dest := filepath.Join(string(filepath.Separator), "r", "out.txt")
	memFs := afero.NewMemMapFs()

	if err := afero.WriteFile(memFs, dest, []byte("pub"), 0o644); err != nil {
		t.Fatalf("seed published logical file: %v", err)
	}

	pub := NewMemPublishSession(memFs)

	out := NewCountingOutputOpener()

	outputBytes, destOpens, destBytes := measurePipelineExtractPublishInstrumentation(
		nil,
		pipeline.ExtractParams{DestPath: dest, Preview: false},
		pub,
		out,
	)

	if destOpens != 1 || destBytes != 3 {
		t.Fatalf("got opens=%d bytes=%d want 1 and 3", destOpens, destBytes)
	}

	if !bytes.Equal(outputBytes, []byte("pub")) {
		t.Fatalf("OutputBytes = %q want pub", outputBytes)
	}
}

func TestMeasurePipelineExtractPublishInstrumentation_DestinationExistsSetsOpen(t *testing.T) {
	t.Parallel()

	pub := NewMemFileSession()
	out := NewCountingOutputOpener()

	_, destOpens, _ := measurePipelineExtractPublishInstrumentation(
		pipeline.ErrDestinationExists,
		pipeline.ExtractParams{DestPath: filepath.Join(string(filepath.Separator), "d.txt")},
		pub,
		out,
	)

	if destOpens != 1 {
		t.Fatalf("DestinationOpens = %d want 1", destOpens)
	}
}

func TestReadAppendDestSnapshotForHeapRestoreMissing(t *testing.T) {
	t.Parallel()

	_, err := readAppendDestSnapshotForHeapRestore(afero.NewMemMapFs(), filepath.Join(string(filepath.Separator), "nope"))
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, errMeasureCountingSrcReadAppendDestSnapshot) {
		t.Fatalf("want errMeasureCountingSrcReadAppendDestSnapshot, got %v", err)
	}
}

func TestMeasurePipelineExtractCountingSrcMem_HeapRemoveFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := "/heap-remove-root"
	srcAbs := filepath.Join(root, "in.txt")
	destAbs := filepath.Join(root, "out.txt")

	memFs := newPoisonRemoveMemMapFs(t, destAbs)
	resolver := NewMemPathResolverWithFS(memFs)

	if err := memFs.MkdirAll(filepath.Dir(srcAbs), 0o755); err != nil {
		t.Fatalf("mkdir src parent: %v", err)
	}

	srcBytes := []byte("line1\nline2\n")
	if err := afero.WriteFile(memFs, srcAbs, srcBytes, 0o644); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	srcOp := &CountingSourceOpener{Immutable: srcBytes, AllowedPath: srcAbs}

	_, _, err := MeasurePipelineExtractCountingSrcMem(
		ctx,
		srcOp,
		memFs,
		resolver,
		pipeline.ExtractParams{
			SrcPath:  srcAbs,
			DestPath: destAbs,
			Root:     root,
			Lines:    fileops.LineRange{Start: 1, End: 1},
		},
	)
	if err == nil {
		t.Fatal("want heap remove error")
	}
	if !errors.Is(err, errMeasureCountingSrcRemoveDestination) {
		t.Fatalf("want errMeasureCountingSrcRemoveDestination, got %v", err)
	}
	if !errors.Is(err, errPoisonRemove) {
		t.Fatalf("want errPoisonRemove in chain, got %v", err)
	}
}

func TestNewCountingMemPublishSession_NilTallyStillCreatesTemp(t *testing.T) {
	t.Parallel()

	session := NewCountingMemPublishSession(afero.NewMemMapFs(), nil)

	tmp, err := session.CreateTemp("", "publish-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	t.Cleanup(func() { _ = tmp.Close() })
}

func TestNewCountingMemPublishSession_CountsTempCreates(t *testing.T) {
	t.Parallel()

	var tally atomic.Int64
	session := NewCountingMemPublishSession(afero.NewMemMapFs(), &tally)

	tmp, err := session.CreateTemp("", "publish-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	t.Cleanup(func() { _ = tmp.Close() })

	if tally.Load() != 1 {
		t.Fatalf("tally = %d want 1", tally.Load())
	}
}
