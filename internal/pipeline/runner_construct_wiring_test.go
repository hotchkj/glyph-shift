package pipeline_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestNewDefaultRunner_nilSourceOpener(t *testing.T) {
	t.Parallel()

	out := testutil.NewMemOutputOpener()
	session := testutil.NewMemPublishSessionForOutput(out)

	_, err := pipeline.NewDefaultRunner(
		nil,
		out,
		testutil.NewMemFileStater(),
		testutil.NoSymlinkPathResolver{},
		session,
	)
	if !errors.Is(err, pipeline.ErrNilSourceOpener) {
		t.Fatalf("got %v want %v", err, pipeline.ErrNilSourceOpener)
	}
}

func TestNewDefaultRunner_nilOutputOpener(t *testing.T) {
	t.Parallel()

	src := testutil.NewMemSourceOpener()
	session := testutil.NewMemPublishSessionForOutput(testutil.NewMemOutputOpener())

	_, err := pipeline.NewDefaultRunner(
		src,
		nil,
		testutil.NewMemFileStater(),
		testutil.NoSymlinkPathResolver{},
		session,
	)
	if !errors.Is(err, pipeline.ErrNilOutputOpener) {
		t.Fatalf("got %v want %v", err, pipeline.ErrNilOutputOpener)
	}
}

func TestNewDefaultRunner_nilFileStater(t *testing.T) {
	t.Parallel()

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	session := testutil.NewMemPublishSessionForOutput(out)

	_, err := pipeline.NewDefaultRunner(
		src,
		out,
		nil,
		testutil.NoSymlinkPathResolver{},
		session,
	)
	if !errors.Is(err, pipeline.ErrNilFileStater) {
		t.Fatalf("got %v want %v", err, pipeline.ErrNilFileStater)
	}
}

func TestNewDefaultRunner_nilPathResolver(t *testing.T) {
	t.Parallel()

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	session := testutil.NewMemPublishSessionForOutput(out)

	_, err := pipeline.NewDefaultRunner(
		src,
		out,
		testutil.NewMemFileStater(),
		nil,
		session,
	)
	if !errors.Is(err, validate.ErrNilPathResolver) {
		t.Fatalf("got %v want %v", err, validate.ErrNilPathResolver)
	}
}

func TestNewDefaultRunner_nilFileSession(t *testing.T) {
	t.Parallel()

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()

	_, err := pipeline.NewDefaultRunner(
		src,
		out,
		testutil.NewMemFileStater(),
		testutil.NoSymlinkPathResolver{},
		nil,
	)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("got %v want %v", err, fileops.ErrNilFileSession)
	}
}
