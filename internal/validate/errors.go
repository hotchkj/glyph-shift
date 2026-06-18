package validate

import "errors"

// ErrNilPathResolver is returned when a nil PathResolver is passed to a function that requires one.
var ErrNilPathResolver = errors.New("validate: nil PathResolver")
