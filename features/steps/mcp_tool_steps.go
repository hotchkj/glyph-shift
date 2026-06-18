package steps

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hotchkj/glyph-shift/features/harness"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
)

const errMsgMCPStructuredMissingOpError = "MCP structuredContent did not contain operation error fields"

type operationErrorFields struct {
	Error string
	Hint  string
	Path  string
	Token string
}

// pathContextFromOperationObj returns a single path token for CLI/MCP error parity. Order is
// output-family keys first (dest, output_path, out_dir) then src so split out_dir errors align
// with pipeline error JSON that emits out_dir.
func pathContextFromOperationObj(obj map[string]interface{}) string {
	for _, k := range []string{"dest", "output_path", "out_dir", "src"} {
		if s, ok := obj[k].(string); ok && s != "" {
			return s
		}
	}

	return ""
}

func tokenContextFromOperationObj(obj map[string]interface{}) string {
	for _, k := range []string{"command", "argument", "field"} {
		if s, ok := obj[k].(string); ok && s != "" {
			return s
		}
	}

	return ""
}

// decodeMCPToolTextContentJSON decodes a single JSON object from text and rejects trailing bytes.
func decodeMCPToolTextContentJSON(trimmed string) (map[string]interface{}, error) {
	var contentMap map[string]interface{}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	if err := dec.Decode(&contentMap); err != nil {
		return nil, fmt.Errorf("parse MCP content JSON: %w", err)
	}

	rest := trimmed[dec.InputOffset():]
	if strings.TrimSpace(rest) != "" {
		return nil, fmt.Errorf("%w: %q", errMCPContentJSONTrailingNonJSON, rest)
	}

	return contentMap, nil
}

func operationErrorFieldsFromMap(obj map[string]interface{}) (*operationErrorFields, error) {
	if err := pipeline.ValidateOperationErrorPayload(obj); err != nil {
		return nil, fmt.Errorf("operation error payload: %w", err)
	}

	errClass, ok := obj["error"].(string)
	if !ok {
		return nil, fmt.Errorf(gotTypeFormat, errStderrErrorFieldNotString, obj["error"])
	}

	if errClass == "" {
		return nil, fmt.Errorf("%w: empty error class", errGlyphShiftStderrJSONFieldContract)
	}

	hint, ok := obj["hint"].(string)
	if !ok {
		return nil, fmt.Errorf(gotTypeFormat, errGlyphShiftStderrJSONFieldContract, obj["hint"])
	}

	path := pathContextFromOperationObj(obj)
	token := tokenContextFromOperationObj(obj)

	return &operationErrorFields{
		Error: errClass,
		Hint:  hint,
		Path:  path,
		Token: token,
	}, nil
}

func (tc *TestContext) captureLastMCPToolResult(result *mcp.CallToolResult) error {
	tc.MCPIsError = result.IsError
	tc.MCPError = nil
	tc.MCPStructuredContent = nil
	tc.MCPContentJSON = nil

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return fmt.Errorf("marshal MCP structuredContent: %w", err)
	}

	var structured map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&structured); err != nil {
		return fmt.Errorf("parse MCP structuredContent: %w", err)
	}

	tc.MCPStructuredContent = structured

	fields, classifyErr := operationErrorFieldsFromMap(structured)
	if classifyErr == nil {
		tc.MCPError = fields
	}

	if len(result.Content) == 1 {
		if txt, ok := result.Content[0].(*mcp.TextContent); ok {
			trimmed := strings.TrimSpace(txt.Text)
			if trimmed != "" {
				contentMap, derr := decodeMCPToolTextContentJSON(trimmed)
				if derr != nil {
					return derr
				}

				tc.MCPContentJSON = contentMap
			}
		}
	}

	return nil
}

func registerThenSaveCLIError(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the CLI JSON error is saved$`, func() error {
		obj, err := parseStrictGlyphShiftStderrJSON(tc.Stderr)
		if err != nil {
			return fmt.Errorf(parseStderrJSONFmt, err, tc.Stderr)
		}

		_, err = operationErrorFieldsFromMap(obj)
		if err != nil {
			return err
		}

		return nil
	})
}

func registerWhenMCPTool(sc *godog.ScenarioContext, tc *TestContext) {
	sc.When(`^the MCP tool "([^"]*)" is invoked with JSON:$`, func(toolName string, doc *godog.DocString) error {
		var args map[string]any
		if err := json.Unmarshal([]byte(doc.Content), &args); err != nil {
			return fmt.Errorf("parse MCP tool arguments: %w", err)
		}

		return tc.invokeMCPTool(toolName, args)
	})

	sc.When(
		`^the MCP split tool is invoked for "([^"]*)" by pattern "([^"]*)" into "([^"]*)"$`,
		func(src, pattern, outDir string) error {
			return tc.invokeMCPTool("split", map[string]any{
				"source":     src,
				"delimiter":  pattern,
				"output_dir": outDir,
			})
		},
	)

	sc.When(
		`^the MCP split tool is invoked for "([^"]*)" by a pattern longer than the maximum into "([^"]*)"$`,
		func(src, outDir string) error {
			return tc.invokeMCPTool("split", map[string]any{
				"source":     src,
				"delimiter":  harness.RegexPatternLongerThanMaximum(),
				"output_dir": outDir,
			})
		},
	)

	sc.When(
		`^the MCP split tool is invoked for "([^"]*)" by a pattern containing a control character into "([^"]*)"$`,
		func(src, outDir string) error {
			return tc.invokeMCPTool("split", map[string]any{
				"source":     src,
				"delimiter":  harness.RegexPatternWithControlCharacter(),
				"output_dir": outDir,
			})
		},
	)
}

// invokeMCPTool routes MCP tool assertions through invokeMCPToolMocked using the workspace mock runner.
func (tc *TestContext) invokeMCPTool(toolName string, args map[string]any) error {
	return tc.invokeMCPToolMocked(toolName, args)
}

func registerThenMCPStructuredContentFieldInt(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the MCP structuredContent field "([^"]*)" is (\d+)$`, func(field, nStr string) error {
		want, err := strconv.Atoi(nStr)
		if err != nil {
			return fmt.Errorf("parse MCP field int: %w", err)
		}

		if tc.MCPStructuredContent == nil {
			return errMCPStructuredContentNil
		}

		raw, exists := tc.MCPStructuredContent[field]
		if !exists {
			return fmt.Errorf("%w: %q", errStdoutJSONMissingField, field)
		}

		if !jsonInt64MatchingExactInt(raw, int64(want)) {
			return fmt.Errorf("%w: field %q want %d got %v", errStdoutJSONFieldValueMismatch, field, want, raw)
		}

		return nil
	})
}

func registerThenMCPToolOperationError(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Then(`^the MCP tool result indicates an operation error$`, func() error {
		if !tc.MCPIsError {
			return fmt.Errorf("%w: MCP result isError=false", errExpectedNonzeroExit)
		}
		if tc.MCPError == nil {
			return fmt.Errorf("%w: %s", errGlyphShiftStderrJSONFieldContract, errMsgMCPStructuredMissingOpError)
		}

		return nil
	})
}

// RegisterMCPParity registers CLI/MCP error parity contract steps.
func RegisterMCPParity(sc *godog.ScenarioContext, tc *TestContext) {
	registerWhenMCPTool(sc, tc)
	registerThenMCPStructuredContentFieldInt(sc, tc)
	registerThenMCPToolOperationError(sc, tc)
	registerThenSaveCLIError(sc, tc)
}
