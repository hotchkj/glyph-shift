package steps

import "github.com/cucumber/godog"

// RegisterTransform registers transform-specific step definitions.
func RegisterTransform(sc *godog.ScenarioContext, tc *TestContext) {
	registerTransformGivenLineEndings(sc, tc)
	registerTransformGivenDirs(sc, tc)
	registerTransformGivenContent(sc, tc)
	registerTransformThenEveryLineTerminator(sc, tc)
	registerTransformThenNoTrailingWS(sc, tc)
	registerTransformThenNewlines(sc, tc)
	RegisterTransformWhen(sc, tc)
	RegisterTransformExtra(sc, tc)
	RegisterTransformPerformance(sc, tc)
}
