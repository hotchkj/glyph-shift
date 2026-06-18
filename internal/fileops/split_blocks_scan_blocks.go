package fileops

import (
	"context"
	"fmt"
	"io"
)

// blocksErrUnclosedWithStartLine wraps ErrUnclosedBlock with the 1-based start delimiter line number
// for actionable diagnostics while preserving errors.Is(.., ErrUnclosedBlock).
func blocksErrUnclosedWithStartLine(startLineNum int) error {
	if startLineNum <= 0 {
		return ErrUnclosedBlock
	}

	return &UnclosedBlockDetailError{StartLine: startLineNum}
}

func scanBlocksMetaSeekable(
	ctx context.Context,
	rs io.ReadSeeker,
	opts BlocksOptions,
	limits BoundedScanLimits,
	lineScanOpts linespanScanOptions,
) (BlocksBoundedScanResult, error) {
	sc := blocksSeekScanner{
		opts:     opts,
		limits:   limits,
		rs:       rs,
		existing: make(map[string]bool),
	}

	err := scanLineSpansWithOptions(ctx, rs, func(span LineSpan) error {
		return isolateMatchLineSpanOnSharedSeeker(rs, span, func() error {
			return sc.handleLineSpan(ctx, span)
		})
	}, lineScanOpts)
	if err != nil {
		return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, err)
	}

	if sc.inside {
		return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, blocksErrUnclosedWithStartLine(sc.blockStartLineNum))
	}

	if sc.blocksFound == 0 {
		return BlocksBoundedScanResult{}, fmt.Errorf(blocksErrorFormat, ErrNoBlocksFound)
	}

	return BlocksBoundedScanResult{
		Metas:                sc.metas,
		BlocksFound:          sc.blocksFound,
		OutputBlockFileCount: len(sc.metas),
		EmptyBlocksDiscarded: sc.emptyDiscarded,
	}, nil
}

type blocksSeekScanner struct {
	opts   BlocksOptions
	limits BoundedScanLimits
	rs     io.ReadSeeker

	existing map[string]bool

	inside bool

	startSpan LineSpan

	innerCount        int
	firstInnerSeen    bool
	firstInnerSpan    LineSpan
	lastInnerSpan     LineSpan
	blockStartLineNum int

	blocksFound    int
	emptyDiscarded int
	emitted        int

	metas []BlockScanMeta
}

func (s *blocksSeekScanner) handleLineSpan(ctx context.Context, span LineSpan) error {
	if !s.inside {
		return s.handleOutsideBlockLine(ctx, span)
	}

	return s.handleInsideBlockLine(ctx, span)
}

func (s *blocksSeekScanner) handleOutsideBlockLine(ctx context.Context, span LineSpan) error {
	startMatch, err := MatchLineSpan(ctx, s.rs, span, s.opts.StartDelimiter)
	if err != nil {
		return err
	}
	if !startMatch {
		return nil
	}

	s.startBlock(span)

	return nil
}

func (s *blocksSeekScanner) startBlock(span LineSpan) {
	s.inside = true
	s.startSpan = span
	s.innerCount = 0
	s.firstInnerSeen = false
	s.blockStartLineNum = span.LineNum
}

func (s *blocksSeekScanner) handleInsideBlockLine(ctx context.Context, span LineSpan) error {
	endMatch, err := MatchLineSpan(ctx, s.rs, span, s.opts.EndDelimiter)
	if err != nil {
		return err
	}

	if endMatch {
		return s.closeBlock(ctx, span)
	}

	s.recordInnerBlockLine(span)

	return nil
}

func (s *blocksSeekScanner) recordInnerBlockLine(span LineSpan) {
	s.innerCount++
	if !s.firstInnerSeen {
		s.firstInnerSpan = span
		s.firstInnerSeen = true
	}
	s.lastInnerSpan = span
}

//nolint:funlen // Seekable blocks close assembles output span, fingerprint, and strategy-specific naming.
func (s *blocksSeekScanner) closeBlock(ctx context.Context, endSpan LineSpan) error {
	s.blocksFound++
	blockFoundOrd := s.blocksFound

	startSp := s.startSpan
	innerCount := s.innerCount
	firstInner := s.firstInnerSpan
	lastInner := s.lastInnerSpan

	s.resetCurrentBlock()

	if innerCount == 0 {
		return s.discardEmptyBlock()
	}

	if err := enforceMaxSections(s.limits, s.emitted+1); err != nil {
		return err
	}

	s.emitted++
	emitOrd := s.emitted

	innerByteStart := firstInner.SerializedStart
	innerByteEndX := lastInner.SerializedEnd

	outStart, outEnd := s.blockOutputBounds(startSp, endSpan, innerByteStart, innerByteEndX)

	fp, err := SHA256SerializedByteSpan(ctx, s.rs, outStart, outEnd)
	if err != nil {
		return err
	}

	startText, innerTexts, err := s.blockNamingTexts(ctx, startSp, firstInner)
	if err != nil {
		return err
	}

	name := chooseBlockFilenameStrings(
		s.opts.Naming,
		emitOrd,
		startText,
		innerTexts,
		s.opts.Extension,
		s.existing,
	)

	meta := BlockScanMeta{
		Name:                   name,
		IncludeDelimiters:      s.opts.IncludeDelimiters,
		StartDelimLineNum:      s.blockStartLineNum,
		InnerStartLineNum:      firstInner.LineNum,
		InnerEndLineNum:        lastInner.LineNum,
		EndDelimLineNum:        endSpan.LineNum,
		InnerContentLineCount:  innerCount,
		EmittedOrdinal:         emitOrd,
		BlockFoundOrdinal:      blockFoundOrd,
		InnerByteStart:         innerByteStart,
		InnerByteEndExclusive:  innerByteEndX,
		OutputByteStart:        outStart,
		OutputByteEndExclusive: outEnd,
		SpanFingerprintSHA256:  fp,
	}

	s.metas = append(s.metas, meta)

	return nil
}

func (s *blocksSeekScanner) blockOutputBounds(
	startSp LineSpan,
	endSpan LineSpan,
	innerByteStart uint64,
	innerByteEndX uint64,
) (outputByteStart, outputByteEnd uint64) {
	if s.opts.IncludeDelimiters {
		return startSp.SerializedStart, endSpan.SerializedEnd
	}

	return innerByteStart, innerByteEndX
}

func (s *blocksSeekScanner) blockNamingTexts(
	ctx context.Context,
	startSp LineSpan,
	firstInner LineSpan,
) (delimiterText string, contentTexts []string, err error) {
	switch s.opts.Naming {
	case FromDelimiter:
		startText, err := lineSpanNamingContentUTF8(ctx, s.rs, startSp)
		return startText, nil, err
	case FromContent:
		innerText, err := lineSpanNamingContentUTF8(ctx, s.rs, firstInner)
		return "", []string{innerText}, err
	case Sequential:
		return "", nil, nil
	default:
		return "", nil, nil
	}
}

func (s *blocksSeekScanner) resetCurrentBlock() {
	s.inside = false
	s.innerCount = 0
	s.firstInnerSeen = false
}

func (s *blocksSeekScanner) discardEmptyBlock() error {
	s.emptyDiscarded++

	return nil
}
