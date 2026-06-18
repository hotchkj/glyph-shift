package fileops

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

const splitTestSeq001 = "001.txt"

const featureLoginName = "feature-login.txt"

var errSplitReaderFailed = errors.New("split reader failed")

type failingSplitReader struct{}

func (failingSplitReader) Read([]byte) (int, error) {
	return 0, errSplitReaderFailed
}

func TestSplit_PreambleAndSequential(t *testing.T) {
	t.Parallel()

	src := "Preamble text\n## Feature: Login\nL1\n## Feature: Signup\nL2\n## Feature: Profile\nL3\n"
	re := regexp.MustCompile(`^##\sFeature:`)

	res, err := Split(context.Background(), SplitOptions{
		Source:    strings.NewReader(src),
		Delimiter: re,
		Naming:    Sequential,
		Extension: ".txt",
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(res.Sections) != 4 {
		t.Fatalf("sections: want 4 got %d", len(res.Sections))
	}

	if res.Sections[0].Name != splitTestSeq001 {
		t.Fatalf("preamble name: %q", res.Sections[0].Name)
	}

	const (
		seq002 = "002.txt"
		seq003 = "003.txt"
		seq004 = "004.txt"
	)

	if res.Sections[1].Name != seq002 || res.Sections[2].Name != seq003 || res.Sections[3].Name != seq004 {
		t.Fatalf("sequential names: %#v", res.Sections)
	}
}

func TestSplitReadAllError(t *testing.T) {
	t.Parallel()

	_, err := Split(context.Background(), SplitOptions{
		Source:    failingSplitReader{},
		Delimiter: regexp.MustCompile(`^---$`),
	})
	if !errors.Is(err, errSplitReaderFailed) {
		t.Fatalf("Split read error = %v want %v", err, errSplitReaderFailed)
	}
}

func TestSplit_EmptyPreambleSkipsFile(t *testing.T) {
	t.Parallel()

	src := "## Section One\nC1\n## Section Two\nC2\n"
	re := regexp.MustCompile(`^##\s`)

	res, err := Split(context.Background(), SplitOptions{
		Source:    strings.NewReader(src),
		Delimiter: re,
		Naming:    Sequential,
		Extension: ".txt",
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(res.Sections) != 2 {
		t.Fatalf("sections: want 2 got %d", len(res.Sections))
	}

	if res.Sections[0].Name != splitTestSeq001 {
		t.Fatalf("first section name: %q", res.Sections[0].Name)
	}
}

func TestSplit_NoDelimiterMatchReturnsError(t *testing.T) {
	t.Parallel()

	_, err := Split(context.Background(), SplitOptions{
		Source:    strings.NewReader("plain text\n"),
		Delimiter: regexp.MustCompile(`^---$`),
		Naming:    Sequential,
		Extension: ".txt",
	})
	if err == nil {
		t.Fatal("want error when delimiter matches no source lines")
	}

	if !errors.Is(err, ErrNoDelimiterMatch) {
		t.Fatalf("expected ErrNoDelimiterMatch, got %v", err)
	}
}

func TestSplit_StripDelimiter(t *testing.T) {
	t.Parallel()

	src := "## A\nBody\n"
	re := regexp.MustCompile(`^##\s`)

	res, err := Split(context.Background(), SplitOptions{
		Source:         strings.NewReader(src),
		Delimiter:      re,
		Naming:         Sequential,
		Extension:      ".txt",
		StripDelimiter: true,
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(res.Sections) != 1 {
		t.Fatalf("sections: want 1 got %d", len(res.Sections))
	}

	if len(res.Sections[0].Lines) != 1 {
		t.Fatalf("lines after strip: want 1 got %d", len(res.Sections[0].Lines))
	}

	if string(res.Sections[0].Lines[0].Content) != "Body" {
		t.Fatalf("content: %q", res.Sections[0].Lines[0].Content)
	}
}

func TestSplit_StripDelimiter_SkipsEmptySections(t *testing.T) {
	t.Parallel()
	input := "---\nfirst\n---\n---\nlast\n"
	opts := SplitOptions{
		Source:         strings.NewReader(input),
		Delimiter:      regexp.MustCompile(`^---$`),
		Naming:         Sequential,
		StripDelimiter: true,
		Extension:      ".txt",
	}
	res, err := Split(context.Background(), opts)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// With strip-delimiter, the section between the two consecutive "---" delimiters
	// has no content lines and should be omitted entirely.
	for _, sec := range res.Sections {
		if len(sec.Lines) == 0 {
			t.Fatalf("section %q has zero lines; empty sections should be skipped", sec.Name)
		}
	}
}

func TestSplit_FromContentSuffix(t *testing.T) {
	t.Parallel()

	src := "## Feature: User Login\nLogin steps\n"
	re := regexp.MustCompile(`^##\sFeature:`)

	res, err := Split(context.Background(), SplitOptions{
		Source:    strings.NewReader(src),
		Delimiter: re,
		Naming:    FromContent,
		Extension: ".feature",
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(res.Sections) != 1 {
		t.Fatalf("sections: want 1 got %d", len(res.Sections))
	}

	const wantName = "user-login.feature"
	if res.Sections[0].Name != wantName {
		t.Fatalf("name: got %q want %q", res.Sections[0].Name, wantName)
	}
}

func TestSplit_DeduplicateCollision(t *testing.T) {
	t.Parallel()

	src := "---\nFeature: Login\na\n---\nFeature: Login\nb\n"
	re := regexp.MustCompile(`^---`)

	res, err := Split(context.Background(), SplitOptions{
		Source:    strings.NewReader(src),
		Delimiter: re,
		Naming:    FromContent,
		Extension: ".txt",
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	if len(res.Sections) != 2 {
		t.Fatalf("sections: want 2 got %d", len(res.Sections))
	}

	if res.Sections[0].Name != featureLoginName {
		t.Fatalf("first section name: got %q, want %q", res.Sections[0].Name, featureLoginName)
	}

	if res.Sections[0].Name == res.Sections[1].Name {
		t.Fatal("expected distinct names")
	}
}

func TestSplit_PreservesLineBytes(t *testing.T) {
	t.Parallel()

	src := "## A\r\ntext\r\n"
	re := regexp.MustCompile(`^##`)

	res, err := Split(context.Background(), SplitOptions{
		Source:    bytes.NewReader([]byte(src)),
		Delimiter: re,
		Naming:    Sequential,
		Extension: ".txt",
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, res.Sections[0].Lines); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf.Bytes(), []byte("## A\r\ntext\r\n")) {
		t.Fatalf("got %q", buf.Bytes())
	}
}

func TestSplit_NilDelimiterReturnsError(t *testing.T) {
	t.Parallel()

	_, err := Split(context.Background(), SplitOptions{
		Source:    strings.NewReader("x\n"),
		Delimiter: nil,
		Naming:    Sequential,
		Extension: ".txt",
	})
	if !errors.Is(err, errSplitNilDelimiter) {
		t.Fatalf("want errSplitNilDelimiter, got %v", err)
	}
}

var errSplitTestReadAllBlocked = errors.New("split non-seekable read failed")

type splitReadAlwaysErr struct{}

func (splitReadAlwaysErr) Read([]byte) (int, error) {
	return 0, errSplitTestReadAllBlocked
}

func TestSplit_NonSeekableReadSourceFails(t *testing.T) {
	t.Parallel()

	_, err := Split(context.Background(), SplitOptions{
		Source:    splitReadAlwaysErr{},
		Delimiter: regexp.MustCompile(`^##`),
		Naming:    Sequential,
		Extension: ".txt",
	})
	if !errors.Is(err, errSplitTestReadAllBlocked) {
		t.Fatalf("want read error, got %v", err)
	}
}

func TestSplit_ContextCanceledBeforeRun(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Split(ctx, SplitOptions{
		Source:    strings.NewReader("x\n"),
		Delimiter: regexp.MustCompile(`^##`),
		Naming:    Sequential,
		Extension: ".txt",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
