package pipeline_test

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

type nilSharedSeamCase struct {
	name string
	run  func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error
	want error
	src  pipeline.SourceOpener
	out  pipeline.OutputOpener
	res  validate.PathResolver
}

func assertNilSharedSeamCases(t *testing.T, cases []nilSharedSeamCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run(tc.src, tc.out, tc.res)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRunExtract_NilSharedSeamsReturnSpecificErrors(t *testing.T) {
	t.Parallel()

	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	resolver := testutil.NoSymlinkPathResolver{}
	publishFS := testutil.NewMemPublishSession(out.Fs)

	assertNilSharedSeamCases(t, []nilSharedSeamCase{
		{
			name: "extract_nil_source",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunExtract(context.Background(), src, out, resolver, publishFS, pipeline.ExtractParams{})
				return err
			},
			want: pipeline.ErrNilSourceOpener,
			src:  nil, out: out, res: resolver,
		},
		{
			name: "extract_nil_output",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunExtract(context.Background(), src, out, resolver, publishFS, pipeline.ExtractParams{})
				return err
			},
			want: pipeline.ErrNilOutputOpener,
			src:  src, out: nil, res: resolver,
		},
		{
			name: "extract_nil_resolver",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunExtract(context.Background(), src, out, resolver, publishFS, pipeline.ExtractParams{})
				return err
			},
			want: validate.ErrNilPathResolver,
			src:  src, out: out, res: nil,
		},
	})
}

func TestRunSplit_NilSharedSeamsReturnSpecificErrors(t *testing.T) {
	t.Parallel()

	root := testRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	resolver := testutil.NoSymlinkPathResolver{}
	publishFS := testutil.NewMemPublishSession(out.Fs)
	delimiter := regexp.MustCompile("^---$")

	assertNilSharedSeamCases(t, []nilSharedSeamCase{
		{
			name: "split_nil_source",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunSplit(context.Background(), src, out, resolver, publishFS, pipeline.SplitParams{
					Root: root, Delimiter: delimiter, Naming: fileops.Sequential,
				})
				return err
			},
			want: pipeline.ErrNilSourceOpener,
			src:  nil, out: out, res: resolver,
		},
		{
			name: "split_nil_output",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunSplit(context.Background(), src, out, resolver, publishFS, pipeline.SplitParams{
					Root: root, Delimiter: delimiter, Naming: fileops.Sequential,
				})
				return err
			},
			want: pipeline.ErrNilOutputOpener,
			src:  src, out: nil, res: resolver,
		},
		{
			name: "split_nil_resolver",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunSplit(context.Background(), src, out, resolver, publishFS, pipeline.SplitParams{
					Root: root, Delimiter: delimiter, Naming: fileops.Sequential,
				})
				return err
			},
			want: validate.ErrNilPathResolver,
			src:  src, out: out, res: nil,
		},
	})
}

func TestRunBlocks_NilSharedSeamsReturnSpecificErrors(t *testing.T) {
	t.Parallel()

	root := testRoot()
	src := testutil.NewMemSourceOpener()
	out := testutil.NewMemOutputOpener()
	resolver := testutil.NoSymlinkPathResolver{}
	publishFS := testutil.NewMemPublishSession(out.Fs)
	delimiter := regexp.MustCompile("^---$")

	assertNilSharedSeamCases(t, []nilSharedSeamCase{
		{
			name: "blocks_nil_source",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunBlocks(context.Background(), src, out, resolver, publishFS, pipeline.BlocksParams{
					Root: root, StartDelimiter: delimiter, EndDelimiter: delimiter, Naming: fileops.Sequential,
				})
				return err
			},
			want: pipeline.ErrNilSourceOpener,
			src:  nil, out: out, res: resolver,
		},
		{
			name: "blocks_nil_output",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunBlocks(context.Background(), src, out, resolver, publishFS, pipeline.BlocksParams{
					Root: root, StartDelimiter: delimiter, EndDelimiter: delimiter, Naming: fileops.Sequential,
				})
				return err
			},
			want: pipeline.ErrNilOutputOpener,
			src:  src, out: nil, res: resolver,
		},
		{
			name: "blocks_nil_resolver",
			run: func(src pipeline.SourceOpener, out pipeline.OutputOpener, resolver validate.PathResolver) error {
				_, err := pipeline.RunBlocks(context.Background(), src, out, resolver, publishFS, pipeline.BlocksParams{
					Root: root, StartDelimiter: delimiter, EndDelimiter: delimiter, Naming: fileops.Sequential,
				})
				return err
			},
			want: validate.ErrNilPathResolver,
			src:  src, out: out, res: nil,
		},
	})
}
