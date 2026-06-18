package steps

import "github.com/cucumber/godog"

// RegisterCommon registers shared step definitions.
func RegisterCommon(sc *godog.ScenarioContext, tc *TestContext) {
	registerNumberedSource(sc, tc)
	registerDocstringSource(sc, tc)
	registerExistingFiles(sc, tc)
	registerWorkspaceSymlinkMap(sc, tc)
	RegisterFailureStreamProof(sc, tc)
	registerThenExitCode(sc, tc)
	registerThenFileContent(sc, tc)
	registerThenLineCount(sc, tc)
}
