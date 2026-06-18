package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

var errDispatchMainGetwdDenied = errors.New("permission denied")

func TestDefaultMainDepsPopulatesGetwd(t *testing.T) {
	t.Parallel()

	if defaultMainDeps().getwd == nil {
		t.Fatal("getwd is nil")
	}
}

func TestDispatchMain_RoutesMCPWhenFirstArgIsMCP(t *testing.T) {
	t.Parallel()

	var (
		gotMCPArgs []string
		gotVer     string
		gotCWD     string
	)
	deps := mainDeps{
		getwd:  func() (string, error) { return "/tmp/workspace", nil },
		stdin:  strings.NewReader(""),
		stdout: io.Discard,
		stderr: io.Discard,
		runMCP: func(args []string, version, cwd string, _ io.Reader, _, _ io.Writer) int {
			gotMCPArgs = append([]string(nil), args...)
			gotVer = version
			gotCWD = cwd
			return 42
		},
		runCLI: func([]string, string) int {
			t.Fatal("runCLI must not be called for mcp dispatch")
			return 0
		},
	}

	code := dispatchMain([]string{"mcp", "tool", "x"}, "1.0.0", deps)
	if code != 42 {
		t.Fatalf("exit code = %d, want 42", code)
	}
	if gotVer != "1.0.0" {
		t.Fatalf("version = %q, want 1.0.0", gotVer)
	}
	if gotCWD != "/tmp/workspace" {
		t.Fatalf("cwd = %q, want /tmp/workspace", gotCWD)
	}
	wantArgs := []string{"tool", "x"}
	if len(gotMCPArgs) != len(wantArgs) {
		t.Fatalf("mcp args = %v, want %v", gotMCPArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotMCPArgs[i] != wantArgs[i] {
			t.Fatalf("mcp args[%d] = %q, want %q", i, gotMCPArgs[i], wantArgs[i])
		}
	}
}

func TestDispatchMain_RoutesCLIForNonMCP(t *testing.T) {
	t.Parallel()

	var (
		gotCLIArgs []string
		gotVer     string
	)
	deps := mainDeps{
		getwd: func() (string, error) {
			t.Fatal("getwd must not be called for CLI dispatch")
			return "", nil
		},
		stdin:  nil,
		stdout: nil,
		stderr: io.Discard,
		runMCP: func([]string, string, string, io.Reader, io.Writer, io.Writer) int {
			t.Fatal("runMCP must not be called for CLI dispatch")
			return 0
		},
		runCLI: func(args []string, version string) int {
			gotCLIArgs = append([]string(nil), args...)
			gotVer = version
			return 7
		},
	}

	code := dispatchMain([]string{"version"}, "dev", deps)
	if code != 7 {
		t.Fatalf("exit code = %d, want 7", code)
	}
	if gotVer != "dev" {
		t.Fatalf("version = %q, want dev", gotVer)
	}
	want := []string{"version"}
	if len(gotCLIArgs) != len(want) {
		t.Fatalf("cli args = %v, want %v", gotCLIArgs, want)
	}
	for i := range want {
		if gotCLIArgs[i] != want[i] {
			t.Fatalf("cli args[%d] = %q, want %q", i, gotCLIArgs[i], want[i])
		}
	}

	codeEmpty := dispatchMain(nil, "v2", deps)
	if codeEmpty != 7 {
		t.Fatalf("empty argv exit = %d, want 7", codeEmpty)
	}
}

func TestDispatchMain_MCPGetwdError(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	deps := mainDeps{
		getwd:  func() (string, error) { return "", errDispatchMainGetwdDenied },
		stdin:  nil,
		stdout: io.Discard,
		stderr: &stderr,
		runMCP: func([]string, string, string, io.Reader, io.Writer, io.Writer) int {
			t.Fatal("runMCP must not run when getwd fails")
			return 0
		},
		runCLI: func([]string, string) int {
			t.Fatal("runCLI must not run for mcp dispatch")
			return 0
		},
	}

	code := dispatchMain([]string{"mcp"}, "1", deps)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	wantStderr := "get working directory: permission denied\n"
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
}
