package steps

import (
	"github.com/cucumber/godog"
)

func registerWhenExtract(sc *godog.ScenarioContext, tc *TestContext) {
	// When the caller extracts lines <range> from "<src>" to "<dst>"
	sc.When(
		`^the caller extracts lines ([\d-]+) from "([^"]*)" to "([^"]*)"$`,
		func(lines, src, dst string) error {
			return runExtractDirect(tc, lines, src, dst)
		},
	)

	sc.When(
		`^the caller extracts lines "([^"]*)" from "([^"]*)" to "([^"]*)"$`,
		func(lines, src, dst string) error {
			return runExtractDirect(tc, lines, src, dst)
		},
	)

	// When the caller extracts lines <range> from "<src>" to "<dst>" with "<option>"
	sc.When(
		`^the caller extracts lines ([\d-]+) from "([^"]*)" to "([^"]*)" with "([^"]*)"$`,
		func(lines, src, dst, option string) error {
			return runExtractDirect(tc, lines, src, dst, option)
		},
	)

	sc.When(
		`^the caller extracts lines "([^"]*)" from "([^"]*)" to "([^"]*)" with "([^"]*)"$`,
		func(lines, src, dst, option string) error {
			return runExtractDirect(tc, lines, src, dst, option)
		},
	)

	// When the caller previews extracting lines <range> from "<src>" to "<dst>"
	sc.When(
		`^the caller previews extracting lines ([\d-]+) from "([^"]*)" to "([^"]*)"$`,
		func(lines, src, dst string) error {
			return runExtractDirect(tc, lines, src, dst, stepFlagPreview)
		},
	)

	sc.When(
		`^the caller previews extracting lines "([^"]*)" from "([^"]*)" to "([^"]*)"$`,
		func(lines, src, dst string) error {
			return runExtractDirect(tc, lines, src, dst, stepFlagPreview)
		},
	)
}

// RegisterExtractWhen registers extract When step definitions.
func RegisterExtractWhen(sc *godog.ScenarioContext, tc *TestContext) {
	registerWhenExtract(sc, tc)
}
