package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

var (
	errMCPProductionOpen = errors.New("cmd mcp production wiring: source opener failed")
	errMCPProductionRun  = errors.New("cmd mcp production wiring: runner failed")
)

func TestFirstProductionMCPInitError_prefersSourceOpenError(t *testing.T) {
	t.Parallel()

	err := firstProductionMCPInitError(errMCPProductionOpen, errMCPProductionRun)
	if !errors.Is(err, errMCPProductionOpen) {
		t.Fatalf("error = %v want %v", err, errMCPProductionOpen)
	}
}

func TestFirstProductionMCPInitError_fallsBackToRunnerError(t *testing.T) {
	t.Parallel()

	err := firstProductionMCPInitError(nil, errMCPProductionRun)
	if !errors.Is(err, errMCPProductionRun) {
		t.Fatalf("error = %v want %v", err, errMCPProductionRun)
	}
}

func TestFirstProductionMCPInitError_nilWhenBothInputsNil(t *testing.T) {
	t.Parallel()

	if err := firstProductionMCPInitError(nil, nil); err != nil {
		t.Fatalf("error = %v want nil", err)
	}
}

func TestMCPWorkspaceRootOrDefault(t *testing.T) {
	t.Parallel()

	wantDefault := "repo/root"
	if got := mcpWorkspaceRootOrDefault("", "repo/root"); got != wantDefault {
		t.Fatalf("default root = %q want %q", got, wantDefault)
	}
	wantExplicit := filepath.FromSlash("workspace/root")
	if got := mcpWorkspaceRootOrDefault("workspace/root", "repo/root"); got != wantExplicit {
		t.Fatalf("explicit root = %q want %q", got, wantExplicit)
	}
}

func TestInvalidMCPServerInputsDetectsMissingPreconditions(t *testing.T) {
	t.Parallel()

	deps := MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}}
	if invalidMCPServerInputs(bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{}, deps) {
		t.Fatal("complete MCP server inputs should be valid")
	}
	if !invalidMCPServerInputs(nil, &bytes.Buffer{}, &bytes.Buffer{}, deps) {
		t.Fatal("nil stdin should be invalid")
	}
	if !invalidMCPServerInputs(bytes.NewReader(nil), nil, &bytes.Buffer{}, deps) {
		t.Fatal("nil stdout should be invalid")
	}
	if !invalidMCPServerInputs(bytes.NewReader(nil), &bytes.Buffer{}, nil, deps) {
		t.Fatal("nil stderr should be invalid")
	}
	if !invalidMCPServerInputs(bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{}, MCPServerDeps{}) {
		t.Fatal("missing dependencies should be invalid")
	}
}

func TestNopWriteCloserCloseNoops(t *testing.T) {
	t.Parallel()

	if err := (nopWriteCloser{Writer: &bytes.Buffer{}}).Close(); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
}

func TestRunMCPServerInvalidInputsReturnExit1WithoutFallbackStderr(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	code := RunMCPServer(
		nil,
		"1.0.0",
		testWorkspaceLexicalDir,
		bytes.NewReader(nil),
		&stdout,
		nil,
		MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout want empty got %q", stdout.String())
	}
}

func TestWriteMCPServerPreconditionDiagListsMissingInputs(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	writeMCPServerPreconditionDiag(&stderr, nil, nil, MCPServerDeps{})

	want := mcpPreconditionFailurePrefix +
		" stdin is nil; stdout is nil; pipeline runner is nil; path resolver is nil\n"
	if got := stderr.String(); got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestRunMCPServerWorkspaceConstructorErrorWritesDiagnostic(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := RunMCPServer(
		nil,
		"1.0.0",
		string([]byte{0}),
		bytes.NewReader(nil),
		&stdout,
		&stderr,
		MCPServerDeps{Runner: errorContractRunner{}, Resolver: testutil.NoSymlinkPathResolver{}},
	)
	if code != 1 {
		t.Fatalf("exit code: got %d want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout want empty got %q", stdout.String())
	}
	if got := stderr.String(); got == "" {
		t.Fatal("stderr want init diagnostic, got empty")
	}
}

func TestNewProductionMCPSourceOpenerRejectsNilSession(t *testing.T) {
	t.Parallel()

	_, err := newProductionMCPSourceOpener(nil)
	if !errors.Is(err, fileops.ErrNilFileSession) {
		t.Fatalf("error = %v want %v", err, fileops.ErrNilFileSession)
	}
}

func TestNewProductionMCPSourceOpenerAcceptsSession(t *testing.T) {
	t.Parallel()

	opener, err := newProductionMCPSourceOpener(testutil.NewMemFileSession())
	if err != nil {
		t.Fatalf("newProductionMCPSourceOpener: %v", err)
	}
	if opener == nil {
		t.Fatal("source opener is nil")
	}
}

func TestNewProductionMCPRunnerRejectsNilSourceOpener(t *testing.T) {
	t.Parallel()

	_, err := newProductionMCPRunner(nil, nil, nil)
	if !errors.Is(err, pipeline.ErrNilSourceOpener) {
		t.Fatalf("error = %v want %v", err, pipeline.ErrNilSourceOpener)
	}
}

func TestNewProductionMCPDepsBuildsRuntimeDeps(t *testing.T) {
	oldSession := newProductionMCPFileSession
	oldResolver := newProductionMCPPathResolver
	oldOutput := newProductionMCPOutputOpener
	oldStater := newProductionMCPFileStater
	t.Cleanup(func() {
		newProductionMCPFileSession = oldSession
		newProductionMCPPathResolver = oldResolver
		newProductionMCPOutputOpener = oldOutput
		newProductionMCPFileStater = oldStater
	})

	newProductionMCPFileSession = func() fileops.FileSession {
		return testutil.NewMemFileSession()
	}
	newProductionMCPPathResolver = func() validate.PathResolver {
		return testutil.NoSymlinkPathResolver{}
	}
	newProductionMCPOutputOpener = func() pipeline.OutputOpener {
		return testutil.NewMemOutputOpener()
	}
	newProductionMCPFileStater = func() pipeline.FileStater {
		return testutil.NewMemFileStater()
	}

	deps, err := newProductionMCPDeps()
	if err != nil {
		t.Fatalf("newProductionMCPDeps: %v", err)
	}
	if deps.Runner == nil || deps.Resolver == nil {
		t.Fatal("production MCP dependencies must be populated")
	}
}
