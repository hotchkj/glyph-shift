package pipeline

import (
	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

func errNilSrcOutResolver(src SourceOpener, out OutputOpener, resolver validate.PathResolver) error {
	switch {
	case src == nil:
		return ErrNilSourceOpener
	case out == nil:
		return ErrNilOutputOpener
	case resolver == nil:
		return validate.ErrNilPathResolver
	default:
		return nil
	}
}

func errNilTransformSeams(stater FileStater, resolver validate.PathResolver, fileSession fileops.FileSession) error {
	switch {
	case stater == nil:
		return ErrNilFileStater
	case resolver == nil:
		return validate.ErrNilPathResolver
	case fileSession == nil:
		return fileops.ErrNilFileSession
	default:
		return nil
	}
}
