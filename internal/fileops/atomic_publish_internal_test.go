package fileops

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"
)

var (
	errAtomicPublishInjectedSync           = errors.New("atomic publish test: injected sync failure")
	errAtomicPublishInjectedClose          = errors.New("atomic publish test: injected close failure")
	errAtomicPublishInjectedCleanupClose   = errors.New("atomic publish test: injected cleanup close failure")
	errAtomicPublishInjectedChmod          = errors.New("atomic publish test: injected chmod failure")
	errAtomicPublishInjectedRemove         = errors.New("atomic publish test: injected remove failure")
	errAtomicPublishInjectedAppendSrcClose = errors.New("atomic publish test: injected append source close failure")
	errAtomicPublishInjectedAppendSrcCopy  = errors.New("atomic publish test: injected append source copy failure")
	errAtomicPublishInjectedOpenRead       = errors.New("atomic publish test: injected open read failure")
	errAtomicPublishInjectedCreateTemp     = errors.New("atomic publish test: injected create temp failure")
)

func TestAtomicPublishNilWriteContentRejected(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDeps(&faultAtomicTemp{}, nil, nil)
	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, nil)
	if !errors.Is(err, ErrAtomicNilWriteContent) {
		t.Fatalf("want ErrAtomicNilWriteContent, got %v", err)
	}
	if deps.removedTemp || deps.renamed {
		t.Fatal("temp lifecycle must not run when writeContent is nil")
	}
}

func TestAtomicPublishSyncFailureRemovesTempAndSkipsRename(t *testing.T) {
	t.Parallel()

	deps, temp := newFaultPublishDeps(&faultAtomicTemp{syncErr: errAtomicPublishInjectedSync}, nil, nil)

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedSync) {
		t.Fatalf("want sync error, got %v", err)
	}

	if !temp.closed {
		t.Fatal("temp must be closed during sync-failure cleanup")
	}
	if deps.renamed {
		t.Fatal("rename must not run after sync failure")
	}
	if !deps.removedTemp {
		t.Fatal("temp must be removed after sync failure")
	}
}

func TestAtomicPublishCloseFailureRemovesTempAndSkipsRename(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDeps(&faultAtomicTemp{closeErr: errAtomicPublishInjectedClose}, nil, nil)

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedClose) {
		t.Fatalf("want close error, got %v", err)
	}

	if deps.renamed {
		t.Fatal("rename must not run after close failure")
	}
	if !deps.removedTemp {
		t.Fatal("temp must be removed after close failure")
	}
}

func TestAtomicPublishChmodFailureRemovesTempAndSkipsRename(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDeps(&faultAtomicTemp{}, errAtomicPublishInjectedChmod, nil)

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedChmod) {
		t.Fatalf("want chmod error, got %v", err)
	}

	if deps.renamed {
		t.Fatal("rename must not run after chmod failure")
	}
	if !deps.removedTemp {
		t.Fatal("temp must be removed after chmod failure")
	}
}

func TestAtomicPublishSyncFailureCleanupRemoveFailureObservedWithPrimary(t *testing.T) {
	t.Parallel()

	deps, temp := newFaultPublishDeps(
		&faultAtomicTemp{syncErr: errAtomicPublishInjectedSync},
		nil,
		errAtomicPublishInjectedRemove,
	)

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedSync) {
		t.Fatalf("want primary sync error via errors.Is, got %v", err)
	}
	if !errors.Is(err, errAtomicPublishInjectedRemove) {
		t.Fatalf("want cleanup remove error via errors.Is, got %v", err)
	}

	if !temp.closed {
		t.Fatal("temp must be closed during sync-failure cleanup")
	}
	if deps.renamed {
		t.Fatal("rename must not run after sync failure")
	}
	if !deps.removedTemp {
		t.Fatal("remove must be attempted after sync failure")
	}
}

func TestAtomicPublishSyncFailureCleanupCloseFailureObservedWithPrimary(t *testing.T) {
	t.Parallel()

	deps, temp := newFaultPublishDeps(
		&faultAtomicTemp{syncErr: errAtomicPublishInjectedSync, closeErr: errAtomicPublishInjectedCleanupClose},
		nil,
		nil,
	)

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedSync) {
		t.Fatalf("want primary sync error via errors.Is, got %v", err)
	}
	if !errors.Is(err, errAtomicPublishInjectedCleanupClose) {
		t.Fatalf("want cleanup close error via errors.Is, got %v", err)
	}

	if !temp.closed {
		t.Fatal("temp close must be attempted during cleanup")
	}
	if deps.renamed {
		t.Fatal("rename must not run after sync failure")
	}
	if !deps.removedTemp {
		t.Fatal("temp must be removed after sync failure cleanup")
	}
}

func TestAtomicPublishAppendExistingSourceCloseFailureReturnedWhenCopySucceeds(t *testing.T) {
	t.Parallel()

	src := &faultAppendSourceReadCloser{
		data:     []byte("existing\n"),
		closeErr: errAtomicPublishInjectedAppendSrcClose,
	}
	deps, _ := newFaultPublishDepsOpenRead(nil, nil, nil, func(string) (io.ReadCloser, error) {
		return src, nil
	})

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishAppend,
	}, writeInternalTestBytes([]byte("new\n")))
	if !errors.Is(err, errAtomicPublishInjectedAppendSrcClose) {
		t.Fatalf("want append source close error via errors.Is, got %v", err)
	}

	if deps.renamed {
		t.Fatal("rename must not run after append source close failure")
	}
	if !deps.removedTemp {
		t.Fatal("temp must be removed after append source close failure")
	}
}

func TestAtomicPublishAppendExistingCopyAndCloseFailuresBothDiscoverable(t *testing.T) {
	t.Parallel()

	src := &faultAppendSourceReadCloser{
		readErr:  errAtomicPublishInjectedAppendSrcCopy,
		closeErr: errAtomicPublishInjectedAppendSrcClose,
	}
	deps, _ := newFaultPublishDepsOpenRead(nil, nil, nil, func(string) (io.ReadCloser, error) {
		return src, nil
	})

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishAppend,
	}, writeInternalTestBytes([]byte("new\n")))
	if !errors.Is(err, errAtomicPublishInjectedAppendSrcCopy) {
		t.Fatalf("want append copy error via errors.Is, got %v", err)
	}
	if !errors.Is(err, errAtomicPublishInjectedAppendSrcClose) {
		t.Fatalf("want append source close error via errors.Is, got %v", err)
	}

	if deps.renamed {
		t.Fatal("rename must not run after append copy failure")
	}
	if !deps.removedTemp {
		t.Fatal("temp must be removed after append copy failure")
	}
}

func TestAtomicPublishCreateRejectsExistingDestination(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDepsOpenRead(nil, nil, nil, func(string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader([]byte("existing\n"))), nil
	})

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishCreate,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, ErrAtomicDestinationExists) {
		t.Fatalf("want ErrAtomicDestinationExists, got %v", err)
	}
	if deps.removedTemp || deps.renamed {
		t.Fatal("temp lifecycle must not run when create destination already exists")
	}
}

func TestAtomicPublishCreateSurfacesDestinationOpenError(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDepsOpenRead(nil, nil, nil, func(string) (io.ReadCloser, error) {
		return nil, errAtomicPublishInjectedOpenRead
	})

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishCreate,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedOpenRead) {
		t.Fatalf("want open-read error, got %v", err)
	}
	if deps.removedTemp || deps.renamed {
		t.Fatal("temp lifecycle must not run when destination existence check fails")
	}
}

func TestAtomicPublishSurfacesCreateTempError(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDeps(nil, nil, nil)
	deps.createTemp = func(_, _ string) (atomicTempFile, error) {
		return nil, errAtomicPublishInjectedCreateTemp
	}

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedCreateTemp) {
		t.Fatalf("want create temp error, got %v", err)
	}
	if deps.removedTemp || deps.renamed {
		t.Fatal("temp cleanup and rename must not run when temp creation fails")
	}
}

func TestAtomicPublishCloseFailureCleanupRemoveFailureObservedWithPrimary(t *testing.T) {
	t.Parallel()

	deps, _ := newFaultPublishDeps(
		&faultAtomicTemp{closeErr: errAtomicPublishInjectedClose},
		nil,
		errAtomicPublishInjectedRemove,
	)

	err := atomicPublishWithDeps(deps.atomicPublishDeps, AtomicPublishOptions{
		Path: "/out.txt",
		Perm: 0o600,
		Mode: AtomicPublishReplace,
	}, writeInternalTestBytes([]byte("replacement\n")))
	if !errors.Is(err, errAtomicPublishInjectedClose) {
		t.Fatalf("want primary close error via errors.Is, got %v", err)
	}
	if !errors.Is(err, errAtomicPublishInjectedRemove) {
		t.Fatalf("want cleanup remove error via errors.Is, got %v", err)
	}

	if deps.renamed {
		t.Fatal("rename must not run after close failure")
	}
	if !deps.removedTemp {
		t.Fatal("remove must be attempted after close failure")
	}
}

func writeInternalTestBytes(data []byte) func(io.Writer) error {
	return func(writer io.Writer) error {
		_, err := writer.Write(data)
		return err
	}
}

type faultPublishDeps struct {
	atomicPublishDeps

	renamed     bool
	removedTemp bool
}

func newFaultPublishDeps(temp *faultAtomicTemp, chmodErr, removeFail error) (*faultPublishDeps, *faultAtomicTemp) {
	return newFaultPublishDepsOpenRead(temp, chmodErr, removeFail, func(string) (io.ReadCloser, error) {
		return nil, fs.ErrNotExist
	})
}

func newFaultPublishDepsOpenRead(
	temp *faultAtomicTemp,
	chmodErr, removeFail error,
	openRead func(string) (io.ReadCloser, error),
) (*faultPublishDeps, *faultAtomicTemp) {
	if temp == nil {
		temp = &faultAtomicTemp{}
	}
	if temp.name == "" {
		temp.name = "/tmp/.glyph-shift-test"
	}

	deps := &faultPublishDeps{}
	deps.atomicPublishDeps = atomicPublishDeps{
		createTemp: func(_, _ string) (atomicTempFile, error) {
			return temp, nil
		},
		remove: func(name string) error {
			deps.removedTemp = name == temp.name
			return removeFail
		},
		rename: func(_, _ string) error {
			deps.renamed = true
			return nil
		},
		chmod: func(string, fs.FileMode) error {
			return chmodErr
		},
		openRead: openRead,
	}

	return deps, temp
}

type faultAppendSourceReadCloser struct {
	data     []byte
	readErr  error
	closeErr error
	pos      int
}

func (f *faultAppendSourceReadCloser) Read(buf []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	if f.pos >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(buf, f.data[f.pos:])
	f.pos += n
	return n, nil
}

func (f *faultAppendSourceReadCloser) Close() error {
	return f.closeErr
}

type faultAtomicTemp struct {
	bytes.Buffer

	name     string
	syncErr  error
	closeErr error
	closed   bool
}

func (t *faultAtomicTemp) Sync() error {
	return t.syncErr
}

func (t *faultAtomicTemp) Close() error {
	t.closed = true
	return t.closeErr
}

func (t *faultAtomicTemp) Name() string {
	return t.name
}
