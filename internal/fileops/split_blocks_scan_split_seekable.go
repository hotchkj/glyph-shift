package fileops

import (
	"context"
	"fmt"
	"io"
	"regexp"
)

type splitSeekPending struct {
	delimiterSpan    LineSpan
	delimMatchEndRel int // exclusive UTF-8 offset within [ContentStart,ContentEnd); from FindLineSpanSubmatchIndex

	firstInnerSpan  *LineSpan
	secondInnerSpan *LineSpan
}

type splitSeekScanner struct {
	opts   SplitOptions
	limits BoundedScanLimits
	rs     io.ReadSeeker

	meta    []SplitSectionMeta
	outSecs int
	delHits int

	existing map[string]bool

	sawDelimiter bool

	pending *splitSeekPending

	preambleDone     bool
	nextDelimEmitSeq int

	lastEmittedLineNum int
	streamTailByte     uint64
}

func (s *splitSeekScanner) pushSection(sectionMeta *SplitSectionMeta) error {
	s.meta = append(s.meta, *sectionMeta)
	s.outSecs++

	return enforceMaxSections(s.limits, s.outSecs)
}

func (s *splitSeekScanner) flushPreamble(
	ctx context.Context,
	firstDelimLine int,
	byteBeforeFirstDelimLine uint64,
) error {
	if s.preambleDone {
		return nil
	}

	s.preambleDone = true

	if firstDelimLine <= 1 {
		return nil
	}

	lastPreamble := firstDelimLine - 1

	seq := s.nextDelimEmitSeq

	name, nerr := preambleSplitOutputFilename(ctx, s.rs, s.opts, seq, s.existing)
	if nerr != nil {
		return nerr
	}

	s.nextDelimEmitSeq++

	fp, ferr := SHA256SerializedByteSpan(ctx, s.rs, 0, byteBeforeFirstDelimLine)
	if ferr != nil {
		return ferr
	}

	return s.pushSection(&SplitSectionMeta{
		Name:                   name,
		OutputStartLineNum:     1,
		OutputEndLineNum:       lastPreamble,
		ContentLineCount:       lastPreamble,
		ByteSpanStart:          0,
		ByteSpanEndExclusive:   byteBeforeFirstDelimLine,
		OrdinalSeq:             seq,
		IsPreambleSection:      true,
		StripDelimiterInOutput: s.opts.StripDelimiter,
		DelimiterLineSourceNum: 0,
		SpanFingerprintSHA256:  fp,
	})
}

func (s *splitSeekScanner) finalizePendingBeforeNextDelim(
	ctx context.Context,
	nextDelimLine int,
	byteBeforeNextDelim uint64,
) error {
	pend := s.pending
	if pend == nil {
		return nil
	}

	s.pending = nil

	geometry, err := splitPendingOutputGeometry(
		pend, s.opts.StripDelimiter, nextDelimLine, byteBeforeNextDelim,
	)
	if err != nil {
		return err
	}
	if geometry.skip {
		return nil
	}

	delimText, outThin, fullThin, err := s.splitPendingNamingStrings(ctx, pend, nextDelimLine)
	if err != nil {
		return err
	}

	fp, err := SHA256SerializedByteSpan(ctx, s.rs, geometry.byteFrom, geometry.byteTo)
	if err != nil {
		return err
	}

	seq := s.nextDelimEmitSeq
	s.nextDelimEmitSeq++

	name, err := s.splitPendingOutputName(ctx, pend, seq, delimText, outThin, fullThin)
	if err != nil {
		return err
	}

	return s.pushSection(&SplitSectionMeta{
		Name:                   name,
		OutputStartLineNum:     geometry.outputStartLine,
		OutputEndLineNum:       geometry.outputEndLine,
		ContentLineCount:       geometry.contentLines,
		ByteSpanStart:          geometry.byteFrom,
		ByteSpanEndExclusive:   geometry.byteTo,
		OrdinalSeq:             seq,
		IsPreambleSection:      false,
		StripDelimiterInOutput: s.opts.StripDelimiter,
		DelimiterLineSourceNum: pend.delimiterSpan.LineNum,
		SpanFingerprintSHA256:  fp,
	})
}

type splitOutputGeometry struct {
	outputStartLine int
	outputEndLine   int
	contentLines    int
	byteFrom        uint64
	byteTo          uint64
	skip            bool
}

func splitPendingOutputGeometry(
	pend *splitSeekPending,
	stripDelimiter bool,
	nextDelimLine int,
	byteBeforeNextDelim uint64,
) (splitOutputGeometry, error) {
	dStart := pend.delimiterSpan.LineNum
	secLen := nextDelimLine - dStart
	if secLen < 1 {
		return splitOutputGeometry{}, fmt.Errorf(splitErrorFormat, errSplitScanEmptySegment)
	}

	delimLineByteStart := pend.delimiterSpan.SerializedStart
	delimLineByteLen := pend.delimiterSpan.SerializedEnd - pend.delimiterSpan.SerializedStart

	outStart, outEnd, contentLines, byteFrom, byteTo, skip := computeSplitSegOutput(
		stripDelimiter,
		dStart,
		nextDelimLine,
		secLen,
		delimLineByteStart,
		delimLineByteLen,
		byteBeforeNextDelim,
	)

	return splitOutputGeometry{
		outputStartLine: outStart,
		outputEndLine:   outEnd,
		contentLines:    contentLines,
		byteFrom:        byteFrom,
		byteTo:          byteTo,
		skip:            skip,
	}, nil
}

func (s *splitSeekScanner) splitPendingNamingStrings(
	ctx context.Context,
	pend *splitSeekPending,
	nextDelimLine int,
) (delimText string, outThin, fullThin []string, err error) {
	var firstS, secondS *string

	if pend.firstInnerSpan != nil {
		v, readErr := lineSpanNamingContentUTF8(ctx, s.rs, *pend.firstInnerSpan)
		if readErr != nil {
			return "", nil, nil, readErr
		}

		firstS = &v
	}

	if pend.secondInnerSpan != nil {
		v, readErr := lineSpanNamingContentUTF8(ctx, s.rs, *pend.secondInnerSpan)
		if readErr != nil {
			return "", nil, nil, readErr
		}

		secondS = &v
	}

	delimText, err = lineSpanNamingContentUTF8(ctx, s.rs, pend.delimiterSpan)
	if err != nil {
		return "", nil, nil, err
	}

	secLen := nextDelimLine - pend.delimiterSpan.LineNum
	outThin, fullThin = thinStringsForSplitNaming(s.opts.StripDelimiter, delimText, secLen, firstS, secondS)

	return delimText, outThin, fullThin, nil
}

func (s *splitSeekScanner) splitPendingOutputName(
	ctx context.Context,
	pend *splitSeekPending,
	seq int,
	delimText string,
	outThin, fullThin []string,
) (string, error) {
	switch s.opts.Naming {
	case FromContent:
		text, nerr := textForSeekableFromContentNaming(&seekableFromContentNamingInput{
			ctx:             ctx,
			src:             s.rs,
			sp:              pend.delimiterSpan,
			delimLinePrefix: delimText,
			matchEndRel:     pend.delimMatchEndRel,
			outThin:         outThin,
			fullThin:        fullThin,
			strip:           s.opts.StripDelimiter,
		})
		if nerr != nil {
			return "", nerr
		}

		return DeduplicateFilename(GenerateFilename(FromContent, seq, text, s.opts.Extension), s.existing), nil
	case Sequential, FromDelimiter:
		return chooseSectionFilenameFromStrings(
			s.opts,
			seq,
			delimText,
			outThin,
			fullThin,
			s.opts.Extension,
			s.existing,
		), nil
	}

	return "", nil
}

func (s *splitSeekScanner) observeDelimiterLine(ctx context.Context, span LineSpan, delimMatchEndRel int) error {
	s.delHits++

	lineStart := span.SerializedStart

	if !s.sawDelimiter {
		s.sawDelimiter = true

		if err := s.flushPreamble(ctx, span.LineNum, lineStart); err != nil {
			return err
		}

		if err := enforceMaxSections(s.limits, s.outSecs+1); err != nil {
			return err
		}

		cp := span
		s.pending = &splitSeekPending{delimiterSpan: cp, delimMatchEndRel: delimMatchEndRel}

		return nil
	}

	if err := s.finalizePendingBeforeNextDelim(ctx, span.LineNum, lineStart); err != nil {
		return err
	}

	cp := span
	s.pending = &splitSeekPending{delimiterSpan: cp, delimMatchEndRel: delimMatchEndRel}

	if err := enforceMaxSections(s.limits, s.outSecs+1); err != nil {
		return err
	}

	return nil
}

func (s *splitSeekScanner) observeNonDelimiterLine(span LineSpan) {
	if s.pending == nil {
		return
	}

	switch {
	case s.pending.firstInnerSpan == nil:
		cp := span
		s.pending.firstInnerSpan = &cp
	case s.pending.secondInnerSpan == nil:
		cp := span
		s.pending.secondInnerSpan = &cp
	default:
	}
}

func scanSplitSectionsMetaSeekable(
	ctx context.Context,
	rs io.ReadSeeker,
	opts SplitOptions,
	limits BoundedScanLimits,
	lineScanOpts linespanScanOptions,
) (SplitBoundedScanResult, error) {
	dopts := opts
	dopts.Source = rs

	sc := splitSeekScanner{
		opts:             dopts,
		limits:           limits,
		rs:               rs,
		existing:         make(map[string]bool),
		nextDelimEmitSeq: 1,
	}

	err := scanLineSpansWithOptions(ctx, rs, sc.observeSplitLineSpan(ctx, opts.Delimiter), lineScanOpts)
	if err != nil {
		return SplitBoundedScanResult{}, fmt.Errorf(splitErrorFormat, err)
	}

	if !sc.sawDelimiter {
		return SplitBoundedScanResult{}, fmt.Errorf(splitErrorFormat, ErrNoDelimiterMatch)
	}

	afterLast := sc.lastEmittedLineNum + 1

	if ferr := sc.finalizePendingBeforeNextDelim(ctx, afterLast, sc.streamTailByte); ferr != nil {
		return SplitBoundedScanResult{}, fmt.Errorf(splitErrorFormat, ferr)
	}

	return SplitBoundedScanResult{
		Sections:           sc.meta,
		OutputSectionCount: sc.outSecs,
		DelimiterLineCount: sc.delHits,
	}, nil
}

func (s *splitSeekScanner) observeSplitLineSpan(
	ctx context.Context,
	delimiter *regexp.Regexp,
) func(LineSpan) error {
	return func(span LineSpan) error {
		return isolateMatchLineSpanOnSharedSeeker(s.rs, span, func() error {
			return s.observeSplitLineSpanOnSeeker(ctx, delimiter, span)
		})
	}
}

func (s *splitSeekScanner) observeSplitLineSpanOnSeeker(
	ctx context.Context,
	delimiter *regexp.Regexp,
	span LineSpan,
) error {
	s.lastEmittedLineNum = span.LineNum
	s.streamTailByte = span.SerializedEnd

	idx, err := FindLineSpanSubmatchIndex(ctx, s.rs, span, delimiter)
	if err != nil {
		return err
	}

	matched := len(idx) >= 2 && idx[0] >= 0 && idx[1] >= 0
	if matched {
		return s.observeDelimiterLine(ctx, span, idx[1])
	}

	s.observeNonDelimiterLine(span)

	return nil
}
