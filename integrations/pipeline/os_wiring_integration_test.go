//go:build integration

// Real-OS justification: these smoke tests prove the pipeline production seams
// can publish through OS source, destination, stat, mkdir, and file-session
// wiring. Byte-level correctness stays covered by internal/pipeline memory tests
// and BDD contracts; this file is intentionally narrow runtime evidence.
package pipeline_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestRunExtract_OSSourceAndDestinationPublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.txt")
	destPath := filepath.Join(dir, "out", "output.txt")

	//nolint:gosec // G306: test fixture in t.TempDir().
	if err := os.WriteFile(srcPath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	session := fileops.NewOSFileSession()

	srcOpener, openErr := pipeline.NewOSSourceOpener(session)
	if openErr != nil {
		t.Fatalf("NewOSSourceOpener: %v", openErr)
	}

	res, err := pipeline.RunExtract(
		context.Background(),
		srcOpener,
		pipeline.NewOSOutputOpener(),
		validate.NewOSPathResolver(),
		session,
		pipeline.ExtractParams{
			SrcPath:  srcPath,
			DestPath: destPath,
			Root:     dir,
			Lines:    fileops.LineRange{Start: 1, End: 2},
			Mkdir:    true,
		},
	)
	if err != nil {
		t.Fatalf("RunExtract: %v", err)
	}

	if res.LinesExtracted != 2 {
		t.Fatalf("LinesExtracted = %d, want 2", res.LinesExtracted)
	}

	//nolint:gosec // G304: destination is under t.TempDir().
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if want := []byte("line1\nline2\n"); !bytes.Equal(got, want) {
		t.Fatalf("dest content = %q, want %q", got, want)
	}
}

func TestRunSplit_OSMultiFilePublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.txt")
	outDir := filepath.Join(dir, "out")
	source := []byte("---\nB\n---\nC\n---\nD\n")

	//nolint:gosec // G306: test fixture in t.TempDir().
	if err := os.WriteFile(srcPath, source, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	session := fileops.NewOSFileSession()

	srcOpener, openErr := pipeline.NewOSSourceOpener(session)
	if openErr != nil {
		t.Fatalf("NewOSSourceOpener: %v", openErr)
	}

	res, err := pipeline.RunSplit(
		context.Background(),
		srcOpener,
		pipeline.NewOSOutputOpener(),
		validate.NewOSPathResolver(),
		session,
		pipeline.SplitParams{
			SrcPath:   srcPath,
			OutDir:    outDir,
			Root:      dir,
			Delimiter: regexp.MustCompile(`^---$`),
			Naming:    fileops.Sequential,
			Extension: ".txt",
			Mkdir:     true,
		},
	)
	if err != nil {
		t.Fatalf("RunSplit: %v", err)
	}

	if len(res.Files) != 3 {
		t.Fatalf("Files len = %d, want 3", len(res.Files))
	}

	firstPath := filepath.Join(outDir, "001.txt")
	//nolint:gosec // G304: output is under t.TempDir().
	got, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first output: %v", err)
	}
	if want := []byte("---\nB\n"); !bytes.Equal(got, want) {
		t.Fatalf("001.txt = %q, want %q", got, want)
	}

	secondPath := filepath.Join(outDir, "002.txt")
	//nolint:gosec // G304: output is under t.TempDir().
	got, err = os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second output: %v", err)
	}
	if want := []byte("---\nC\n"); !bytes.Equal(got, want) {
		t.Fatalf("002.txt = %q, want %q", got, want)
	}
}

func TestRunBlocks_OSMultiFilePublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.txt")
	outDir := filepath.Join(dir, "out")
	source := []byte("preamble\n```go\nalpha\n```\ntext\n```go\nbeta\n```\n")

	//nolint:gosec // G306: test fixture in t.TempDir().
	if err := os.WriteFile(srcPath, source, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	session := fileops.NewOSFileSession()

	srcOpener, openErr := pipeline.NewOSSourceOpener(session)
	if openErr != nil {
		t.Fatalf("NewOSSourceOpener: %v", openErr)
	}

	res, err := pipeline.RunBlocks(
		context.Background(),
		srcOpener,
		pipeline.NewOSOutputOpener(),
		validate.NewOSPathResolver(),
		session,
		pipeline.BlocksParams{
			SrcPath:        srcPath,
			OutDir:         outDir,
			Root:           dir,
			StartDelimiter: regexp.MustCompile("^```go$"),
			EndDelimiter:   regexp.MustCompile("^```$"),
			Naming:         fileops.Sequential,
			Extension:      ".txt",
			Mkdir:          true,
		},
	)
	if err != nil {
		t.Fatalf("RunBlocks: %v", err)
	}

	if res.BlocksFound != 2 {
		t.Fatalf("BlocksFound = %d, want 2", res.BlocksFound)
	}
	if len(res.Files) != 2 {
		t.Fatalf("Files len = %d, want 2", len(res.Files))
	}

	firstPath := filepath.Join(outDir, "001.txt")
	//nolint:gosec // G304: output is under t.TempDir().
	got, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("read first output: %v", err)
	}
	if want := []byte("alpha\n"); !bytes.Equal(got, want) {
		t.Fatalf("001.txt = %q, want %q", got, want)
	}

	secondPath := filepath.Join(outDir, "002.txt")
	//nolint:gosec // G304: output is under t.TempDir().
	got, err = os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("read second output: %v", err)
	}
	if want := []byte("beta\n"); !bytes.Equal(got, want) {
		t.Fatalf("002.txt = %q, want %q", got, want)
	}
}

func TestRunTransform_OSInPlacePublication(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "sample.txt")

	//nolint:gosec // G306: test fixture in t.TempDir().
	if err := os.WriteFile(srcPath, []byte("hello   \nworld\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	targetLF := fileops.TargetLF
	res, err := pipeline.RunTransform(
		context.Background(),
		pipeline.NewOSFileStater(),
		validate.NewOSPathResolver(),
		fileops.NewOSFileSession(),
		pipeline.TransformParams{
			FilePath: srcPath,
			Root:     dir,
			Opts: fileops.TransformOptions{
				LineEndings:  &targetLF,
				TrimTrailing: true,
			},
			Yes: true,
		},
	)
	if err != nil {
		t.Fatalf("RunTransform: %v", err)
	}

	if res.ChangeCount != 1 {
		t.Fatalf("ChangeCount = %d, want 1", res.ChangeCount)
	}

	//nolint:gosec // G304: transformed source is under t.TempDir().
	got, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("read transformed source: %v", err)
	}
	if want := []byte("hello\nworld\n"); !bytes.Equal(got, want) {
		t.Fatalf("source content = %q, want %q", got, want)
	}
}
