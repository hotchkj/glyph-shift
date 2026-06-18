package fileops

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrAtomicDestinationExists = errors.New("atomic destination exists")

// ErrAtomicNilWriteContent is returned when writeContent is nil.
var ErrAtomicNilWriteContent = errors.New("atomic publish: nil write content callback")

type AtomicPublishMode int

const (
	AtomicPublishCreate AtomicPublishMode = iota
	AtomicPublishReplace
	AtomicPublishAppend
)

type AtomicPublishOptions struct {
	Path string
	Perm fs.FileMode
	Mode AtomicPublishMode
}

type atomicTempFile interface {
	io.Writer
	Sync() error
	Close() error
	Name() string
}

type atomicPublishDeps struct {
	createTemp func(dir, pattern string) (atomicTempFile, error)
	remove     func(name string) error
	rename     func(oldpath, newpath string) error
	chmod      func(name string, mode fs.FileMode) error
	openRead   func(path string) (io.ReadCloser, error)
}

// AtomicPublish writes the full destination payload to a temporary file in the same
// directory as opts.Path, fsyncs and closes that file, applies opts.Perm, then
// renames it into place. Temp files are never created outside the destination directory.
//
// On failure after the temp file was created, the temp path is removed; the previous
// destination (replace/append) remains unchanged until a successful rename.
func AtomicPublish(session FileSession, opts AtomicPublishOptions, writeContent func(io.Writer) error) error {
	if session == nil {
		return ErrNilFileSession
	}

	if dec, ok := session.(AtomicPublishStagingDecorator); ok {
		inner := writeContent
		writeContent = func(w io.Writer) error {
			return inner(dec.WrapAtomicPublishStagingWriter(opts.Path, w))
		}
	}

	deps := atomicPublishDeps{
		createTemp: func(dir, pattern string) (atomicTempFile, error) {
			return session.CreateTemp(dir, pattern)
		},
		remove: session.Remove,
		rename: session.Rename,
		chmod:  session.Chmod,
		openRead: func(path string) (io.ReadCloser, error) {
			h, openErr := session.OpenRead(path)
			if openErr != nil {
				return nil, openErr
			}

			return h, nil
		},
	}

	return atomicPublishWithDeps(deps, opts, writeContent)
}

func atomicPublishRejectExistingWhenCreate(deps atomicPublishDeps, opts AtomicPublishOptions) error {
	if opts.Mode != AtomicPublishCreate {
		return nil
	}

	exists, existsErr := atomicDestinationExists(deps, opts.Path)
	if existsErr != nil {
		return existsErr
	}

	if exists {
		return fmt.Errorf("%w: %s", ErrAtomicDestinationExists, opts.Path)
	}

	return nil
}

func atomicPublishAfterTempOpen(
	deps atomicPublishDeps,
	opts AtomicPublishOptions,
	tmp atomicTempFile,
	tmpPath string,
	writeContent func(io.Writer) error,
	tmpClosed *bool,
) error {
	if opts.Mode == AtomicPublishAppend {
		if copyErr := atomicCopyExistingIfPresent(deps, opts.Path, tmp); copyErr != nil {
			return copyErr
		}
	}

	closed, err := writeSyncCloseAtomicTemp(tmp, writeContent)
	*tmpClosed = closed
	if err != nil {
		return err
	}

	if chmodErr := deps.chmod(tmpPath, opts.Perm); chmodErr != nil {
		return fmt.Errorf("atomic publish: chmod temp: %w", chmodErr)
	}

	if renameErr := deps.rename(tmpPath, opts.Path); renameErr != nil {
		return fmt.Errorf("atomic publish: rename temp: %w", renameErr)
	}

	return nil
}

func writeSyncCloseAtomicTemp(tmp atomicTempFile, writeContent func(io.Writer) error) (bool, error) {
	if writeErr := writeContent(tmp); writeErr != nil {
		return false, fmt.Errorf("atomic publish: write temp: %w", writeErr)
	}

	if syncErr := tmp.Sync(); syncErr != nil {
		return false, fmt.Errorf("atomic publish: sync temp: %w", syncErr)
	}

	if closeErr := tmp.Close(); closeErr != nil {
		return true, fmt.Errorf("atomic publish: close temp: %w", closeErr)
	}

	return true, nil
}

func atomicPublishWithDeps(
	deps atomicPublishDeps,
	opts AtomicPublishOptions,
	writeContent func(io.Writer) error,
) (retErr error) {
	if writeContent == nil {
		return ErrAtomicNilWriteContent
	}

	if err := atomicPublishRejectExistingWhenCreate(deps, opts); err != nil {
		return err
	}

	tmp, err := deps.createTemp(filepath.Dir(opts.Path), ".glyph-shift-*")
	if err != nil {
		return fmt.Errorf("atomic publish: create temp: %w", err)
	}

	tmpPath := tmp.Name()
	tmpClosed := false

	defer func() {
		retErr = cleanupAtomicTempOnError(retErr, tmp, tmpClosed, deps, tmpPath)
	}()

	retErr = atomicPublishAfterTempOpen(deps, opts, tmp, tmpPath, writeContent, &tmpClosed)

	return retErr
}

func cleanupAtomicTempOnError(
	retErr error,
	tmp interface{ Close() error },
	tmpClosed bool,
	deps atomicPublishDeps,
	tmpPath string,
) error {
	if retErr == nil {
		return nil
	}

	var cleanupErrs []error

	if !tmpClosed {
		if cerr := tmp.Close(); cerr != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("atomic publish: cleanup close temp: %w", cerr))
		}
	}

	if rerr := deps.remove(tmpPath); rerr != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("atomic publish: cleanup remove temp: %w", rerr))
	}

	if len(cleanupErrs) > 0 {
		return errors.Join(append([]error{retErr}, cleanupErrs...)...)
	}

	return retErr
}

func atomicDestinationExists(deps atomicPublishDeps, path string) (bool, error) {
	existing, err := deps.openRead(path)
	if err == nil {
		if closeErr := existing.Close(); closeErr != nil {
			return true, fmt.Errorf("atomic publish: close existing destination: %w", closeErr)
		}

		return true, nil
	}

	if atomicIsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("atomic publish: check destination: %w", err)
}

func atomicCopyExistingIfPresent(deps atomicPublishDeps, path string, dst io.Writer) error {
	existing, err := deps.openRead(path)
	if err != nil {
		if atomicIsNotExist(err) {
			return nil
		}

		return fmt.Errorf("atomic publish: open append destination: %w", err)
	}

	_, copyErr := io.Copy(dst, existing)

	closeErr := existing.Close()

	var copyWrapped error
	if copyErr != nil {
		copyWrapped = fmt.Errorf("atomic publish: copy append destination: %w", copyErr)
	}

	if closeErr != nil {
		closeWrapped := fmt.Errorf("atomic publish: close append destination read: %w", closeErr)
		if copyWrapped != nil {
			return errors.Join(copyWrapped, closeWrapped)
		}

		return closeWrapped
	}

	return copyWrapped
}

func atomicIsNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) || os.IsNotExist(err)
}
