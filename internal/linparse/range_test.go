package linparse_test

import (
	"errors"
	"testing"

	"github.com/hotchkj/glyph-shift/internal/linparse"
)

type parseCLIRangeCase struct {
	name      string
	input     string
	wantStart int
	wantEnd   int
	errIs     error
	anyErr    bool
}

func mustErrIs(t *testing.T, err, want error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error wrapping %v, got nil", want)
	}
	if !errors.Is(err, want) {
		t.Fatalf("expected error %v, got %v", want, err)
	}
}

func mustAnyErr(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func mustParseOK(t *testing.T, start, end, wantStart, wantEnd int, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start != wantStart {
		t.Errorf("start: got %d, want %d", start, wantStart)
	}
	if end != wantEnd {
		t.Errorf("end: got %d, want %d", end, wantEnd)
	}
}

func TestParseCLIRange(t *testing.T) {
	t.Parallel()
	tests := []parseCLIRangeCase{
		{name: "closed range", input: "45-55", wantStart: 45, wantEnd: 55},
		{name: "open end", input: "95-", wantStart: 95, wantEnd: 0},
		{name: "open start", input: "-10", wantStart: 0, wantEnd: 10},
		{name: "whitespace trimmed", input: "  10-20  ", wantStart: 10, wantEnd: 20},
		{name: "single line", input: "5-5", wantStart: 5, wantEnd: 5},
		{name: "large numbers", input: "999999-1000000", wantStart: 999999, wantEnd: 1000000},
		{name: "empty string", input: "", errIs: linparse.ErrEmptyLineRange},
		{name: "whitespace only", input: "   ", errIs: linparse.ErrEmptyLineRange},
		{name: "non-numeric", input: "abc", anyErr: true},
		{name: "non-numeric end", input: "5-abc", anyErr: true},
		{name: "non-numeric start open", input: "abc-", anyErr: true},
		{name: "ambiguous negative style", input: "-5-10", anyErr: true},
		{name: "single number no dash", input: "42", errIs: linparse.ErrInvalidLineRange},
		{name: "dash only", input: "-", anyErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end, err := linparse.ParseCLIRange(tt.input)
			if tt.errIs != nil {
				mustErrIs(t, err, tt.errIs)
				return
			}
			if tt.anyErr {
				mustAnyErr(t, err)
				return
			}
			mustParseOK(t, start, end, tt.wantStart, tt.wantEnd, err)
		})
	}
}
