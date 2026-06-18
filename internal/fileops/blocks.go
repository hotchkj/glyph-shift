package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
)

var (
	errBlocksNilStart = errors.New("start delimiter is required")
	errBlocksNilEnd   = errors.New("end delimiter is required")
)

const blocksErrorFormat = "blocks: %w"

// BlocksOptions configures block extraction between start/end delimiter patterns.
type BlocksOptions struct {
	Source            io.Reader
	StartDelimiter    *regexp.Regexp
	EndDelimiter      *regexp.Regexp
	Naming            NamingStrategy
	IncludeDelimiters bool
	Extension         string // includes leading dot
}

// Block is one extracted block with filename stem resolved and line content.
type Block struct {
	Name  string
	Lines []Line
}

// BlocksResult holds extracted blocks and optional warnings (non-error diagnostics).
type BlocksResult struct {
	// Blocks lists only blocks that produce an output file (non-empty block body; empty blocks are omitted).
	Blocks []Block
	// BlocksFound is the total count of blocks closed by an end delimiter, including empty inner blocks.
	BlocksFound int
	Warnings    []string
}

// ExtractBlocks reads lines from Source and extracts blocks between delimiters. It is a helper API returning
// []Block with []Line payloads (not the pipeline byte-span apply path).
//
// This helper does not build results from a ScanBlocksMeta pass: it materializes the entire source via
// ReadLinesFromContext, then scans those lines in memory. Memory use tracks full source size—callers must not
// equate that with production bounded-memory scanning.
//
// Pipeline RunBlocks uses fileops.ScanBlocksMeta plus byte-span replay with fingerprint verification; that is
// a different contract from the full-line helper stack here. For bounded metadata scans that avoid buffering
// whole lines during the seekable scan phase, see ScanBlocksMeta docs.
func ExtractBlocks(ctx context.Context, opts BlocksOptions) (BlocksResult, error) {
	if opts.StartDelimiter == nil {
		return BlocksResult{}, fmt.Errorf(blocksErrorFormat, errBlocksNilStart)
	}

	if opts.EndDelimiter == nil {
		return BlocksResult{}, fmt.Errorf(blocksErrorFormat, errBlocksNilEnd)
	}

	if err := ctx.Err(); err != nil {
		return BlocksResult{}, fmt.Errorf(blocksErrorFormat, err)
	}

	lines, err := ReadLinesFromContext(ctx, opts.Source)
	if err != nil {
		return BlocksResult{}, fmt.Errorf(blocksErrorFormat, err)
	}

	ext := opts.Extension

	return scanBlocks(lines, opts.StartDelimiter, opts.EndDelimiter, opts.Naming, opts.IncludeDelimiters, ext)
}

func scanBlocks(
	lines []Line,
	startRE, endRE *regexp.Regexp,
	strategy NamingStrategy,
	includeDelims bool,
	ext string,
) (BlocksResult, error) {
	var blocks []Block

	existing := make(map[string]bool)
	inside := false

	var startLine Line

	var blockStartLineNum int

	var inner []Line

	emitSeq := 1
	blocksFound := 0

	for lineIdx := range lines {
		line := lines[lineIdx]
		lineStr := string(line.Content)

		if !inside {
			if startRE.MatchString(lineStr) {
				inside = true
				startLine = line
				blockStartLineNum = lineIdx + 1
				inner = nil
			}

			continue
		}

		if endRE.MatchString(lineStr) {
			blocksFound++
			block, ok := buildBlock(&blockBuildOptions{
				startLine:     startLine,
				inner:         inner,
				endLine:       line,
				strategy:      strategy,
				includeDelims: includeDelims,
				ext:           ext,
				seq:           emitSeq,
				existing:      existing,
			})
			if ok {
				blocks = append(blocks, block)
				emitSeq++
			}

			inside = false
			inner = nil

			continue
		}

		inner = append(inner, line)
	}

	if err := validateCompletedBlocksScan(inside, blockStartLineNum, blocksFound); err != nil {
		return BlocksResult{}, err
	}

	return BlocksResult{Blocks: blocks, BlocksFound: blocksFound, Warnings: nil}, nil
}

func validateCompletedBlocksScan(inside bool, blockStartLineNum, blocksFound int) error {
	if inside {
		return &UnclosedBlockDetailError{StartLine: blockStartLineNum}
	}

	if blocksFound == 0 {
		return fmt.Errorf("%w: start and end patterns did not match any complete blocks", ErrNoBlocksFound)
	}

	return nil
}

type blockBuildOptions struct {
	startLine     Line
	inner         []Line
	endLine       Line
	strategy      NamingStrategy
	includeDelims bool
	ext           string
	seq           int
	existing      map[string]bool
}

func buildBlock(opts *blockBuildOptions) (Block, bool) {
	if len(opts.inner) == 0 {
		return Block{}, false
	}

	var out []Line

	if opts.includeDelims {
		out = append(append(append([]Line(nil), opts.startLine), opts.inner...), opts.endLine)
	} else {
		out = append([]Line(nil), opts.inner...)
	}

	name := chooseBlockFilename(opts.strategy, opts.seq, opts.startLine, opts.inner, opts.ext, opts.existing)

	return Block{Name: name, Lines: out}, true
}

func chooseBlockFilename(
	strategy NamingStrategy,
	seq int,
	startLine Line,
	inner []Line,
	ext string,
	existing map[string]bool,
) string {
	filenameStrategy := Sequential
	text := ""

	switch strategy {
	case Sequential:
	case FromDelimiter:
		filenameStrategy = FromDelimiter
		text = string(startLine.Content)
	case FromContent:
		filenameStrategy = FromContent
		text = string(inner[0].Content)
	}

	base := GenerateFilename(filenameStrategy, seq, text, ext)

	return DeduplicateFilename(base, existing)
}
