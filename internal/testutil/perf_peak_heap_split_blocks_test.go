// perf_peak_heap_split_blocks_test.go exercises nil-dependency guards on peak-heap measurement
// facades without invoking the pipeline or touching the workspace filesystem.
package testutil_test

import (
	"context"
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
	"github.com/spf13/afero"
)

func TestMeasureRunSplitPeakHeap_NilDeps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mem := afero.NewMemMapFs()
	resolver := testutil.NewMemPathResolverWithFS(mem)
	src := &testutil.CountingSourceOpener{Immutable: []byte("x"), AllowedPath: "/src"}
	out := testutil.NewNonRetainingOutputOpener()
	params := pipeline.SplitParams{}

	cases := []struct {
		name     string
		resolver validate.PathResolver
		src      *testutil.CountingSourceOpener
		out      testutil.PeakHeapOutputOpener
	}{
		{name: "nil_resolver", resolver: nil, src: src, out: out},
		{name: "nil_source", resolver: resolver, src: nil, out: out},
		{name: "nil_output", resolver: resolver, src: src, out: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			meas, res, err := testutil.MeasureRunSplitPeakHeap(ctx, tc.src, tc.out, tc.resolver, params)
			if err == nil {
				t.Fatal("MeasureRunSplitPeakHeap: expected error for nil dependency")
			}

			if !errors.Is(err, testutil.ErrMeasureRunSplitPeakHeapNilDeps) {
				t.Fatalf("MeasureRunSplitPeakHeap: want ErrMeasureRunSplitPeakHeapNilDeps, got %v", err)
			}

			if meas != (testutil.SplitLargeOutputMemMeasurement{}) {
				t.Fatalf("MeasureRunSplitPeakHeap: expected zero measurement, got %+v", meas)
			}

			if len(res.Sections) != 0 || len(res.Files) != 0 || len(res.Warnings) != 0 {
				t.Fatalf("MeasureRunSplitPeakHeap: expected empty result, got %+v", res)
			}
		})
	}
}

func TestMeasureRunBlocksPeakHeap_NilDeps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mem := afero.NewMemMapFs()
	resolver := testutil.NewMemPathResolverWithFS(mem)
	src := &testutil.CountingSourceOpener{Immutable: []byte("x"), AllowedPath: "/src"}
	out := testutil.NewNonRetainingOutputOpener()
	params := pipeline.BlocksParams{}

	cases := []struct {
		name     string
		resolver validate.PathResolver
		src      *testutil.CountingSourceOpener
		out      testutil.PeakHeapOutputOpener
	}{
		{name: "nil_resolver", resolver: nil, src: src, out: out},
		{name: "nil_source", resolver: resolver, src: nil, out: out},
		{name: "nil_output", resolver: resolver, src: src, out: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			meas, res, err := testutil.MeasureRunBlocksPeakHeap(ctx, tc.src, tc.out, tc.resolver, params)
			if err == nil {
				t.Fatal("MeasureRunBlocksPeakHeap: expected error for nil dependency")
			}

			if !errors.Is(err, testutil.ErrMeasureRunBlocksPeakHeapNilDeps) {
				t.Fatalf("MeasureRunBlocksPeakHeap: want ErrMeasureRunBlocksPeakHeapNilDeps, got %v", err)
			}

			if meas != (testutil.BlocksLargeOutputMemMeasurement{}) {
				t.Fatalf("MeasureRunBlocksPeakHeap: expected zero measurement, got %+v", meas)
			}

			if len(res.Blocks) != 0 || res.BlocksFound != 0 || len(res.Files) != 0 || len(res.Warnings) != 0 {
				t.Fatalf("MeasureRunBlocksPeakHeap: expected empty result, got %+v", res)
			}
		})
	}
}
