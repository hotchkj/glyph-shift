package steps

import (
	"fmt"

	"github.com/cucumber/godog"
	"github.com/hotchkj/glyph-shift/features/harness"
)

func splitFlagForOption(option string) (string, error) {
	switch option {
	case "strip delimiters":
		return "--strip-delimiter", nil
	case stepOptOverwrite:
		return stepFlagForce, nil
	case stepOptCreateDirectories:
		return stepFlagMkdir, nil
	default:
		return "", fmt.Errorf("%w: %q", errUnknownSplitOption, option)
	}
}

func registerWhenSplit(sc *godog.ScenarioContext, tc *TestContext) {
	// When the caller splits "<src>" by pattern "<pat>" into "<dir>"
	sc.When(
		`^the caller splits "([^"]*)" by pattern "([^"]*)" into "([^"]*)"$`,
		func(src, pat, dir string) error {
			return runSplitDirect(tc, splitDirectInput{Src: src, Delimiter: pat, OutDir: dir})
		},
	)

	sc.When(
		`^the caller splits "([^"]*)" by a pattern longer than the maximum into "([^"]*)"$`,
		func(src, dir string) error {
			return runSplitDirect(tc, splitDirectInput{
				Src: src, Delimiter: harness.RegexPatternLongerThanMaximum(), OutDir: dir,
			})
		},
	)

	sc.When(
		`^the caller splits "([^"]*)" by a pattern containing a control character into "([^"]*)"$`,
		func(src, dir string) error {
			return runSplitDirect(tc, splitDirectInput{
				Src: src, Delimiter: harness.RegexPatternWithControlCharacter(), OutDir: dir,
			})
		},
	)

	// When the caller splits "<src>" by pattern "<pat>" into "<dir>" with "<option>"
	sc.When(
		`^the caller splits "([^"]*)" by pattern "([^"]*)" into "([^"]*)" with "([^"]*)"$`,
		func(src, pat, dir, option string) error {
			flag, err := splitFlagForOption(option)
			if err != nil {
				return err
			}

			in := splitDirectInput{Src: src, Delimiter: pat, OutDir: dir}

			switch flag {
			case "--strip-delimiter":
				in.StripDelimiter = true
			case stepFlagForce:
				in.Force = true
			case stepFlagMkdir:
				in.Mkdir = true
			}

			return runSplitDirect(tc, in)
		},
	)

	// When the caller splits "<src>" by pattern "<pat>" into "<dir>" with a max-files limit of N
	sc.When(
		`^the caller splits "([^"]*)" by pattern "([^"]*)" into "([^"]*)" with a max-files limit of (\d+)$`,
		func(src, pat, dir string, maxFiles int) error {
			return runSplitDirect(tc, splitDirectInput{
				Src: src, Delimiter: pat, OutDir: dir,
				MaxFilesSpecified: true, MaxFiles: maxFiles,
			})
		},
	)

	// When the caller splits "<src>" by pattern "<pat>" into "<dir>" named "<names>"
	sc.When(
		`^the caller splits "([^"]*)" by pattern "([^"]*)" into "([^"]*)" named "([^"]*)"$`,
		func(src, pat, dir, names string) error {
			return runSplitDirect(tc, splitDirectInput{Src: src, Delimiter: pat, OutDir: dir, NamesRaw: names})
		},
	)

	// When the caller previews splitting "<src>" by pattern "<pat>" into "<dir>"
	sc.When(
		`^the caller previews splitting "([^"]*)" by pattern "([^"]*)" into "([^"]*)"$`,
		func(src, pat, dir string) error {
			return runSplitDirect(tc, splitDirectInput{Src: src, Delimiter: pat, OutDir: dir, Preview: true})
		},
	)
}

// RegisterSplitWhen registers split When step definitions.
func RegisterSplitWhen(sc *godog.ScenarioContext, tc *TestContext) {
	registerWhenSplit(sc, tc)
}
