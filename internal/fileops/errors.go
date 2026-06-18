package fileops

import "errors"

// ErrNilFileSession is returned when a nil FileSession is passed to a function that requires one.
var ErrNilFileSession = errors.New("fileops: nil FileSession")

// ErrNilWhitespaceSpillBacking is returned when trim-trailing spill needs scratch storage but none was supplied.
var ErrNilWhitespaceSpillBacking = errors.New("fileops: nil whitespace spill backing")

// ErrUnsupportedWhitespaceSpillHandle is returned when a session temp handle cannot back stream spill I/O.
var ErrUnsupportedWhitespaceSpillHandle = errors.New("fileops: unsupported whitespace spill temp handle")

// ErrNoDelimiterMatch is returned when split delimiter matching finds no source lines.
var ErrNoDelimiterMatch = errors.New("no delimiter match")

// ErrNoBlocksFound is returned when block extraction finds no complete start/end delimiter pairs.
var ErrNoBlocksFound = errors.New("no blocks found")
