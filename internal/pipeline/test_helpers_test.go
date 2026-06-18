package pipeline_test

import (
	"errors"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/spf13/afero"
)

func assertDestinationExistsOperationContract(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, pipeline.ErrDestinationExists) {
		t.Fatalf("want errors.Is(ErrDestinationExists), got: %v", err)
	}

	co := pipeline.ClassifyOperationError(err, "")
	if co.Error != "destination_exists" {
		t.Fatalf("ClassifyOperationError.Error got %q want destination_exists", co.Error)
	}

	if co.ExitCode != pipeline.ExitDestExists {
		t.Fatalf("ClassifyOperationError.ExitCode got %d want %d", co.ExitCode, pipeline.ExitDestExists)
	}
}

func assertUnclosedBlockErrorCarriesActionableStartLocation(t *testing.T, err error, fallbackPath string) {
	t.Helper()
	if err == nil {
		t.Fatal("nil error")
	}

	if !errors.Is(err, fileops.ErrUnclosedBlock) {
		t.Fatalf("want errors.Is(fileops.ErrUnclosedBlock), got: %v", err)
	}

	var detail *fileops.UnclosedBlockDetailError
	if !errors.As(err, &detail) || detail.StartLine <= 0 {
		t.Fatalf("want UnclosedBlockDetailError with positive StartLine, got %v", err)
	}

	co := pipeline.ClassifyOperationError(err, fallbackPath)
	if co.Error != "unclosed_block" {
		t.Fatalf("ClassifyOperationError.Error got %q want unclosed_block", co.Error)
	}

	wantStartLine := detail.StartLine
	if co.IntFields["start_line"] != wantStartLine {
		t.Fatalf("start_line: got %d want %d", co.IntFields["start_line"], wantStartLine)
	}

	if co.Hint != err.Error() {
		t.Fatalf("hint: got %q want %q", co.Hint, err.Error())
	}
}

func mustWriteAferoFile(t *testing.T, fs afero.Fs, path string, data []byte) {
	t.Helper()

	if err := afero.WriteFile(fs, path, data, 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func mustAbsPlannedOutputPath(t *testing.T, outDir, basename string) string {
	t.Helper()

	joined := filepath.Join(outDir, basename)
	cleaned := filepath.Clean(joined)

	if filepath.IsAbs(cleaned) {
		return cleaned
	}

	got, err := filepath.Abs(cleaned)
	if err != nil {
		t.Fatalf("Abs planned path %q: %v", joined, err)
	}

	return filepath.Clean(got)
}

func mustAbsCanonicalPath(t *testing.T, path string) string {
	t.Helper()

	cp := filepath.Clean(path)

	if filepath.IsAbs(cp) {
		return cp
	}

	got, err := filepath.Abs(cp)
	if err != nil {
		t.Fatalf("Abs path %q: %v", path, err)
	}

	return filepath.Clean(got)
}

func regexpBlocksStdFence() (startRE, endRE *regexp.Regexp) {
	return regexp.MustCompile(`^<<BEGIN>>$`), regexp.MustCompile(`^<<END>>$`)
}
