package fileops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// TransformFileWithContext reads a source file under cooperative lock, applies transforms,
// and atomically commits when apply is true and content changed. Preview calls (apply false)
// use [OpenLockedSourceRead] (shared lock); apply paths use exclusive modify locking.
func TransformFileWithContext(
	ctx context.Context,
	path string,
	opts TransformOptions,
	apply bool,
	fs FileSession,
) (TransformFileResult, error) {
	if fs == nil {
		return TransformFileResult{}, ErrNilFileSession
	}

	if err := ctx.Err(); err != nil {
		return TransformFileResult{}, err
	}

	return transformFileUsingSession(ctx, path, opts, apply, fs)
}

func readAtMostFromReader(r io.Reader, buf []byte) (n int, err error) {
	for n < len(buf) {
		nn, rerr := r.Read(buf[n:])
		n += nn

		if n == len(buf) {
			return n, nil
		}

		if errors.Is(rerr, io.EOF) {
			return n, io.EOF
		}

		if rerr != nil {
			return n, rerr
		}
	}

	return n, nil
}

// transformSourceBodyReader returns a reader for the remainder of the file after classifying the
// opening window for Git-style binary detection.
func transformSourceBodyReader(file io.ReadSeeker) (body io.Reader, isBinary bool, err error) {
	head := make([]byte, binaryCheckSize)
	headBytes, readErr := readAtMostFromReader(file, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, false, readErr
	}

	if scanForNull(head[:headBytes]) {
		return nil, true, nil
	}

	return io.MultiReader(bytes.NewReader(head[:headBytes]), file), false, nil
}

func transformNoOptsResult(path string, body io.Reader) (TransformFileResult, error) {
	hIn := newSHA256Reader(body)
	outTrack := newSHA256CountWriter()

	if _, copyErr := io.Copy(outTrack, hIn); copyErr != nil {
		return TransformFileResult{}, fmt.Errorf("transform file: copy: %w", copyErr)
	}

	would := hIn.n != outTrack.n || hIn.Digest() != outTrack.Digest()

	return TransformFileResult{
		Path:        path,
		Skipped:     true,
		SkipReason:  transformSkipReasonNoTransform,
		WouldChange: would,
	}, nil
}

// transformWouldChangeFromStats reports whether opts would alter bytes, using the same
// line-semantic counters as streaming and [TransformLines]. For current options (line endings,
// trim trailing space/tab, final newline) this matches input vs transformed byte equality.
func transformWouldChangeFromStats(opts TransformOptions, res *TransformFileResult) bool {
	if res == nil {
		return false
	}

	return lineEndingsWouldChange(opts, res) || trimTrailingWouldChange(opts, res) || finalNewlineWouldChange(opts, res)
}

func lineEndingsWouldChange(opts TransformOptions, res *TransformFileResult) bool {
	if opts.LineEndings != nil && res.EndingsChanged > 0 {
		return true
	}

	return false
}

func trimTrailingWouldChange(opts TransformOptions, res *TransformFileResult) bool {
	return opts.TrimTrailing && res.TrailingTrimmed > 0
}

func finalNewlineWouldChange(opts TransformOptions, res *TransformFileResult) bool {
	return opts.FinalNewline && res.FinalNewlineAdded
}

func transformActiveStreamResult(
	ctx context.Context,
	path string,
	body io.Reader,
	opts TransformOptions,
) (res TransformFileResult, would bool, srcDigest [sha256.Size]byte, err error) {
	hIn := newSHA256Reader(body)

	res, streamErr := runTransformStream(ctx, hIn, opts, nil, nil)
	if streamErr != nil {
		return TransformFileResult{}, false, [sha256.Size]byte{}, fmt.Errorf("transform file: stream: %w", streamErr)
	}

	would = transformWouldChangeFromStats(opts, &res)
	res.WouldChange = would
	res.Path = path

	return res, would, hIn.Digest(), nil
}

func commitTransformStreamReplay(mod *Modifier, ctx context.Context, opts TransformOptions) error {
	return mod.CommitFromWriter(func(writer io.Writer) ([sha256.Size]byte, error) {
		if _, seekCommitErr := mod.file.Seek(0, io.SeekStart); seekCommitErr != nil {
			return [sha256.Size]byte{}, seekCommitErr
		}

		bodyReplay, binReplay, replayErr := transformSourceBodyReader(mod.file)
		if replayErr != nil {
			return [sha256.Size]byte{}, replayErr
		}

		if binReplay {
			return [sha256.Size]byte{}, fmt.Errorf("%w", errTransformBinaryOnReplay)
		}

		hReplay := newSHA256Reader(bodyReplay)

		spill := ResolveWhitespaceSpillBacking(mod.fs)
		if _, commitStreamErr := runTransformStream(ctx, hReplay, opts, writer, spill); commitStreamErr != nil {
			return [sha256.Size]byte{}, commitStreamErr
		}

		return hReplay.Digest(), nil
	})
}

func transformFileUsingSession(
	ctx context.Context,
	path string,
	opts TransformOptions,
	apply bool,
	fs FileSession,
) (TransformFileResult, error) {
	if !apply {
		return previewTransformLockedSource(ctx, path, opts, fs)
	}

	mod, openErr := OpenForModifyLocked(path, fs)
	if openErr != nil {
		return TransformFileResult{}, fmt.Errorf("transform file: %w", openErr)
	}
	defer mod.Abort()

	res, shouldCommit, srcDigest, inspectErr := inspectTransformLockedSource(ctx, path, opts, mod)
	if inspectErr != nil {
		return TransformFileResult{}, inspectErr
	}

	if shouldCommit {
		mod.hash = srcDigest

		if commitErr := commitTransformStreamReplay(mod, ctx, opts); commitErr != nil {
			return TransformFileResult{}, fmt.Errorf("transform file: commit: %w", commitErr)
		}
	}

	return res, nil
}

type lockedSourceTransform struct {
	result    TransformFileResult
	srcDigest [sha256.Size]byte
	would     bool
}

func transformLockedSourceBody(
	ctx context.Context,
	path string,
	opts TransformOptions,
	source io.ReadSeeker,
	rewind bool,
) (lockedSourceTransform, error) {
	if rewind {
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return lockedSourceTransform{}, fmt.Errorf("transform file: seek: %w", err)
		}
	}

	body, isBinary, bodyErr := transformSourceBodyReader(source)
	if bodyErr != nil {
		return lockedSourceTransform{}, fmt.Errorf("transform file: source: %w", bodyErr)
	}
	if isBinary {
		return lockedSourceTransform{
			result: TransformFileResult{Path: path, Skipped: true, SkipReason: "binary"},
		}, nil
	}
	if !transformOptsActive(opts) {
		res, err := transformNoOptsResult(path, body)
		return lockedSourceTransform{result: res}, err
	}

	res, would, srcDigest, streamErr := transformActiveStreamResult(ctx, path, body, opts)
	if streamErr != nil {
		return lockedSourceTransform{}, streamErr
	}

	return lockedSourceTransform{result: res, srcDigest: srcDigest, would: would}, nil
}

func previewTransformLockedSource(
	ctx context.Context,
	path string,
	opts TransformOptions,
	fs FileSession,
) (TransformFileResult, error) {
	source, openErr := OpenLockedSourceRead(path, fs)
	if openErr != nil {
		return TransformFileResult{}, fmt.Errorf("transform file: %w", openErr)
	}
	defer func() {
		_ = source.Close()
	}()

	lt, bodyErr := transformLockedSourceBody(ctx, path, opts, source, false)
	if bodyErr != nil {
		return TransformFileResult{}, bodyErr
	}

	return lt.result, nil
}

func inspectTransformLockedSource(
	ctx context.Context,
	path string,
	opts TransformOptions,
	mod *Modifier,
) (result TransformFileResult, shouldCommit bool, sourceDigest [sha256.Size]byte, err error) {
	lt, bodyErr := transformLockedSourceBody(ctx, path, opts, mod.file, true)
	if bodyErr != nil {
		return TransformFileResult{}, false, [sha256.Size]byte{}, bodyErr
	}

	return lt.result, lt.would && !lt.result.Skipped, lt.srcDigest, nil
}
