package cmd

import (
	"bytes"
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

var errRootDispatchFactory = errors.New("cmd root dispatch: runner factory failed")

func wantPrintUsageGolden(t *testing.T) string {
	t.Helper()

	var buf bytes.Buffer
	printUsage(&buf)

	return buf.String()
}

func TestDispatchCLI_nilStdout_returnsExit1(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"version"},
		"1.0.0",
		testWorkspaceLexicalDir,
		nil,
		&stderr,
		errorContractRunner{},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr want empty got %q", stderr.String())
	}
}

func TestDispatchCLI_nilStderr_returnsExit1(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	code := DispatchCLI(
		[]string{"version"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		nil,
		errorContractRunner{},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}

	if stdout.Len() != 0 {
		t.Fatalf("stdout want empty got %q", stdout.String())
	}
}

func TestDispatchCLI_noArgs_matchesPrintUsageGoldenExit0(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		nil,
		"9.9.9",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		errorContractRunner{},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr want empty got %q", stderr.String())
	}

	want := wantPrintUsageGolden(t)
	if stdout.String() != want {
		t.Fatalf("stdout mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

func TestDispatchCLIWithRunnerFactoryErrorWritesDiagnostic(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := dispatchCLIWithRunnerFactory(
		[]string{"version"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		func() (pipeline.Runner, error) {
			return nil, errRootDispatchFactory
		},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout want empty got %q", stdout.String())
	}
	want := "pipeline runner: " + errRootDispatchFactory.Error() + "\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr got %q want %q", got, want)
	}
}

func TestNewPipelineRunnerFromFactoriesBuildsRunnerWithFakes(t *testing.T) {
	t.Parallel()

	runner, err := newPipelineRunnerFromFactories(fakePipelineRunnerFactories(nil))
	if err != nil {
		t.Fatalf("newPipelineRunnerFromFactories: %v", err)
	}
	if runner == nil {
		t.Fatal("runner is nil")
	}
}

func TestNewPipelineRunnerFromFactoriesReturnsSourceOpenerError(t *testing.T) {
	t.Parallel()

	runner, err := newPipelineRunnerFromFactories(fakePipelineRunnerFactories(errRootDispatchFactory))
	if !errors.Is(err, errRootDispatchFactory) {
		t.Fatalf("error = %v, want %v", err, errRootDispatchFactory)
	}
	if runner != nil {
		t.Fatalf("runner = %#v, want nil", runner)
	}
}

func fakePipelineRunnerFactories(sourceErr error) pipelineRunnerFactories {
	return pipelineRunnerFactories{
		newSession: func() fileops.FileSession {
			return testutil.NewMemFileSession()
		},
		newSourceOpener: func(fileops.FileSession) (pipeline.SourceOpener, error) {
			if sourceErr != nil {
				return nil, sourceErr
			}

			return testutil.NewMemSourceOpener(), nil
		},
		newOutputOpener: func() pipeline.OutputOpener {
			return testutil.NewMemOutputOpener()
		},
		newFileStater: func() pipeline.FileStater {
			return testutil.NewMemFileStater()
		},
		newPathResolver: func() validate.PathResolver {
			return testutil.NoSymlinkPathResolver{}
		},
	}
}

func TestDispatchCLI_topLevelDashHelp_matchesGoldenExit0(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"--help"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		errorContractRunner{},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr want empty got %q", stderr.String())
	}

	want := wantPrintUsageGolden(t)
	if stdout.String() != want {
		t.Fatalf("stdout mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}

func TestDispatchCLI_topShortHelp_matchesGoldenExit0(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"-h"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		errorContractRunner{},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	want := wantPrintUsageGolden(t)
	if stdout.String() != want {
		t.Fatalf("stdout mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr want empty got %q", stderr.String())
	}
}

func TestDispatchCLI_helpCommand_matchesGoldenExit0(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := DispatchCLI(
		[]string{"help"},
		"1.0.0",
		testWorkspaceLexicalDir,
		&stdout,
		&stderr,
		errorContractRunner{},
	)
	if code != 0 {
		t.Fatalf("exit code: got %d want 0", code)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr want empty got %q", stderr.String())
	}

	want := wantPrintUsageGolden(t)
	if stdout.String() != want {
		t.Fatalf("stdout mismatch\n--- got ---\n%s\n--- want ---\n%s", stdout.String(), want)
	}
}
