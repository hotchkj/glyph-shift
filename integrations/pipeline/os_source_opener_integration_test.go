//go:build integration

// Real-OS justification: production SourceOpener construction and session injection
// are verified here; unit tests must not reference production opener types (forbidigo).
package pipeline_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

var errOSSourceOpenerStubFileSession = errors.New("pipeline os source opener integration test: stub FileSession")

type osSourceOpenerStubFileSession struct{}

func (osSourceOpenerStubFileSession) OpenRead(string) (fileops.SessionReadHandle, error) {
	return nil, errOSSourceOpenerStubFileSession
}

func (osSourceOpenerStubFileSession) OpenRDWR(string) (fileops.SessionRDWRHandle, error) {
	return nil, errOSSourceOpenerStubFileSession
}

func (osSourceOpenerStubFileSession) CreateTemp(string, string) (fileops.SessionTempHandle, error) {
	return nil, errOSSourceOpenerStubFileSession
}

func (osSourceOpenerStubFileSession) Remove(string) error {
	return errOSSourceOpenerStubFileSession
}

func (osSourceOpenerStubFileSession) Rename(string, string) error {
	return errOSSourceOpenerStubFileSession
}

func (osSourceOpenerStubFileSession) Chmod(string, fs.FileMode) error {
	return errOSSourceOpenerStubFileSession
}

var _ fileops.FileSession = osSourceOpenerStubFileSession{}

func TestNewOSSourceOpenerRejectsNilSession(t *testing.T) {
	t.Parallel()

	if _, err := pipeline.NewOSSourceOpener(nil); !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("NewOSSourceOpener(nil) = %v want ErrNilFileSession", err)
	}
}

func TestOSSourceOpenerUsesInjectedFileSessionWithoutHostIO(t *testing.T) {
	t.Parallel()

	opener, err := pipeline.NewOSSourceOpener(osSourceOpenerStubFileSession{})
	if err != nil {
		t.Fatalf("NewOSSourceOpener: %v", err)
	}

	if _, openErr := opener.Open("source.txt"); !errors.Is(openErr, errOSSourceOpenerStubFileSession) {
		t.Fatalf("Open = %v want stub session error", openErr)
	}
}
