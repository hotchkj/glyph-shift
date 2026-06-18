package steps

import (
	"github.com/cucumber/godog"
)

func registerWhenMCPHelp(sc *godog.ScenarioContext, tc *TestContext) {
	sc.When(`^the caller requests MCP help$`, func() error {
		return tc.runGlyphShiftSubcommand([]string{"mcp", "--help"})
	})
}

// RegisterCLISurface registers non-operation CLI surface smoke steps.
func RegisterCLISurface(sc *godog.ScenarioContext, tc *TestContext) {
	registerWhenMCPHelp(sc, tc)
}
