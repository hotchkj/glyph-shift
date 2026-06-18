package testutil

// BuildMaxFilesExceededSplitPrefix constructs a newline-delimited source where delimiter matches
// occur only after binaryCheckReadWindow bytes and where the delimiter count yields more sections
// than maxAllow. Prefix is standalone: the forbidden tail begins immediately after Prefix when
// using TailGuardSourceOpener.
//
// delim must match full delimiter lines (for example regexp ^---$ corresponds to "---" plus LF).
func BuildMaxFilesExceededSplitPrefix(padByte byte, delimLine string, maxAllow int) []byte {
	// preamble padding: filler lines strictly between binary window and delimiter section.
	if delimLine == "" {
		panic("BuildMaxFilesExceededSplitPrefix: empty delimLine")
	}

	if maxAllow < 1 {
		panic("BuildMaxFilesExceededSplitPrefix: maxAllow must be positive")
	}

	padUnit := []byte{padByte, '\n'}
	repeat := BoundednessBinaryCheckReadWindow/len(padUnit) + 1
	head := bytesRepeatLine(padUnit, repeat)

	// delimLine must include newline if pattern expects logical line endings.
	secCount := maxAllow + 1
	sec := delimBytesSection(delimLine, secCount)

	out := append(append(make([]byte, 0, len(head)+len(sec)), head...), sec...)

	return out
}

// BuildMaxFilesExceededBlocksPrefix constructs a newline-delimited source where pad keeps the
// binary window safe, followed by blocksCount complete non-empty blocks.
//
//nolint:gocyclo // Delimiter-aligned block scaffolding mirrors extract fixtures with newline hygiene.
func BuildMaxFilesExceededBlocksPrefix(
	padByte byte,
	headerLine string,
	beginLine string,
	bodyLine string,
	endLine string,
	blocksCount int,
) []byte {
	if blocksCount < minBlocksCountForMaxFilesExceededFixture {
		panic("BuildMaxFilesExceededBlocksPrefix: blocksCount must be >= 2")
	}

	padUnit := []byte{padByte, '\n'}
	repeat := BoundednessBinaryCheckReadWindow/len(padUnit) + 1
	head := bytesRepeatLine(padUnit, repeat)

	var sb []byte
	sb = append(sb, head...)
	if headerLine != "" {
		sb = appendLineString(sb, headerLine)
	}

	for range blocksCount {
		sb = appendBlockFixture(sb, beginLine, bodyLine, endLine)
	}

	return sb
}

func appendBlockFixture(dst []byte, beginLine, bodyLine, endLine string) []byte {
	dst = appendLineString(dst, beginLine)
	dst = appendLineString(dst, bodyLine)

	return appendLineString(dst, endLine)
}

func appendLineString(dst []byte, line string) []byte {
	dst = append(dst, []byte(line)...)
	if line == "" || line[len(line)-1] != '\n' {
		dst = append(dst, '\n')
	}

	return dst
}

func bytesRepeatLine(unit []byte, repeat int) []byte {
	buf := make([]byte, 0, len(unit)*repeat)
	for range repeat {
		buf = append(buf, unit...)
	}

	return buf
}

func delimBytesSection(delimLine string, sectionCount int) []byte {
	// Alternate delimiter and short body lines: --- , x , --- , ...
	var sectionBytes []byte
	for secIdx := range sectionCount {
		sectionBytes = append(sectionBytes, []byte(delimLine)...)
		if delimLine[len(delimLine)-1] != '\n' {
			sectionBytes = append(sectionBytes, '\n')
		}

		if secIdx != sectionCount-1 {
			sectionBytes = append(sectionBytes, 'x', '\n')
		} else {
			sectionBytes = append(sectionBytes, 'z', '\n')
		}
	}

	return sectionBytes
}

// BuildLargeSplitSingleSectionSource returns a source with one delimiter-terminated section that
// contains many lines of deterministic payload (streaming-friendly shape).
//
// delimLinePrefix is the delimiter line matching params.Delimiter (for example "---\n" for "^---$").
func BuildLargeSplitSingleSectionSource(lineCount, lineLen int, delimLinePrefix []byte) []byte {
	core := repeatingLinePayload(lineLen, byte('_'))
	var out []byte
	out = append(out, delimLinePrefix...)
	for range lineCount {
		out = append(out, core...)
		out = append(out, '\n')
	}

	return out
}

// BuildLargeBlocksSingleBodySource returns a source containing one balanced non-empty block.
func BuildLargeBlocksSingleBodySource(
	headerLine []byte,
	beginLine []byte,
	endLine []byte,
	lineCount int,
	lineLen int,
) []byte {
	core := repeatingLinePayload(lineLen, byte('='))
	out := append(append([]byte(nil), headerLine...), beginLine...)
	for range lineCount {
		out = append(out, core...)
		out = append(out, '\n')
	}

	out = append(out, endLine...)

	return out
}

func repeatingLinePayload(lineLen int, filler byte) []byte {
	if lineLen <= 0 {
		return nil
	}

	b := bytesRepeatLine([]byte{filler}, lineLen)

	return append([]byte(nil), b...)
}
