package fileops

import (
	"context"
	"crypto/sha256"
	"io"
	"regexp"
)

// TestingLockedReplaySource returns the modifier's locked handle as a replay source for fileops_test
// black-box tests that must simulate CommitFromWriter callback behavior while the lock is held.
func TestingLockedReplaySource(modifier *Modifier) interface {
	io.Reader
	io.Seeker
	io.WriterAt
} {
	return modifier.file
}

// TestingSetModifierHash records the expected locked-source digest for CommitFromWriter tests.
func TestingSetModifierHash(modifier *Modifier, hash [sha256.Size]byte) {
	modifier.hash = hash
}

type TestingCommitTempFile interface {
	io.Writer
	Sync() error
	Close() error
}

func TestingWriteSyncCloseCommitTemp(tmp TestingCommitTempFile, newContent []byte) (bool, error) {
	return writeSyncCloseCommitTemp(tmp, newContent)
}

// TestingTrimIncompleteUTF8Suffix exposes naming UTF-8 trim behavior for black-box mutation coverage.
func TestingTrimIncompleteUTF8Suffix(b []byte) []byte {
	return trimIncompleteUTF8Suffix(b)
}

// TestingStreamSourcePrefixBytesAppend exposes streaming prefix reads used by seekable naming paths.
func TestingStreamSourcePrefixBytesAppend(ctx context.Context, r io.Reader, byteLen int64) ([]byte, error) {
	return streamSourcePrefixBytesAppend(ctx, r, byteLen)
}

func TestingTextForFromContentStrings(
	re *regexp.Regexp,
	delimLineText string,
	outLines, fullSec []string,
	strip bool,
) string {
	return textForFromContentStrings(re, delimLineText, outLines, fullSec, strip)
}

func TestingThinStringsForSplitNaming(
	strip bool,
	delimText string,
	secLen int,
	firstInner, secondInner *string,
) (outThin, fullThin []string) {
	return thinStringsForSplitNaming(strip, delimText, secLen, firstInner, secondInner)
}

func TestingChooseSectionFilenameFromStrings(
	opts SplitOptions,
	seq int,
	delimLineText string,
	outLines []string,
	fullSec []string,
	ext string,
	existing map[string]bool,
) string {
	return chooseSectionFilenameFromStrings(opts, seq, delimLineText, outLines, fullSec, ext, existing)
}
