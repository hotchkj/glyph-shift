//go:build integration

package cli_test

func cliSurfaceCases() []cliCase {
	return []cliCase{
		{
			name:         "CLI_surface_mcp_subcommand_help",
			argv:         []string{"mcp", "--help"},
			wantExit:     cliExitSuccess(),
			stdoutGolden: "surface/stdout/mcp-help.golden",
		},
	}
}
