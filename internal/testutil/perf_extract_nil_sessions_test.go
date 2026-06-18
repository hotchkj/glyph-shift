package testutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
)

func TestMeasurePipelineExtract_NilSourceOpenerErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	out := testutil.NewCountingOutputOpener()
	sess := testutil.NewMemFileSession()

	_, _, err := testutil.MeasurePipelineExtract(
		ctx,
		nil,
		out,
		sess,
		testutil.NewSyntheticAbsentPathResolver(),
		pipeline.ExtractParams{},
	)
	if err == nil {
		t.Fatal("want error for nil source opener")
	}
}

func TestMeasurePipelineExtract_NilDestinationOpenerErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := &testutil.CountingSourceOpener{Immutable: []byte("a\n")}
	sess := testutil.NewMemFileSession()

	_, _, err := testutil.MeasurePipelineExtract(
		ctx,
		src,
		nil,
		sess,
		testutil.NewSyntheticAbsentPathResolver(),
		pipeline.ExtractParams{},
	)
	if err == nil {
		t.Fatal("want error for nil counting output opener")
	}
}

func TestMeasurePipelineExtract_NilPublishSessionErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := &testutil.CountingSourceOpener{Immutable: []byte("a\n")}
	out := testutil.NewCountingOutputOpener()

	_, _, err := testutil.MeasurePipelineExtract(
		ctx,
		src,
		out,
		nil,
		testutil.NewSyntheticAbsentPathResolver(),
		pipeline.ExtractParams{},
	)
	if err == nil {
		t.Fatal("want error for nil publish session")
	}
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("want fileops.ErrNilFileSession, got %v", err)
	}
}
