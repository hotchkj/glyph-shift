package pipeline

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func TestClassifyOperationError_pathContextOverridesPrimaryForTraversal(t *testing.T) {
	t.Parallel()

	outDir := "out/dir"
	wrapped := pathContextError(PathRoleOutDir, outDir, fmt.Errorf("validate out-dir: %w", validate.ErrPathTraversal))
	if !errors.Is(wrapped, validate.ErrPathTraversal) {
		t.Fatal("expected errors.Is to reach validate.ErrPathTraversal")
	}

	got := ClassifyOperationError(wrapped, classifyTestPrimaryPath)
	want := ErrorOutcome{
		Error:    "invalid_input",
		OutDir:   outDir,
		Hint:     wrapped.Error(),
		ExitCode: ExitValidation,
	}
	if !outcomesEqual(&got, &want) {
		t.Fatalf("outcome: got %#v want %#v", got, want)
	}
}

func TestClassifyOperationError_fingerprintMismatchUsesPathContextOutputPath(t *testing.T) {
	t.Parallel()

	dest := "build/out.txt"
	wrapped := pathContextError(PathRoleOutputPath, dest, fmt.Errorf("copy: %w", fileops.ErrSpanFingerprintMismatch))
	if !errors.Is(wrapped, fileops.ErrSpanFingerprintMismatch) {
		t.Fatal("expected errors.Is to reach ErrSpanFingerprintMismatch")
	}

	got := ClassifyOperationError(wrapped, classifyTestPrimaryPath)
	want := ErrorOutcome{
		Error:      "source_fingerprint_mismatch",
		OutputPath: dest,
		Hint:       wrapped.Error(),
		ExitCode:   ExitGeneral,
	}
	if !outcomesEqual(&got, &want) {
		t.Fatalf("outcome: got %#v want %#v", got, want)
	}
}

func TestClassifyOperationError_internalErrorPathContextRolesMatchSlots(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		role PathRole
		want ErrorOutcome
	}{
		{
			name: "src",
			role: PathRoleSrc,
			want: ErrorOutcome{
				Error:    "internal_error",
				Src:      "src.txt",
				Hint:     errClassifyTestUnexpected.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "dest",
			role: PathRoleDest,
			want: ErrorOutcome{
				Error:    "internal_error",
				Dest:     "dest.txt",
				Hint:     errClassifyTestUnexpected.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "out_dir",
			role: PathRoleOutDir,
			want: ErrorOutcome{
				Error:    "internal_error",
				OutDir:   "out_dir.txt",
				Hint:     errClassifyTestUnexpected.Error(),
				ExitCode: ExitGeneral,
			},
		},
		{
			name: "output_path",
			role: PathRoleOutputPath,
			want: ErrorOutcome{
				Error:      "internal_error",
				OutputPath: "output_path.txt",
				Hint:       errClassifyTestUnexpected.Error(),
				ExitCode:   ExitGeneral,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := tc.name + ".txt"
			got := ClassifyOperationError(pathContextError(tc.role, path, errClassifyTestUnexpected), classifyTestPrimaryPath)
			if !outcomesEqual(&got, &tc.want) {
				t.Fatalf("outcome: got %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestPathContextError_errorsAsThroughFmtWrap_errorsIsSentinel(t *testing.T) {
	t.Parallel()

	inner := errClassifyTestUnexpected
	pc := pathContextError(PathRoleOutputPath, filepath.Join("rel", "piece.go"), inner)
	outer := fmt.Errorf("outer: %w", pc)

	var got *PathContextError
	if !errors.As(outer, &got) {
		t.Fatal("expected errors.As to recover *PathContextError through fmt.Errorf wrapper")
	}
	if got.Context.Role != PathRoleOutputPath || got.Context.Path != filepath.Join("rel", "piece.go") {
		t.Fatalf("PathContextError: got role=%v path=%q", got.Context.Role, got.Context.Path)
	}
	if !errors.Is(outer, inner) {
		t.Fatal("expected errors.Is to reach inner sentinel through wrapped PathContextError chain")
	}
}

func TestWithPathRole_errors_Is_As_match_path_Context_Error(t *testing.T) {
	t.Parallel()

	inner := fmt.Errorf("%w", validate.ErrPathTraversal)
	wrapped := WithPathRole(PathRoleOutDir, filepath.Join("rel", "out-piece"), inner)
	if !errors.Is(wrapped, validate.ErrPathTraversal) {
		t.Fatal("errors.Is lost sentinel behind WithPathRole")
	}

	var pc *PathContextError
	if !errors.As(wrapped, &pc) {
		t.Fatal("errors.As missing *PathContextError from WithPathRole")
	}
	if pc.Context.Role != PathRoleOutDir || pc.Context.Path != filepath.Join("rel", "out-piece") {
		t.Fatalf("PathContext mismatch: %+v", pc.Context)
	}
}

func TestClassify_WithPathRole_sentinel_overrides_fallback_primary_src(t *testing.T) {
	t.Parallel()

	wrongPrimary := filepath.Join("workspace", "src", "should-not-slot-as-src.go")
	dotsOut := filepath.Join(
		"..", "..", "..", "..", "..", "..", "..", "..", "..",
	)
	destPath := filepath.Join(string(filepath.Separator), "abs", dotsOut, "dest-out")
	err := WithPathRole(PathRoleDest, destPath, fmt.Errorf("%w", validate.ErrPathTraversal))

	got := ClassifyOperationError(err, wrongPrimary)

	want := ErrorOutcome{
		Error:    opErrClassInvalidInput,
		Hint:     err.Error(),
		ExitCode: ExitValidation,
		Dest:     destPath,
	}
	if !outcomesEqual(&got, &want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	if got.Src != "" {
		t.Fatalf("wanted empty Src fallback; got %q", got.Src)
	}
}
