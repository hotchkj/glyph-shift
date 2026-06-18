package steps

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cucumber/godog"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

// layer2SplitBlocksContractOutDir is the output directory used in split/blocks JSON contract
// scenarios (see bdd/core/split_contract.feature and blocks_contract.feature).
const (
	layer2SplitBlocksContractOutDir   = "out"
	layer2DestinationExistsSentinel   = "destination_exists"
	layer2ExtractContractDest         = "output.txt"
	layer2SplitBlocksFirstOutputPath  = "out/001.md"
	layer2PatternMockHintFormat       = "bdd mock: %w"
	layer2UnusedTransformDestFallback = "unused-transform-dest"
	layer2QuotedErrorFormat           = "%w: %q"
	layer2OperationSplit              = "split"
	layer2OperationBlocks             = "blocks"
	layer2SentinelInvalidPattern      = "invalid_pattern"
	layer2SentinelPatternTooLong      = "pattern_too_long"
	layer2SentinelControlCharsInput   = "control_chars_in_input"
)

var errLayer2DestinationPathRequired = errors.New("layer2 destination path required")

var errLayer2PatternLogicalFieldRequired = errors.New(
	"pattern logical field required for pattern validation sentinel",
)

var errLayer2UnsupportedPatternLogicalField = errors.New("unsupported pattern logical field")

var errLayer2UnsupportedPatternOperation = errors.New("unsupported pattern validation operation")

func ensureLayer2PatternLogicalField(operation, field string) error {
	switch operation {
	case layer2OperationSplit:
		if field == "delimiter" {
			return nil
		}
	case layer2OperationBlocks:
		if field == "start_line" || field == "end_line" {
			return nil
		}
	default:
		return fmt.Errorf(layer2QuotedErrorFormat, errLayer2UnsupportedPatternOperation, operation)
	}

	return fmt.Errorf(layer2QuotedErrorFormat, errLayer2UnsupportedPatternLogicalField, field)
}

func isLayer2PatternValidationSentinel(name string) bool {
	switch name {
	case layer2SentinelInvalidPattern, layer2SentinelPatternTooLong, layer2SentinelControlCharsInput:
		return true
	default:
		return false
	}
}

func patternValidationSentinelError(name, logicalField string) (patternErr, mapErr error) {
	switch logicalField {
	case "":
		return nil, fmt.Errorf(layer2QuotedErrorFormat, errLayer2PatternLogicalFieldRequired, name)
	default:
	}

	var causeErr error

	switch name {
	case layer2SentinelPatternTooLong:
		causeErr = fmt.Errorf(layer2PatternMockHintFormat, validate.ErrPatternTooLong)
	case layer2SentinelControlCharsInput:
		causeErr = fmt.Errorf(layer2PatternMockHintFormat, validate.ErrControlChar)
	case layer2SentinelInvalidPattern:
		causeErr = fmt.Errorf(layer2PatternMockHintFormat, validate.ErrInvalidPattern)
	default:
		return nil, fmt.Errorf(layer2QuotedErrorFormat, errUnknownPipelineSentinel, name)
	}

	return &pipeline.PatternFieldError{Field: logicalField, Cause: causeErr}, nil
}

func pipelineSentinelErrors() map[string]error {
	// Structured detail types are required for JSON contract variants that mandate integer fields
	// (see internal/pipeline/error_contract.go, docs/glyph-shift-json-contract.md). Bare sentinels alone
	// classify as internal_error; mocks use the typed errors production paths emit.
	//
	// Pattern validation mocks (invalid_pattern, pattern_too_long, control_chars_in_input) are built
	// in sentinelErr with Split vs Blocks logical field names per docs/glyph-shift-json-contract.md.
	return map[string]error{
		"range_exceeds_file": &fileops.RangeExceedsFileError{
			FileLines:  50,
			RangeStart: 45,
			RangeEnd:   120,
		},
		"empty_range": &fileops.EmptyRangeError{Start: 2, End: 1},
		"unclosed_block": &fileops.UnclosedBlockDetailError{
			StartLine: 12,
		},
		"no_blocks_found":    fileops.ErrNoBlocksFound,
		"no_delimiter_match": fileops.ErrNoDelimiterMatch,
		"source_not_found":   pipeline.ErrSourceNotFound,
		"binary_source":      pipeline.ErrBinarySource,
		"not_regular_file":   pipeline.ErrNotRegularFile,
		"directory_not_file": pipeline.ErrDirectoryNotFile,
		"max_files_exceeded": &fileops.MaxFilesExceededDetailError{
			MaxFiles:         10,
			WouldCreateCount: 11,
		},
		"names_count_mismatch": &pipeline.NamesCountMismatchError{
			NamesCount: 2, OutputCount: 3,
		},
		"invalid_line_endings":      pipeline.ErrInvalidLineEndings,
		"no_transform_specified":    pipeline.ErrNoTransformSpecified,
		"transform_skipped_unknown": pipeline.ErrTransformSkippedUnknown,
	}
}

func ensureMockRunner(tc *TestContext) *MockRunner {
	if tc.MockRunner == nil {
		tc.MockRunner = &MockRunner{}
	}

	return tc.MockRunner
}

func parseCommaSeparatedBasenames(list string) []string {
	parts := strings.Split(list, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			out = append(out, s)
		}
	}

	return out
}

// layer2AbsolutePathsForOutBasenames maps output basenames from contract Given steps to absolute
// native paths under the BDD workspace (matching real pipeline RunSplit/RunBlocks pres.Files).
func layer2AbsolutePathsForOutBasenames(tc *TestContext, outDirLogical, filesCSV string) []string {
	basenames := parseCommaSeparatedBasenames(filesCSV)
	out := make([]string, 0, len(basenames))

	for _, b := range basenames {
		rel := filepath.ToSlash(filepath.Join(outDirLogical, b))
		out = append(out, tc.Ws.Join(rel))
	}

	return out
}

func sentinelErr(name, destinationPath string) (mapped, mapErr error) {
	if name == layer2DestinationExistsSentinel {
		if destinationPath == "" {
			return nil, fmt.Errorf(layer2QuotedErrorFormat, errLayer2DestinationPathRequired, name)
		}

		return &pipeline.DestinationExistsError{Path: destinationPath}, nil
	}

	switch name {
	case layer2SentinelInvalidPattern, layer2SentinelPatternTooLong, layer2SentinelControlCharsInput:
		return nil, fmt.Errorf(layer2QuotedErrorFormat, errLayer2PatternLogicalFieldRequired, name)
	default:
	}

	sentinelMap := pipelineSentinelErrors()
	pipelineErr, ok := sentinelMap[name]
	if !ok {
		return nil, fmt.Errorf(layer2QuotedErrorFormat, errUnknownPipelineSentinel, name)
	}

	return pipelineErr, nil
}

type layer2OperationErrorSet struct {
	extractErr   error
	splitErr     error
	blocksErr    error
	transformErr error
}

func layer2OperationErrorsForSentinel(tc *TestContext, sentinel string) (layer2OperationErrorSet, error) {
	if isLayer2PatternValidationSentinel(sentinel) {
		return layer2OperationErrorSet{}, fmt.Errorf(layer2QuotedErrorFormat, errLayer2PatternLogicalFieldRequired, sentinel)
	}

	var errs layer2OperationErrorSet

	extractErr, err := sentinelErr(sentinel, tc.Ws.Join(layer2ExtractContractDest))
	if err != nil {
		return layer2OperationErrorSet{}, err
	}

	transformErr, err := sentinelErr(sentinel, tc.Ws.Join(layer2UnusedTransformDestFallback))
	if err != nil {
		return layer2OperationErrorSet{}, err
	}

	errs.extractErr = extractErr
	errs.transformErr = transformErr

	splitErr, err := sentinelErr(sentinel, tc.Ws.Join(layer2SplitBlocksFirstOutputPath))
	if err != nil {
		return layer2OperationErrorSet{}, err
	}

	blocksErr, err := sentinelErr(sentinel, tc.Ws.Join(layer2SplitBlocksFirstOutputPath))
	if err != nil {
		return layer2OperationErrorSet{}, err
	}

	errs.splitErr = splitErr
	errs.blocksErr = blocksErr

	return errs, nil
}

func setLayer2PatternValidationError(tc *TestContext, operation, sentinel, field string) error {
	if err := ensureLayer2PatternLogicalField(operation, field); err != nil {
		return err
	}

	patternErr, err := patternValidationSentinelError(sentinel, field)
	if err != nil {
		return err
	}

	mockRunner := ensureMockRunner(tc)

	switch operation {
	case layer2OperationSplit:
		mockRunner.splitErr = patternErr
	case layer2OperationBlocks:
		mockRunner.blocksErr = patternErr
	default:
		return fmt.Errorf(layer2QuotedErrorFormat, errLayer2UnsupportedPatternOperation, operation)
	}

	return nil
}

//nolint:varnamelen // Godog step glue uses short names consistent with other step files.
func registerLayer2ExtractSplitBlocksMocks(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^the extract operation reports success extracting (\d+) lines$`, func(nStr string) error {
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse line count: %w", err)
		}

		m := ensureMockRunner(tc)
		// Contract scenarios use --destination output.txt; cmd preview JSON reads WouldCreatePath.
		m.extractResult = fileops.ExtractResult{
			LinesExtracted:  n,
			WouldCreatePath: tc.Ws.Join(layer2ExtractContractDest),
		}
		m.extractErr = nil

		return nil
	})

	sc.Given(`^the split operation reports success with files "([^"]*)"$`, func(filesCSV string) error {
		m := ensureMockRunner(tc)
		m.splitResult = pipeline.SplitPipelineResult{
			Files: layer2AbsolutePathsForOutBasenames(tc, layer2SplitBlocksContractOutDir, filesCSV),
		}
		m.splitErr = nil

		return nil
	})

	//nolint:lll // Godog step regex; length exceeds wrap budget intentionally.
	sc.Given(
		`^the blocks operation reports success with (\d+) non-empty blocks? and (\d+) empty blocks? creating files "([^"]*)"$`,
		func(contentStr, emptyStr, filesCSV string) error {
			contentN, err := strconv.Atoi(contentStr)
			if err != nil {
				return fmt.Errorf("parse non-empty blocks: %w", err)
			}

			emptyN, err := strconv.Atoi(emptyStr)
			if err != nil {
				return fmt.Errorf("parse empty blocks: %w", err)
			}

			files := layer2AbsolutePathsForOutBasenames(tc, layer2SplitBlocksContractOutDir, filesCSV)
			if len(files) != contentN {
				return fmt.Errorf("%w: got %d files for %d content blocks", errBlocksFileCountMismatch, len(files), contentN)
			}

			m := ensureMockRunner(tc)
			m.blocksResult = pipeline.BlocksPipelineResult{
				BlocksFound: contentN + emptyN,
				Files:       files,
			}
			m.blocksErr = nil

			return nil
		},
	)
}

//nolint:lll,varnamelen // Godog regex length; short names match existing step style.
func registerLayer2TransformLineEndingMocks(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^the transform operation reports (\d+) LF endings converted to CRLF$`, func(nStr string) error {
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse ending stats: %w", err)
		}

		m := ensureMockRunner(tc)
		m.transformResult = pipeline.TransformPipelineResult{
			Result: fileops.TransformFileResult{
				WouldChange:    true,
				EndingsChanged: n,
				LFFound:        n,
				LFConverted:    n,
				CRFound:        0,
				CRConverted:    0,
				CRLFFound:      0,
				CRLFConverted:  0,
				Skipped:        false,
				SkipReason:     "",
			},
			ChangeCount: n,
		}
		m.transformErr = nil

		return nil
	})
}

//nolint:lll,varnamelen // Godog regex length; short names match existing step style.
func registerLayer2TransformWhitespaceMocks(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^the transform operation reports (\d+) line trimmed of trailing whitespace$`, func(nStr string) error {
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse trailing trimmed: %w", err)
		}

		m := ensureMockRunner(tc)
		m.transformResult = pipeline.TransformPipelineResult{
			Result: fileops.TransformFileResult{
				WouldChange:     true,
				TrailingTrimmed: n,
				Skipped:         false,
				SkipReason:      "",
			},
			ChangeCount: n,
		}
		m.transformErr = nil

		return nil
	})

	sc.Given(`^the transform operation reports a final newline was added$`, func() error {
		m := ensureMockRunner(tc)
		m.transformResult = pipeline.TransformPipelineResult{
			Result: fileops.TransformFileResult{
				WouldChange:       true,
				FinalNewlineAdded: true,
				Skipped:           false,
				SkipReason:        "",
			},
		}
		m.transformErr = nil

		return nil
	})

	sc.Given(`^the transform operation reports no final newline change was needed$`, func() error {
		m := ensureMockRunner(tc)
		m.transformResult = pipeline.TransformPipelineResult{
			Result: fileops.TransformFileResult{
				WouldChange:       false,
				FinalNewlineAdded: false,
				Skipped:           false,
				SkipReason:        "",
			},
		}
		m.transformErr = nil

		return nil
	})
}

//nolint:varnamelen // Godog step glue uses short names consistent with other step files.
func registerLayer2FailureMocks(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^the extract operation reports error "([^"]*)"$`, func(sentinel string) error {
		e, err := sentinelErr(sentinel, tc.Ws.Join(layer2ExtractContractDest))
		if err != nil {
			return err
		}

		m := ensureMockRunner(tc)
		m.extractErr = e

		return nil
	})

	sc.Given(`^the operation reports error "([^"]*)"$`, func(sentinel string) error {
		errs, err := layer2OperationErrorsForSentinel(tc, sentinel)
		if err != nil {
			return err
		}

		m := ensureMockRunner(tc)
		m.extractErr = errs.extractErr
		m.splitErr = errs.splitErr
		m.blocksErr = errs.blocksErr
		m.transformErr = errs.transformErr

		return nil
	})

	//nolint:lll // Godog step regex; the phrase keeps operation and logical JSON field explicit.
	sc.Given(
		`^the (split|blocks) operation reports pattern validation error "([^"]*)" for field "([^"]*)"$`,
		func(operation, sentinel, field string) error {
			return setLayer2PatternValidationError(tc, operation, sentinel, field)
		},
	)
}

func registerLayer2FileFixtureGiven(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^a source file "([^"]*)" with content "([^"]*)"$`, func(name, text string) error {
		return writeSourceFile(tc, name, []byte(unescapeContent(text)))
	})
}

func registerLayer2When(sc *godog.ScenarioContext, tc *TestContext) {
	sc.When(`^the CLI command is invoked as "([^"]*)"$`, func(cmdLine string) error {
		fields := strings.Fields(cmdLine)
		if len(fields) == 0 {
			return errEmptyCLIRenderLine
		}

		if fields[0] != "glyph-shift" {
			return fmt.Errorf("%w: got %q", errCLIRenderMissingGlyphShift, fields[0])
		}

		args := fields[1:]

		return tc.runGlyphShiftMocked(args)
	})

	sc.When(`^the MCP tool "([^"]*)" is called with JSON:$`, func(toolName string, doc *godog.DocString) error {
		var args map[string]any
		if err := json.Unmarshal([]byte(doc.Content), &args); err != nil {
			return fmt.Errorf("parse MCP tool JSON: %w", err)
		}

		return tc.invokeMCPToolMocked(toolName, args)
	})
}

func registerLayer2Then(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the MCP tool result indicates success$`, func() error {
		if tc.MCPIsError {
			return errExpectedMCPSuccessGotError
		}

		return nil
	})

	sc.Then(`^the MCP structuredContent is exactly:$`, func(doc *godog.DocString) error {
		if tc.MCPStructuredContent == nil {
			return errMCPStructuredContentNil
		}

		return assertJSONSpecExactMatch(tc.MCPStructuredContent, doc)
	})

	sc.Then(
		`^the MCP structuredContent validates against the MCP outputSchema for tool "([^"]*)"$`,
		func(toolName string) error {
			if tc.MCPStructuredContent == nil {
				return errMCPStructuredContentNil
			}

			return ValidateMCPStructuredContentAgainstToolDeclaredOutputSchema(toolName, tc.MCPStructuredContent)
		},
	)
}

// RegisterLayer2 registers Given/When/Then for JSON contract scenarios (stubbed operation results).
func RegisterLayer2(sc *godog.ScenarioContext, tc *TestContext) {
	registerLayer2ExtractSplitBlocksMocks(sc, tc)
	registerLayer2TransformLineEndingMocks(sc, tc)
	registerLayer2TransformWhitespaceMocks(sc, tc)
	registerLayer2FailureMocks(sc, tc)
	registerLayer2FileFixtureGiven(sc, tc)
	registerLayer2When(sc, tc)
	registerLayer2Then(sc, tc)
}
