package steps

import "github.com/cucumber/godog"

// RegisterBlocks registers blocks-specific step definitions.
// Shared steps (directory file count, file exists, starts with, stderr, etc.) come from
// RegisterCommon, RegisterExtract, and RegisterSplit.
func RegisterBlocks(sc *godog.ScenarioContext, tc *TestContext) {
	RegisterBlocksWhen(sc, tc)
}
