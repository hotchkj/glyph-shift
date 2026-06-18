package fileops

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

func TestExtractBlocks_twoBlocksExcludeDelims(t *testing.T) {
	t.Parallel()

	src := "intro\n```gherkin\nFeature: A\n```\n\n```gherkin\nFeature: B\n```\n"
	start := regexp.MustCompile("^```gherkin")
	end := regexp.MustCompile("^```$")

	res, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         Sequential,
		Extension:      ".txt",
	})
	if err != nil {
		t.Fatalf("ExtractBlocks: %v", err)
	}

	if res.BlocksFound != 2 {
		t.Fatalf("BlocksFound: want 2 got %d", res.BlocksFound)
	}

	if len(res.Blocks) != 2 {
		t.Fatalf("blocks: want 2 got %d", len(res.Blocks))
	}

	if len(res.Warnings) != 0 {
		t.Fatalf("warnings: %v", res.Warnings)
	}

	first := string(res.Blocks[0].Lines[0].Content)
	if first != "Feature: A" {
		t.Fatalf("first block first line: want Feature: A got %q", first)
	}
}

func TestExtractBlocks_includeDelimiters(t *testing.T) {
	t.Parallel()

	src := "```go\nx\n```\n"
	start := regexp.MustCompile("^```go")
	end := regexp.MustCompile("^```$")

	res, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:            strings.NewReader(src),
		StartDelimiter:    start,
		EndDelimiter:      end,
		Naming:            Sequential,
		IncludeDelimiters: true,
		Extension:         ".txt",
	})
	if err != nil {
		t.Fatalf("ExtractBlocks: %v", err)
	}

	if res.BlocksFound != 1 {
		t.Fatalf("BlocksFound: want 1 got %d", res.BlocksFound)
	}

	if len(res.Blocks) != 1 {
		t.Fatalf("blocks: want 1 got %d", len(res.Blocks))
	}

	got, werr := writeLinesToBuf(res.Blocks[0].Lines)
	if werr != nil {
		t.Fatalf("write: %v", werr)
	}

	if !bytes.Equal(got, []byte("```go\nx\n```\n")) {
		t.Fatalf("want ```go\\nx\\n```\\n got %q", got)
	}
}

func TestExtractBlocks_emptyBlockNoOutput(t *testing.T) {
	t.Parallel()

	src := "```gherkin\n```\n"
	start := regexp.MustCompile("^```gherkin")
	end := regexp.MustCompile("^```$")

	res, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         Sequential,
		Extension:      ".txt",
	})
	if err != nil {
		t.Fatalf("ExtractBlocks: %v", err)
	}

	if res.BlocksFound != 1 {
		t.Fatalf("BlocksFound: want 1 got %d", res.BlocksFound)
	}

	if len(res.Blocks) != 0 {
		t.Fatalf("blocks: want 0 got %d", len(res.Blocks))
	}
}

func TestExtractBlocks_unclosedBlockError(t *testing.T) {
	t.Parallel()

	src := "```gherkin\norphan\n"
	start := regexp.MustCompile("^```gherkin")
	end := regexp.MustCompile("^```$")

	res, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         Sequential,
		Extension:      ".txt",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrUnclosedBlock) {
		t.Fatalf("want ErrUnclosedBlock got %v", err)
	}

	if res.BlocksFound != 0 {
		t.Fatalf("BlocksFound: want 0 got %d", res.BlocksFound)
	}

	if len(res.Blocks) != 0 {
		t.Fatalf("blocks: want 0 got %d", len(res.Blocks))
	}
}

func TestExtractBlocks_noBlocksFoundError(t *testing.T) {
	t.Parallel()

	src := "plain\ntext\n"
	start := regexp.MustCompile("^```gherkin")
	end := regexp.MustCompile("^```$")

	res, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         Sequential,
		Extension:      ".txt",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, ErrNoBlocksFound) {
		t.Fatalf("want ErrNoBlocksFound got %v", err)
	}

	if res.BlocksFound != 0 {
		t.Fatalf("BlocksFound: want 0 got %d", res.BlocksFound)
	}

	if len(res.Blocks) != 0 {
		t.Fatalf("blocks: want 0 got %d", len(res.Blocks))
	}
}

func TestExtractBlocks_nilStartError(t *testing.T) {
	t.Parallel()

	end := regexp.MustCompile("^$")

	_, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader("a\n"),
		StartDelimiter: nil,
		EndDelimiter:   end,
	})
	if !errors.Is(err, errBlocksNilStart) {
		t.Fatalf("want errBlocksNilStart, got %v", err)
	}
}

func TestExtractBlocks_contentNaming(t *testing.T) {
	t.Parallel()

	src := "```gherkin\nFeature: My Story\n```\n"
	start := regexp.MustCompile("^```gherkin")
	end := regexp.MustCompile("^```$")

	res, err := ExtractBlocks(context.Background(), BlocksOptions{
		Source:         strings.NewReader(src),
		StartDelimiter: start,
		EndDelimiter:   end,
		Naming:         FromContent,
		Extension:      ".feature",
	})
	if err != nil {
		t.Fatalf("ExtractBlocks: %v", err)
	}

	if len(res.Blocks) != 1 {
		t.Fatalf("blocks: want 1 got %d", len(res.Blocks))
	}

	if res.Blocks[0].Name != "feature-my-story.feature" {
		t.Fatalf("name: got %q", res.Blocks[0].Name)
	}
}

// writeLinesToBuf serializes lines for assertions.
func writeLinesToBuf(lines []Line) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteLinesTo(&buf, lines); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
