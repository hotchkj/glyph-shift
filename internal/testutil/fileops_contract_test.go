package testutil

import "github.com/hotchkj/glyph-shift/internal/fileops"

// Compile-time assertion: MemTestSession must satisfy fileops.FileSession.
var _ fileops.FileSession = (*MemTestSession)(nil)
