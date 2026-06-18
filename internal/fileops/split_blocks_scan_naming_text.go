package fileops

import (
	"context"
	"errors"
	"io"
	"regexp"
	"strings"
)

var (
	errDelimiterMatchEndMissingForFromContent = errors.New(
		"fileops: delimiter match end offset missing for FromContent naming",
	)
	errNilSeekableFromContentNamingInput = errors.New("fileops: nil seekableFromContentNamingInput")
)

// seekableFromContentNamingInput packages arguments for textForSeekableFromContentNaming.
type seekableFromContentNamingInput struct {
	ctx             context.Context
	src             io.ReadSeeker
	sp              LineSpan
	delimLinePrefix string
	matchEndRel     int
	outThin         []string
	fullThin        []string
	strip           bool
}

// textForSeekableFromContentNaming derives FromContent filename input for seekable split scan using the
// delimiter regex match end offset from FindLineSpanSubmatchIndex on the full CONTENT span, then bounded
// UTF-8 reads from that byte position - not FindStringIndex on the capped delimiter-line prefix string.
func textForSeekableFromContentNaming(in *seekableFromContentNamingInput) (string, error) {
	if in == nil {
		return "", errNilSeekableFromContentNamingInput
	}

	if in.strip {
		return strippedSeekableFromContentText(in), nil
	}

	if in.matchEndRel < 0 {
		return "", errDelimiterMatchEndMissingForFromContent
	}

	suff, err := seekableFromContentSuffix(in)
	if err != nil {
		return "", err
	}

	suff = strings.TrimSpace(suff)
	if suff != "" {
		return suff, nil
	}

	if len(in.fullThin) > 1 {
		return in.fullThin[1], nil
	}

	return in.delimLinePrefix, nil
}

func strippedSeekableFromContentText(in *seekableFromContentNamingInput) string {
	if len(in.outThin) > 0 {
		return in.outThin[0]
	}

	return in.delimLinePrefix
}

func seekableFromContentSuffix(in *seekableFromContentNamingInput) (string, error) {
	contentLen := in.sp.ContentEnd - in.sp.ContentStart
	matchEndRel := in.matchEndRel
	if matchEndRel > int(contentLen) { //nolint:gosec // G115: UTF-8 match indices are bounded by span length cast
		matchEndRel = int(contentLen) //nolint:gosec // G115: clamp uses same bound as comparison
	}

	//nolint:gosec // G115: matchEndRel is non-negative and clamped to contentLen above.
	suffixStart := in.sp.ContentStart + uint64(matchEndRel)

	return readSerializedSpanContentPrefixUTF8(
		in.ctx,
		in.src,
		suffixStart,
		in.sp.ContentEnd,
		NamingMaterializationMaxBytes,
	)
}

func textForFromContentStrings(re *regexp.Regexp, delimLineText string, outLines, fullSec []string, strip bool) string {
	lineStr := delimLineText

	if strip {
		if len(outLines) > 0 {
			return outLines[0]
		}

		return lineStr
	}

	loc := re.FindStringIndex(lineStr)
	if len(loc) >= 2 && loc[1] <= len(lineStr) {
		suffix := strings.TrimSpace(lineStr[loc[1]:])
		if suffix != "" {
			return suffix
		}
	}

	if len(fullSec) > 1 {
		return fullSec[1]
	}

	return lineStr
}

func thinStringsForSplitNaming(
	strip bool,
	delimText string,
	secLen int,
	firstInner, secondInner *string,
) (outThin, fullThin []string) {
	fullThin = []string{delimText}

	if firstInner != nil {
		fullThin = append(fullThin, *firstInner)
	}

	if secondInner != nil {
		fullThin = append(fullThin, *secondInner)
	}

	if strip {
		return strippedThinStrings(secLen, firstInner, secondInner), fullThin
	}

	return fullThin, fullThin
}

func strippedThinStrings(secLen int, firstInner, secondInner *string) []string {
	if secLen <= 1 || firstInner == nil {
		return nil
	}

	outThin := []string{*firstInner}
	if secLen > 2 && secondInner != nil {
		outThin = append(outThin, *secondInner)
	}

	return outThin
}

func chooseSequentialSectionFilename(seq int, ext string, existing map[string]bool) string {
	base := GenerateFilename(Sequential, seq, "", ext)

	return DeduplicateFilename(base, existing)
}

func chooseSectionFilenameFromStrings(
	opts SplitOptions,
	seq int,
	delimLineText string,
	outLines []string,
	fullSec []string,
	ext string,
	existing map[string]bool,
) string {
	re := opts.Delimiter

	switch opts.Naming {
	case FromDelimiter:
		base := GenerateFilename(FromDelimiter, seq, delimLineText, ext)

		return DeduplicateFilename(base, existing)
	case FromContent:
		text := textForFromContentStrings(re, delimLineText, outLines, fullSec, opts.StripDelimiter)
		base := GenerateFilename(FromContent, seq, text, ext)

		return DeduplicateFilename(base, existing)
	case Sequential:
		return chooseSequentialSectionFilename(seq, ext, existing)
	default:
		return chooseSequentialSectionFilename(seq, ext, existing)
	}
}

func chooseBlockFilenameStrings(
	strategy NamingStrategy,
	seq int,
	startLineText string,
	innerTexts []string,
	ext string,
	existing map[string]bool,
) string {
	filenameStrategy := Sequential
	text := ""

	switch strategy {
	case Sequential:
	case FromDelimiter:
		filenameStrategy = FromDelimiter
		text = startLineText
	case FromContent:
		filenameStrategy = FromContent
		text = innerTexts[0]
	}

	base := GenerateFilename(filenameStrategy, seq, text, ext)

	return DeduplicateFilename(base, existing)
}
