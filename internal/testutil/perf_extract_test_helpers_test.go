package testutil_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

func mustBuildExtractFixture(t *testing.T, fx testutil.ExtractFixture) (source, golden []byte) {
	t.Helper()

	ctx := context.Background()

	src, want, err := testutil.BuildExtractFixture(ctx, fx)
	if err != nil {
		t.Fatalf("BuildExtractFixture: %v", err)
	}

	return src, want
}

func mustMeasureFileopsExtract(
	t *testing.T, src []byte, lines fileops.LineRange, appendMode bool,
) (testutil.ExtractMeasurement, fileops.ExtractResult) {
	t.Helper()

	meas, res, err := testutil.MeasureFileopsExtract(context.Background(), src, lines, appendMode)
	if err != nil {
		t.Fatalf("MeasureFileopsExtract: %v", err)
	}

	return meas, res
}

//nolint:gocritic // hugeParam: test helper forwards CLI-shaped ExtractParams.
func mustMeasurePipelineExtract(
	t *testing.T,
	src *testutil.CountingSourceOpener,
	out *testutil.CountingOutputOpener,
	params pipeline.ExtractParams,
) (testutil.ExtractMeasurement, fileops.ExtractResult) {
	t.Helper()

	memFs := afero.NewMemMapFs()

	sess := testutil.NewMemFileSession()
	sess.SetFs(memFs)

	meas, res, err := testutil.MeasurePipelineExtract(
		context.Background(),
		src,
		out,
		sess,
		testutil.NewSyntheticAbsentPathResolver(),
		params,
	)
	if err != nil {
		t.Fatalf("MeasurePipelineExtract: %v", err)
	}

	return meas, res
}

//nolint:gocritic // hugeParam: test helper forwards CLI-shaped ExtractParams.
func mustMeasurePipelineExtractCountingSrcMem(
	t *testing.T,
	src *testutil.CountingSourceOpener,
	memFs afero.Fs,
	resolver validate.PathResolver,
	params pipeline.ExtractParams,
) (testutil.ExtractMeasurement, fileops.ExtractResult) {
	t.Helper()

	meas, res, err := testutil.MeasurePipelineExtractCountingSrcMem(
		context.Background(),
		src,
		memFs,
		resolver,
		params,
	)
	if err != nil {
		t.Fatalf("MeasurePipelineExtractCountingSrcMem: %v", err)
	}

	return meas, res
}

func requireBytesEqual(tb testing.TB, got, want []byte, prefix string) {
	tb.Helper()

	if bytes.Equal(got, want) {
		return
	}

	tb.Fatalf("%s: bytes mismatch, want_len=%d got_len=%d", prefix, len(want), len(got))
}

func requirePositive(tb testing.TB, name string, value int64) {
	tb.Helper()

	if value > 0 {
		return
	}

	tb.Fatalf("%s must be positive, got %d", name, value)
}

func requireZero(tb testing.TB, name string, value int64) {
	tb.Helper()

	if value == 0 {
		return
	}

	tb.Fatalf("%s want 0 got %d", name, value)
}

func requireEQ(tb testing.TB, name string, got, want int) {
	tb.Helper()

	if got == want {
		return
	}

	tb.Fatalf("%s = %d, want %d", name, got, want)
}
