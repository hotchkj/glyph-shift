package linparse

import (
	"fmt"
	"strconv"
	"strings"
)

const closedRangeParts = 2

func trimAndCheckEmpty(raw string) (trimmed string, err error) {
	trimmed = strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w", ErrEmptyLineRange)
	}

	return trimmed, nil
}

func parseOpenStartRange(segment string) (end int, ok bool, err error) {
	if !strings.HasPrefix(segment, "-") || strings.Contains(segment[1:], "-") {
		return 0, false, nil
	}

	endVal, perr := strconv.Atoi(strings.TrimPrefix(segment, "-"))
	if perr != nil {
		return 0, false, invalidLineRangeInteger(strings.TrimPrefix(segment, "-"))
	}

	return endVal, true, nil
}

func parseOpenEndRange(segment string) (start int, ok bool, err error) {
	if !strings.HasSuffix(segment, "-") {
		return 0, false, nil
	}

	startStr := strings.TrimSuffix(segment, "-")
	if startStr == "" {
		return 0, false, fmt.Errorf("%w", ErrInvalidLineRange)
	}

	startVal, perr := strconv.Atoi(startStr)
	if perr != nil {
		return 0, false, invalidLineRangeInteger(startStr)
	}

	return startVal, true, nil
}

func parseClosedLineRange(segment string) (start, end int, err error) {
	parts := strings.SplitN(segment, "-", closedRangeParts)
	if len(parts) != closedRangeParts {
		return 0, 0, fmt.Errorf("%w", ErrInvalidLineRange)
	}

	startVal, perr := strconv.Atoi(parts[0])
	if perr != nil {
		return 0, 0, invalidLineRangeInteger(parts[0])
	}

	endVal, perr := strconv.Atoi(parts[1])
	if perr != nil {
		return 0, 0, invalidLineRangeInteger(parts[1])
	}

	return startVal, endVal, nil
}

// ParseCLIRange parses a CLI --lines value such as "45-55", "95-", or "-10".
func ParseCLIRange(lineRange string) (start, end int, err error) {
	normalized, err := trimAndCheckEmpty(lineRange)
	if err != nil {
		return 0, 0, err
	}

	if endVal, ok, perr := parseOpenStartRange(normalized); perr != nil {
		return 0, 0, perr
	} else if ok {
		return 0, endVal, nil
	}

	if startVal, ok, perr := parseOpenEndRange(normalized); perr != nil {
		return 0, 0, perr
	} else if ok {
		return startVal, 0, nil
	}

	return parseClosedLineRange(normalized)
}
