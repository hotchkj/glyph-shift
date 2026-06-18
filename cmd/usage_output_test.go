package cmd

import (
	"bytes"
	"flag"
	"testing"
)

const topLevelUsageText = `glyph-shift - byte-faithful file operations

Usage: glyph-shift <command> [options]
    or: glyph-shift mcp [options]

Commands:
  extract     Extract a line range from a file
  split       Split a file by delimiter pattern
  blocks      Extract fenced blocks from a file
  transform   Transform line endings and whitespace
  version     Print version

Run as MCP server: glyph-shift mcp [--workspace-root <dir>]

Use "glyph-shift <command> --help" for more information about a command.`

const (
	splitUsageWant = `Usage: glyph-shift split --source <path> --delimiter <regex> --output-dir <dir> [options]

Split a file into multiple files at each line matching the delimiter pattern.

Options:
`
	blocksUsageWant = `Usage: glyph-shift blocks --source <path> --start-line <regex> ` +
		`--end-line <regex> --output-dir <dir> [options]

Extract lines between start and end delimiter patterns into separate files.

Options:
`
	transformUsageWant = `Usage: glyph-shift transform --source <path> [options]

Mechanical line-ending and whitespace transforms. Executes by default; use --preview to inspect without writing.

Options:
`
)

func TestUsagePrintersWriteExpectedHeaders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		run  func(*flag.FlagSet, *bytes.Buffer)
		want string
	}{
		{
			name: "version",
			run:  func(fs *flag.FlagSet, out *bytes.Buffer) { versionUsage(fs, out) },
			want: "Usage: glyph-shift version\n\nPrint the glyph-shift release version string.\n",
		},
		{
			name: "mcp",
			run:  func(fs *flag.FlagSet, out *bytes.Buffer) { mcpUsage(fs, out) },
			want: "Usage: glyph-shift mcp [options]\n\nRun as an MCP (Model Context Protocol) server over stdio.\n\nOptions:\n",
		},
		{
			name: "split",
			run:  func(fs *flag.FlagSet, out *bytes.Buffer) { splitUsage(fs, out) },
			want: splitUsageWant,
		},
		{
			name: "blocks",
			run:  func(fs *flag.FlagSet, out *bytes.Buffer) { blocksUsage(fs, out) },
			want: blocksUsageWant,
		},
		{
			name: "transform",
			run:  func(fs *flag.FlagSet, out *bytes.Buffer) { transformUsage(fs, out) },
			want: transformUsageWant,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fs := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			var out bytes.Buffer
			tc.run(fs, &out)

			if out.String() != tc.want {
				t.Fatalf("usage output = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestPrintUsageWritesTopLevelCommandSummary(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	printUsage(&out)

	if out.String() != topLevelUsageText+"\n" {
		t.Fatalf("usage output = %q", out.String())
	}
}
