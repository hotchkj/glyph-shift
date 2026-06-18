package testutil

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

type internalTransformPathResolver struct{}

func (internalTransformPathResolver) Lstat(string) (fs.FileInfo, error) {
	return nil, fs.ErrNotExist
}

func (internalTransformPathResolver) EvalSymlinks(path string) (string, error) {
	return path, nil
}

func TestMeasurePipelineTransformPeakHeapRejectsNilDependencies(t *testing.T) {
	t.Parallel()

	mem := NewMemFileSession()
	st := NewMemFileStaterWithFS(mem.Fs)

	cases := []struct {
		name     string
		st       pipeline.FileStater
		resolver validate.PathResolver
		session  fileops.FileSession
	}{
		{name: "nil_stater", st: nil, resolver: internalTransformPathResolver{}, session: mem},
		{name: "nil_resolver", st: st, resolver: nil, session: mem},
		{name: "nil_session", st: st, resolver: internalTransformPathResolver{}, session: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := MeasurePipelineTransformPeakHeap(
				context.Background(),
				tc.st,
				tc.resolver,
				tc.session,
				pipeline.TransformParams{},
			)
			if !errors.Is(err, errMeasurePipelineTransformNilDeps) {
				t.Fatalf("error = %v, want %v", err, errMeasurePipelineTransformNilDeps)
			}
		})
	}
}

func TestMeasurePipelineTransformCountingSrcMemRejectsNilDependencies(t *testing.T) {
	t.Parallel()

	mem := NewMemFileSession()
	st := NewMemFileStaterWithFS(mem.Fs)
	session := &CountingTransformMemSession{Mem: mem}

	cases := []struct {
		name     string
		st       pipeline.FileStater
		resolver validate.PathResolver
		session  *CountingTransformMemSession
	}{
		{name: "nil_stater", st: nil, resolver: internalTransformPathResolver{}, session: session},
		{name: "nil_resolver", st: st, resolver: nil, session: session},
		{name: "nil_session", st: st, resolver: internalTransformPathResolver{}, session: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := MeasurePipelineTransformCountingSrcMem(
				context.Background(),
				tc.st,
				tc.resolver,
				tc.session,
				pipeline.TransformParams{},
			)
			if !errors.Is(err, errMeasurePipelineTransformCountingNilDeps) {
				t.Fatalf("error = %v, want %v", err, errMeasurePipelineTransformCountingNilDeps)
			}
		})
	}
}

func TestMeasurePipelineTransformPreviewRejectsApplyParams(t *testing.T) {
	t.Parallel()

	mem := NewMemFileSession()
	st := NewMemFileStaterWithFS(mem.Fs)

	_, _, err := MeasurePipelineTransformRecordsPreviewWithoutWrites(
		context.Background(),
		st,
		internalTransformPathResolver{},
		mem,
		pipeline.TransformParams{Yes: true},
	)
	if !errors.Is(err, errMeasurePipelineTransformCountingNilDeps) {
		t.Fatalf("error = %v, want %v", err, errMeasurePipelineTransformCountingNilDeps)
	}
}

func TestMeasurePipelineTransformRecordsPreviewWithoutWrites_NilSessionErrors(t *testing.T) {
	t.Parallel()

	mem := NewMemFileSession()
	st := NewMemFileStaterWithFS(mem.Fs)

	_, _, err := MeasurePipelineTransformRecordsPreviewWithoutWrites(
		context.Background(),
		st,
		internalTransformPathResolver{},
		nil,
		pipeline.TransformParams{Yes: false},
	)
	if !errors.Is(err, errMeasurePipelineTransformCountingNilDeps) {
		t.Fatalf("error = %v, want %v", err, errMeasurePipelineTransformCountingNilDeps)
	}
}

func TestMeasurePipelineTransformApplyRejectsPreviewParams(t *testing.T) {
	t.Parallel()

	mem := NewMemFileSession()
	st := NewMemFileStaterWithFS(mem.Fs)

	_, _, err := MeasurePipelineTransformApplyCountsWritebackBytes(
		context.Background(),
		st,
		internalTransformPathResolver{},
		mem,
		pipeline.TransformParams{Yes: false},
		mem.Fs,
		"/unused",
	)
	if !errors.Is(err, errMeasurePipelineTransformCountingNilDeps) {
		t.Fatalf("error = %v, want %v", err, errMeasurePipelineTransformCountingNilDeps)
	}
}

func TestCountingTransformMemSessionNilMemMethodsReturnSentinel(t *testing.T) {
	t.Parallel()

	session := &CountingTransformMemSession{}
	cases := []struct {
		name string
		run  func() error
	}{
		{name: "open_read", run: func() error {
			_, err := session.OpenRead("/src.txt")
			return err
		}},
		{name: "open_rdwr", run: func() error {
			_, err := session.OpenRDWR("/src.txt")
			return err
		}},
		{name: "create_temp", run: func() error {
			_, err := session.CreateTemp("", "transform-*")
			return err
		}},
		{name: "remove", run: func() error {
			return session.Remove("/tmp")
		}},
		{name: "rename", run: func() error {
			return session.Rename("/old", "/new")
		}},
		{name: "chmod", run: func() error {
			return session.Chmod("/src.txt", 0o644)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run()
			if !errors.Is(err, ErrCountingTransformMemSessionNilMem) {
				t.Fatalf("error = %v, want %v", err, ErrCountingTransformMemSessionNilMem)
			}
		})
	}
}

func TestCountingCreateTempTransformSessionSkipsNilCounter(t *testing.T) {
	t.Parallel()

	inner := NewMemFileSession()
	session := &countingCreateTempTransformSession{inner: inner}
	tmpFile, err := session.CreateTemp("", "transform-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	name := tmpFile.Name()
	if closeErr := tmpFile.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
	if removeErr := session.Remove(name); removeErr != nil {
		t.Fatalf("Remove: %v", removeErr)
	}
}

func TestCountingCreateTempTransformSessionDelegatesModifyOperations(t *testing.T) {
	t.Parallel()

	inner := NewMemFileSession()
	session := &countingCreateTempTransformSession{inner: inner}

	if _, err := session.OpenRDWR("/missing.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("OpenRDWR missing = %v, want fs.ErrNotExist", err)
	}
	if err := session.Rename("/old.txt", "/new.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Rename missing = %v, want fs.ErrNotExist", err)
	}
	if err := session.Chmod("/missing.txt", 0o644); err != nil {
		t.Fatalf("Chmod delegates permissive mem behavior: %v", err)
	}
}

func TestMeasurePipelineTransformApplySurfacesWritebackStatError(t *testing.T) {
	t.Parallel()

	root := filepath.FromSlash("/workspace")
	mem := NewMemFileSession()
	st := NewMemFileStaterWithFS(mem.Fs)
	srcPath := filepath.Join(root, "apply-stat.txt")
	if writeErr := mem.Fs.MkdirAll(filepath.Dir(srcPath), 0o750); writeErr != nil {
		t.Fatalf("mkdir: %v", writeErr)
	}
	if writeErr := afero.WriteFile(mem.Fs, srcPath, []byte("a\r\n"), 0o644); writeErr != nil {
		t.Fatalf("write source: %v", writeErr)
	}

	lf := fileops.TargetLF
	_, _, err := MeasurePipelineTransformApplyCountsWritebackBytes(
		context.Background(),
		st,
		internalTransformPathResolver{},
		mem,
		pipeline.TransformParams{
			FilePath: srcPath,
			Root:     root,
			Opts:     fileops.TransformOptions{LineEndings: &lf},
			Yes:      true,
		},
		afero.NewMemMapFs(),
		srcPath,
	)
	if err == nil {
		t.Fatal("expected writeback stat error")
	}
}

func TestCountingTransformMemSessionOpenRDWR_NotExistUsesFsErrNotExist(t *testing.T) {
	t.Parallel()

	mem := NewMemFileSession()
	ctr := &CountingTransformMemSession{Mem: mem}

	path := filepath.Join(string(filepath.Separator), "missing-transform.txt")

	_, err := ctr.OpenRDWR(path)
	if err == nil {
		t.Fatal("OpenRDWR: want error")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("OpenRDWR: want fs.ErrNotExist, got %v", err)
	}
}
