package steps

import (
	"fmt"

	"github.com/cucumber/godog"
)

func blocksFlagForOption(option string) (string, error) {
	switch option {
	case "include delimiters":
		return "--include-delimiters", nil
	case stepOptOverwrite:
		return stepFlagForce, nil
	case stepOptCreateDirectories:
		return stepFlagMkdir, nil
	default:
		return "", fmt.Errorf("%w: %q", errUnknownBlocksOption, option)
	}
}

func registerWhenBlocks(sc *godog.ScenarioContext, tc *TestContext) {
	// When the caller extracts blocks from "<src>" between "<start>" and "<end>" into "<dir>"
	sc.When(
		`^the caller extracts blocks from "([^"]*)" between "([^"]*)" and "([^"]*)" into "([^"]*)"$`,
		func(src, start, end, dir string) error {
			return runBlocksDirect(tc, blocksDirectInput{Src: src, Start: start, End: end, OutDir: dir})
		},
	)

	// When the caller extracts blocks from "<src>" between "<start>" and "<end>" into "<dir>" with "<option>"
	sc.When(
		`^the caller extracts blocks from "([^"]*)" between "([^"]*)" and "([^"]*)" into "([^"]*)" with "([^"]*)"$`,
		func(src, start, end, dir, option string) error {
			flag, err := blocksFlagForOption(option)
			if err != nil {
				return err
			}

			in := blocksDirectInput{Src: src, Start: start, End: end, OutDir: dir}

			switch flag {
			case "--include-delimiters":
				in.IncludeDelimiters = true
			case stepFlagForce:
				in.Force = true
			case stepFlagMkdir:
				in.Mkdir = true
			}

			return runBlocksDirect(tc, in)
		},
	)

	// When the caller extracts blocks from "<src>" between "<start>" and "<end>" into "<dir>" named "<names>"
	sc.When(
		`^the caller extracts blocks from "([^"]*)" between "([^"]*)" and "([^"]*)" into "([^"]*)" named "([^"]*)"$`,
		func(src, start, end, dir, names string) error {
			return runBlocksDirect(tc, blocksDirectInput{
				Src: src, Start: start, End: end, OutDir: dir, NamesRaw: names,
			})
		},
	)

	// When the caller extracts blocks from "<src>" between "<start>" and "<end>" into "<dir>" with a max-files limit of N
	//nolint:lll // Godog step regex; length exceeds wrap budget intentionally.
	sc.When(
		`^the caller extracts blocks from "([^"]*)" between "([^"]*)" and "([^"]*)" into "([^"]*)" with a max-files limit of (\d+)$`,
		func(src, start, end, dir string, maxFiles int) error {
			return runBlocksDirect(tc, blocksDirectInput{
				Src: src, Start: start, End: end, OutDir: dir,
				MaxFilesSpecified: true, MaxFiles: maxFiles,
			})
		},
	)

	// When the caller previews extracting blocks from "<src>" between "<start>" and "<end>" into "<dir>"
	sc.When(
		`^the caller previews extracting blocks from "([^"]*)" between "([^"]*)" and "([^"]*)" into "([^"]*)"$`,
		func(src, start, end, dir string) error {
			return runBlocksDirect(tc, blocksDirectInput{
				Src: src, Start: start, End: end, OutDir: dir, Preview: true,
			})
		},
	)
}

// RegisterBlocksWhen registers blocks When step definitions.
func RegisterBlocksWhen(sc *godog.ScenarioContext, tc *TestContext) {
	registerWhenBlocks(sc, tc)
}
